package meterry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Enabled          bool
	BaseURL          string
	ProjectID        string
	APIKey           string
	ExtractorRuleSet string
	OutboxDir        string
	RequestTimeout   time.Duration
	RetryInterval    time.Duration
}

type Client struct {
	cfg    Config
	outbox *outbox
	http   *http.Client
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once
}

func New(cfg Config) (*Client, error) {
	if !cfg.Enabled {
		return &Client{cfg: cfg}, nil
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.ProjectID) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.ExtractorRuleSet) == "" {
		return nil, errors.New("meterry enabled requires base_url, project_id, api_key, and extractor_rule_set_id")
	}
	box, err := openOutbox(cfg.OutboxDir)
	if err != nil {
		return nil, err
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 3 * time.Second
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = time.Second
	}
	c := &Client{
		cfg:    cfg,
		outbox: box,
		http:   &http.Client{Timeout: cfg.RequestTimeout},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go c.worker()
	return c, nil
}

func (c *Client) Enabled() bool { return c != nil && c.cfg.Enabled }

func (c *Client) Enqueue(event Event) error {
	if !c.Enabled() {
		return nil
	}
	return c.outbox.append(event)
}

func (c *Client) Close() error {
	if !c.Enabled() {
		return nil
	}
	c.once.Do(func() { close(c.stop) })
	<-c.done
	return nil
}

func (c *Client) worker() {
	defer close(c.done)
	for {
		event, err := c.outbox.first()
		if err == nil {
			if err := c.send(event); err == nil {
				_ = c.outbox.ack(event.IdempotencyKey)
				continue
			}
		}
		select {
		case <-c.stop:
			return
		case <-time.After(c.cfg.RetryInterval):
		}
	}
}

func (c *Client) send(event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/v1/projects/" + c.cfg.ProjectID + "/extractor-rule-sets/" + c.cfg.ExtractorRuleSet + "/events/ingest"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", event.IdempotencyKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("meterry ingest returned %s: %s", resp.Status, strings.TrimSpace(string(limited)))
	}
	return nil
}
