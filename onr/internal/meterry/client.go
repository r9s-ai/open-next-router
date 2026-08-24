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
	"github.com/r9s-ai/open-next-router/pkg/controlplane"
)

type Config struct {
	Enabled                 bool
	BaseURL                 string
	ProjectID               string
	APIKey                  string
	ExtractorRuleSet        string
	OutboxDir               string
	RequestTimeout          time.Duration
	RetryInterval           time.Duration
	BalanceEnabled          bool
	BalanceCurrency         string
	BalanceTimeout          time.Duration
	BalanceCacheTTL         time.Duration
	BalanceNegativeCacheTTL time.Duration
	ControlPlane            *controlplane.Client
}

type balanceCacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

type balanceCall struct {
	done    chan struct{}
	allowed bool
	err     error
}

type Client struct {
	cfg                     Config
	sdk                     *sdk.Client
	state                   *subjectStateStore
	balanceTimeout          time.Duration
	balanceCacheTTL         time.Duration
	balanceNegativeCacheTTL time.Duration
	balanceMu               sync.Mutex
	balanceCache            map[string]balanceCacheEntry
	balanceCalls            map[string]*balanceCall
	outbox                  eventOutbox
	stop                    chan struct{}
	done                    chan struct{}
	once                    sync.Once
}

func New(cfg Config) (*Client, error) {
	if !cfg.Enabled {
		return &Client{cfg: cfg}, nil
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.ProjectID) == "" || strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.ExtractorRuleSet) == "" {
		return nil, errors.New("meterry enabled requires base_url, project_id, api_key, and extractor_rule_set_id")
	}
	var box eventOutbox
	var err error
	if cfg.ControlPlane == nil {
		box, err = openOutbox(cfg.OutboxDir)
		if err != nil {
			return nil, err
		}
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 3 * time.Second
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = time.Second
	}
	var state *subjectStateStore
	// Redis is the shared source of truth for subject and webhook state. Keep
	// the file-backed store only for the standalone/local-outbox mode.
	if cfg.ControlPlane == nil {
		state, err = openSubjectState(cfg.OutboxDir)
		if err != nil {
			return nil, err
		}
	}
	if cfg.BalanceCurrency == "" {
		cfg.BalanceCurrency = "USD"
	}
	if cfg.BalanceTimeout <= 0 {
		cfg.BalanceTimeout = time.Second
	}
	if cfg.BalanceCacheTTL <= 0 {
		cfg.BalanceCacheTTL = 3 * time.Second
	}
	if cfg.BalanceNegativeCacheTTL <= 0 {
		cfg.BalanceNegativeCacheTTL = time.Second
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
		cfg:                     cfg,
		sdk:                     sdkClient,
		state:                   state,
		balanceTimeout:          cfg.BalanceTimeout,
		balanceCacheTTL:         cfg.BalanceCacheTTL,
		balanceNegativeCacheTTL: cfg.BalanceNegativeCacheTTL,
		balanceCache:            make(map[string]balanceCacheEntry),
		balanceCalls:            make(map[string]*balanceCall),
		outbox:                  box,
		stop:                    make(chan struct{}),
		done:                    make(chan struct{}),
	}
	if cfg.ControlPlane != nil {
		if err := cfg.ControlPlane.EnsureBillingGroup(context.Background()); err != nil {
			return nil, fmt.Errorf("init Redis billing stream: %w", err)
		}
		box = &redisOutbox{controlPlane: cfg.ControlPlane}
		c.outbox = box
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
	if c.cfg.ControlPlane != nil {
		state, err := c.cfg.ControlPlane.GetSubjectState(ctx, subjectType, subjectID)
		if err != nil {
			return false, err
		}
		if state.Blocked {
			return false, nil
		}
		if cached, ok, err := c.cfg.ControlPlane.GetBalanceCache(ctx, subjectType, subjectID, c.cfg.BalanceCurrency); err == nil && ok && time.Now().Before(cached.ExpiresAt) {
			return cached.Allowed, nil
		}
	}
	key := subjectKey(subjectType, subjectID) + "/" + strings.TrimSpace(c.cfg.BalanceCurrency)
	now := time.Now()
	c.balanceMu.Lock()
	if entry, ok := c.balanceCache[key]; ok && now.Before(entry.expiresAt) {
		c.balanceMu.Unlock()
		return entry.allowed, nil
	}
	if call, ok := c.balanceCalls[key]; ok {
		c.balanceMu.Unlock()
		select {
		case <-call.done:
			return call.allowed, call.err
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	call := &balanceCall{done: make(chan struct{})}
	c.balanceCalls[key] = call
	c.balanceMu.Unlock()

	call.allowed, call.err = c.readBalanceWithSharedCache(ctx, subjectType, subjectID)
	c.balanceMu.Lock()
	if call.err == nil {
		ttl := c.balanceCacheTTL
		if !call.allowed {
			ttl = c.balanceNegativeCacheTTL
		}
		c.balanceCache[key] = balanceCacheEntry{allowed: call.allowed, expiresAt: time.Now().Add(ttl)}
	}
	delete(c.balanceCalls, key)
	close(call.done)
	c.balanceMu.Unlock()
	return call.allowed, call.err
}

func (c *Client) readBalanceWithSharedCache(ctx context.Context, subjectType, subjectID string) (bool, error) {
	if c.cfg.ControlPlane == nil {
		return c.fetchBalance(ctx, subjectType, subjectID)
	}
	if cached, ok, err := c.cfg.ControlPlane.GetBalanceCache(ctx, subjectType, subjectID, c.cfg.BalanceCurrency); err == nil && ok && time.Now().Before(cached.ExpiresAt) {
		return cached.Allowed, nil
	}
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	acquired, err := c.cfg.ControlPlane.AcquireBalanceRefreshLock(ctx, subjectType, subjectID, c.cfg.BalanceCurrency, token, c.balanceTimeout)
	if err != nil {
		return c.fetchBalance(ctx, subjectType, subjectID)
	}
	if acquired {
		defer func() {
			_ = c.cfg.ControlPlane.ReleaseBalanceRefreshLock(context.Background(), subjectType, subjectID, c.cfg.BalanceCurrency, token)
		}()
		return c.fetchAndCacheBalance(ctx, subjectType, subjectID)
	}
	if allowed, ok := c.waitForSharedBalance(ctx, subjectType, subjectID); ok {
		return allowed, nil
	}
	return c.fetchAndCacheBalance(ctx, subjectType, subjectID)
}

func (c *Client) waitForSharedBalance(ctx context.Context, subjectType, subjectID string) (bool, bool) {
	deadline := time.NewTimer(c.balanceTimeout)
	poll := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer poll.Stop()
	for {
		select {
		case <-deadline.C:
			return false, false
		case <-poll.C:
			cached, ok, err := c.cfg.ControlPlane.GetBalanceCache(ctx, subjectType, subjectID, c.cfg.BalanceCurrency)
			if err == nil && ok && time.Now().Before(cached.ExpiresAt) {
				return cached.Allowed, true
			}
		}
	}
}

func (c *Client) fetchAndCacheBalance(ctx context.Context, subjectType, subjectID string) (bool, error) {
	allowed, err := c.fetchBalance(ctx, subjectType, subjectID)
	if err != nil {
		return false, err
	}
	ttl := c.balanceCacheTTL
	if !allowed {
		ttl = c.balanceNegativeCacheTTL
	}
	_ = c.cfg.ControlPlane.SetBalanceCache(ctx, subjectType, subjectID, c.cfg.BalanceCurrency, controlplane.BalanceCacheValue{
		Allowed:   allowed,
		ExpiresAt: time.Now().Add(ttl),
	}, ttl)
	return allowed, nil
}

func (c *Client) fetchBalance(ctx context.Context, subjectType, subjectID string) (bool, error) {
	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.balanceTimeout)
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
	subjectType = strings.TrimSpace(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	err := c.state.applyWebhook(strings.TrimSpace(eventID), strings.TrimSpace(eventType), subjectType, subjectID)
	if err == nil {
		c.invalidateBalance(subjectType, subjectID)
		if c.cfg.ControlPlane != nil {
			state := controlplane.SubjectState{}
			stateChanged := false
			switch strings.ToLower(strings.TrimSpace(eventType)) {
			case "wallet.insufficient_balance", "usage_limit.exhausted":
				state.Blocked = true
				state.BlockedReason = strings.ToLower(strings.TrimSpace(eventType))
				stateChanged = true
			case "wallet.balance_changed":
				state.Blocked = false
				stateChanged = true
			}
			if stateChanged {
				if err := c.cfg.ControlPlane.SetSubjectState(context.Background(), subjectType, subjectID, state); err != nil {
					return err
				}
			}
			if err := c.cfg.ControlPlane.InvalidateBalanceCache(context.Background(), subjectType, subjectID, c.cfg.BalanceCurrency); err != nil {
				return err
			}
			if _, err := c.cfg.ControlPlane.MarkWebhook(context.Background(), strings.TrimSpace(eventID), 7*24*time.Hour); err != nil {
				return err
			}
		}
	}
	return err
}

func (c *Client) invalidateBalance(subjectType, subjectID string) {
	key := subjectKey(subjectType, subjectID) + "/" + strings.TrimSpace(c.cfg.BalanceCurrency)
	if key == "/"+strings.TrimSpace(c.cfg.BalanceCurrency) {
		return
	}
	c.balanceMu.Lock()
	delete(c.balanceCache, key)
	c.balanceMu.Unlock()
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
		event, token, err := c.outbox.first()
		if err == nil {
			if err := c.send(event); err == nil {
				_ = c.outbox.ack(token)
				continue
			} else {
				_ = c.outbox.fail(token, event)
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
