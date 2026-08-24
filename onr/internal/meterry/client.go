package meterry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
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
	BalanceEnabled   bool
	BalanceCurrency  string
	BalanceTimeout   time.Duration
}

type Client struct {
	cfg            Config
	ingestURL      string
	balanceURL     string
	state          *subjectStateStore
	balanceTimeout time.Duration
	outbox         *outbox
	http           *http.Client
	stop           chan struct{}
	done           chan struct{}
	once           sync.Once
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
	state, err := openSubjectState(cfg.OutboxDir)
	if err != nil {
		return nil, err
	}
	if cfg.BalanceCurrency == "" {
		cfg.BalanceCurrency = "USD"
	}
	if cfg.BalanceTimeout <= 0 {
		cfg.BalanceTimeout = time.Second
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	balanceURL := base + "/v1/projects/" + url.PathEscape(cfg.ProjectID) + "/wallets/realtime/amount"
	c := &Client{
		cfg:            cfg,
		ingestURL:      base + "/v1/projects/" + url.PathEscape(cfg.ProjectID) + "/extractor-rule-sets/" + url.PathEscape(cfg.ExtractorRuleSet) + "/events/ingest",
		balanceURL:     balanceURL,
		state:          state,
		balanceTimeout: cfg.BalanceTimeout,
		outbox:         box,
		http:           &http.Client{Timeout: cfg.RequestTimeout},
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
	go c.worker()
	return c, nil
}

func (c *Client) Enabled() bool { return c != nil && c.cfg.Enabled }

func (c *Client) BalanceEnabled() bool {
	return c != nil && c.cfg.Enabled && c.cfg.BalanceEnabled
}

func (c *Client) CheckBalance(ctx context.Context, subjectType, subjectID string) (bool, error) {
	if !c.BalanceEnabled() || strings.TrimSpace(subjectType) == "" || strings.TrimSpace(subjectID) == "" {
		return true, nil
	}
	if c.state != nil && c.state.isBlocked(subjectType, subjectID) {
		return false, nil
	}
	q := url.Values{}
	q.Set("subject_type", subjectType)
	q.Set("subject_id", subjectID)
	q.Set("currency", c.cfg.BalanceCurrency)
	checkCtx, cancel := context.WithTimeout(ctx, c.balanceTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, c.balanceURL+"?"+q.Encode(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return false, fmt.Errorf("meterry balance lookup returned %s: %s", resp.Status, strings.TrimSpace(string(limited)))
	}
	var payload struct {
		Balance string `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, err
	}
	balance := new(big.Rat)
	if _, ok := balance.SetString(strings.TrimSpace(payload.Balance)); !ok {
		return false, fmt.Errorf("meterry balance response has invalid balance %q", payload.Balance)
	}
	return balance.Sign() > 0, nil
}

func (c *Client) ApplyWebhook(eventID, eventType, subjectType, subjectID string) error {
	if c == nil || !c.Enabled() {
		return nil
	}
	return c.state.applyWebhook(strings.TrimSpace(eventID), strings.TrimSpace(eventType), subjectType, subjectID)
}

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

func (c *Client) send(event Event) (retErr error) {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.ingestURL, bytes.NewReader(body))
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
	defer func() {
		if closeErr := resp.Body.Close(); retErr == nil && closeErr != nil {
			retErr = closeErr
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("meterry ingest returned %s: %s", resp.Status, strings.TrimSpace(string(limited)))
	}
	return nil
}
