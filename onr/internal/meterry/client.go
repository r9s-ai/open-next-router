package meterry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	sdk "github.com/meterry-com/meterry-go"
	"github.com/meterry-com/meterry-go/pkg/types"
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
	sdk            *sdk.Client
	state          *subjectStateStore
	balanceTimeout time.Duration
	outbox         *outbox
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
	sdkClient, err := sdk.NewClient(sdk.Config{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		HTTPClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("init meterry sdk: %w", err)
	}
	c := &Client{
		cfg:            cfg,
		sdk:            sdkClient,
		state:          state,
		balanceTimeout: cfg.BalanceTimeout,
		outbox:         box,
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
	checkCtx, cancel := context.WithTimeout(ctx, c.balanceTimeout)
	defer cancel()
	snapshot, err := c.sdk.Manager.ReadVirtualWalletAmountForProject(checkCtx, c.cfg.ProjectID, types.ReadVirtualWalletRequest{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Currency:    c.cfg.BalanceCurrency,
	})
	if err != nil {
		return false, err
	}
	return snapshot.Balance.Sign() > 0, nil
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

func (c *Client) send(event Event) error {
	rawJSON, err := json.Marshal(event.RawJSON)
	if err != nil {
		return err
	}
	var billing *types.XBilling
	if len(event.Billing) > 0 {
		billingJSON, marshalErr := json.Marshal(event.Billing)
		if marshalErr != nil {
			return marshalErr
		}
		billing = &types.XBilling{}
		if err := json.Unmarshal(billingJSON, billing); err != nil {
			return err
		}
	}
	_, err = c.sdk.Ingest.IngestRawEvent(context.Background(), c.cfg.ProjectID, c.cfg.ExtractorRuleSet, types.IngestEventRequest{
		Source:          event.Source,
		ExternalEventID: event.ExternalEventID,
		IdempotencyKey:  event.IdempotencyKey,
		SubjectType:     event.SubjectType,
		SubjectID:       event.SubjectID,
		OccurredAt:      event.OccurredAt,
		RawJSON:         rawJSON,
		XBilling:        billing,
		Metadata:        stringMetadata(event.Metadata),
	})
	return err
}

func stringMetadata(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		if key == "" || value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			out[key] = text
			continue
		}
		encoded, err := json.Marshal(value)
		if err == nil {
			out[key] = string(encoded)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
