package controlplane

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr                 string
	Username             string
	Password             string
	TLS                  bool
	KeyPrefix            string
	OperationTimeout     time.Duration
	AccessKeyHashSecret  string
	BillingStream        string
	BillingConsumerGroup string
	BillingConsumerName  string
	BillingMaxAttempts   int
}

type Client struct {
	rdb                *redis.Client
	prefix             string
	timeout            time.Duration
	hashSecret         []byte
	billingStream      string
	billingGroup       string
	billingConsumer    string
	billingMaxAttempts int
}

type AccessKeyRecord struct {
	Name        string            `json:"name"`
	SecretHash  string            `json:"secret_hash"`
	Status      string            `json:"status"`
	SubjectType string            `json:"subject_type"`
	SubjectID   string            `json:"subject_id"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Version     int64             `json:"version"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type SubjectState struct {
	Blocked       bool      `json:"blocked"`
	BlockedReason string    `json:"blocked_reason,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	Version       int64     `json:"version"`
}

type BalanceCacheValue struct {
	Allowed   bool      `json:"allowed"`
	Balance   string    `json:"balance"`
	ExpiresAt time.Time `json:"expires_at"`
}

type StreamMessage struct {
	ID     string
	Values map[string]string
}

func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, errors.New("redis address is required")
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = "onr"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 500 * time.Millisecond
	}
	if cfg.BillingMaxAttempts <= 0 {
		cfg.BillingMaxAttempts = 10
	}
	if strings.TrimSpace(cfg.BillingConsumerName) == "" {
		host, _ := os.Hostname()
		cfg.BillingConsumerName = fmt.Sprintf("onr-%s-%d", strings.TrimSpace(host), os.Getpid())
	}
	options, err := redis.ParseURL(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("parse redis address: %w", err)
	}
	if options.Addr == "" {
		options.Addr = cfg.Addr
	}
	options.Username = cfg.Username
	options.Password = cfg.Password
	if cfg.TLS {
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Client{
		rdb:                redis.NewClient(options),
		prefix:             strings.Trim(strings.TrimSpace(cfg.KeyPrefix), ":"),
		timeout:            cfg.OperationTimeout,
		hashSecret:         []byte(cfg.AccessKeyHashSecret),
		billingStream:      strings.TrimSpace(cfg.BillingStream),
		billingGroup:       strings.TrimSpace(cfg.BillingConsumerGroup),
		billingConsumer:    strings.TrimSpace(cfg.BillingConsumerName),
		billingMaxAttempts: cfg.BillingMaxAttempts,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.withTimeout(ctx, func(ctx context.Context) error { return c.rdb.Ping(ctx).Err() })
}

func (c *Client) withTimeout(ctx context.Context, fn func(context.Context) error) error {
	if c == nil || c.rdb == nil {
		return errors.New("redis client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return fn(callCtx)
}

func (c *Client) key(parts ...string) string {
	values := []string{c.prefix}
	for _, part := range parts {
		values = append(values, strings.Trim(strings.TrimSpace(part), ":"))
	}
	return strings.Join(values, ":")
}

func (c *Client) accessKeyRecordKey(name string) string { return c.key("access_key", name) }
func (c *Client) accessKeyIndexKey() string             { return c.key("access_key", "index") }
func (c *Client) subjectKey(subjectType, subjectID string) string {
	return c.key("subject", subjectType, subjectID)
}
func (c *Client) balanceKey(subjectType, subjectID, currency string) string {
	return c.key("balance", subjectType, subjectID, currency)
}

func (c *Client) HashAccessKey(secret string) string {
	mac := hmac.New(sha256.New, c.hashSecret)
	_, _ = mac.Write([]byte(secret))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) LookupAccessKey(ctx context.Context, secret string) (*AccessKeyRecord, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, nil
	}
	hash := c.HashAccessKey(strings.TrimSpace(secret))
	var name string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		name, err = c.rdb.HGet(ctx, c.accessKeyIndexKey(), hash).Result()
		return err
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c.GetAccessKey(ctx, name)
}

func (c *Client) GetAccessKey(ctx context.Context, name string) (*AccessKeyRecord, error) {
	record, err := c.GetAccessKeyRecord(ctx, name)
	if err != nil || record == nil {
		return nil, err
	}
	if record.Status != "active" || (record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt)) {
		return nil, nil
	}
	return record, nil
}

// GetAccessKeyRecord returns the complete record for administrative views,
// including revoked and expired keys.
func (c *Client) GetAccessKeyRecord(ctx context.Context, name string) (*AccessKeyRecord, error) {
	var raw string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		raw, err = c.rdb.Get(ctx, c.accessKeyRecordKey(name)).Result()
		return err
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record AccessKeyRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return nil, fmt.Errorf("decode Redis access key: %w", err)
	}
	return &record, nil
}

func (c *Client) PutAccessKey(ctx context.Context, record AccessKeyRecord) error {
	if strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.SecretHash) == "" {
		return errors.New("access key name and secret hash are required")
	}
	if record.Status == "" {
		record.Status = "active"
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return c.withTimeout(ctx, func(ctx context.Context) error {
		pipe := c.rdb.TxPipeline()
		pipe.Set(ctx, c.accessKeyRecordKey(record.Name), raw, 0)
		pipe.HSet(ctx, c.accessKeyIndexKey(), record.SecretHash, record.Name)
		_, err := pipe.Exec(ctx)
		return err
	})
}

func (c *Client) CreateAccessKey(ctx context.Context, record AccessKeyRecord) error {
	if strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.SecretHash) == "" {
		return errors.New("access key name and secret hash are required")
	}
	if record.Status == "" {
		record.Status = "active"
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	const createScript = `
if redis.call('exists', KEYS[1]) == 1 then return 0 end
if redis.call('hexists', KEYS[2], ARGV[2]) == 1 then return -1 end
redis.call('set', KEYS[1], ARGV[1])
redis.call('hset', KEYS[2], ARGV[2], ARGV[3])
return 1`
	var result int64
	err = c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		result, err = c.rdb.Eval(ctx, createScript,
			[]string{c.accessKeyRecordKey(record.Name), c.accessKeyIndexKey()},
			raw, record.SecretHash, record.Name,
		).Int64()
		return err
	})
	if err != nil {
		return err
	}
	switch result {
	case 0:
		return fmt.Errorf("access key %q already exists", record.Name)
	case -1:
		return fmt.Errorf("access key secret is already associated with another key")
	case 1:
		return nil
	default:
		return fmt.Errorf("unexpected access key creation result %d", result)
	}
}

func (c *Client) RevokeAccessKey(ctx context.Context, name string) error {
	record, err := c.GetAccessKeyRecord(ctx, name)
	if err != nil || record == nil {
		return err
	}
	record.Status = "revoked"
	record.Version++
	return c.PutAccessKey(ctx, *record)
}

// ListAccessKeyRecords returns all records for administrative views.
func (c *Client) ListAccessKeyRecords(ctx context.Context) ([]AccessKeyRecord, error) {
	var names map[string]string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		names, err = c.rdb.HGetAll(ctx, c.accessKeyIndexKey()).Result()
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]AccessKeyRecord, 0, len(names))
	for _, name := range names {
		record, err := c.GetAccessKeyRecord(ctx, name)
		if err != nil {
			return nil, err
		}
		if record != nil {
			out = append(out, *record)
		}
	}
	return out, nil
}

func (c *Client) ListAccessKeys(ctx context.Context) ([]AccessKeyRecord, error) {
	var names map[string]string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		names, err = c.rdb.HGetAll(ctx, c.accessKeyIndexKey()).Result()
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]AccessKeyRecord, 0, len(names))
	for _, name := range names {
		record, err := c.GetAccessKey(ctx, name)
		if err != nil {
			return nil, err
		}
		if record != nil {
			out = append(out, *record)
		}
	}
	return out, nil
}

func (c *Client) RotateAccessKey(ctx context.Context, name string) (string, error) {
	record, err := c.GetAccessKeyRecord(ctx, name)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("access key %q not found", name)
	}
	secret, err := NewAccessKeySecret()
	if err != nil {
		return "", err
	}
	oldHash := record.SecretHash
	record.SecretHash = c.HashAccessKey(secret)
	record.Version++
	raw, err := json.Marshal(*record)
	if err != nil {
		return "", err
	}
	err = c.withTimeout(ctx, func(ctx context.Context) error {
		pipe := c.rdb.TxPipeline()
		pipe.HDel(ctx, c.accessKeyIndexKey(), oldHash)
		pipe.HSet(ctx, c.accessKeyIndexKey(), record.SecretHash, record.Name)
		pipe.Set(ctx, c.accessKeyRecordKey(record.Name), raw, 0)
		_, err := pipe.Exec(ctx)
		return err
	})
	return secret, err
}

func (c *Client) GetSubjectState(ctx context.Context, subjectType, subjectID string) (SubjectState, error) {
	var raw string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		raw, err = c.rdb.Get(ctx, c.subjectKey(subjectType, subjectID)).Result()
		return err
	})
	if errors.Is(err, redis.Nil) {
		return SubjectState{}, nil
	}
	if err != nil {
		return SubjectState{}, err
	}
	var state SubjectState
	return state, json.Unmarshal([]byte(raw), &state)
}

func (c *Client) SetSubjectState(ctx context.Context, subjectType, subjectID string, state SubjectState) error {
	state.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.withTimeout(ctx, func(ctx context.Context) error {
		return c.rdb.Set(ctx, c.subjectKey(subjectType, subjectID), raw, 0).Err()
	})
}

func (c *Client) MarkWebhook(ctx context.Context, eventID string, retention time.Duration) (bool, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return false, errors.New("webhook event id is required")
	}
	var marked bool
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		marked, err = c.rdb.SetNX(ctx, c.key("webhook", eventID), "1", retention).Result()
		return err
	})
	return marked, err
}

func (c *Client) GetBalanceCache(ctx context.Context, subjectType, subjectID, currency string) (BalanceCacheValue, bool, error) {
	var raw string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		raw, err = c.rdb.Get(ctx, c.balanceKey(subjectType, subjectID, currency)).Result()
		return err
	})
	if errors.Is(err, redis.Nil) {
		return BalanceCacheValue{}, false, nil
	}
	if err != nil {
		return BalanceCacheValue{}, false, err
	}
	var value BalanceCacheValue
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return BalanceCacheValue{}, false, err
	}
	return value, true, nil
}

func (c *Client) SetBalanceCache(ctx context.Context, subjectType, subjectID, currency string, value BalanceCacheValue, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.withTimeout(ctx, func(ctx context.Context) error {
		return c.rdb.Set(ctx, c.balanceKey(subjectType, subjectID, currency), raw, ttl).Err()
	})
}

func (c *Client) InvalidateBalanceCache(ctx context.Context, subjectType, subjectID, currency string) error {
	return c.withTimeout(ctx, func(ctx context.Context) error {
		return c.rdb.Del(ctx, c.balanceKey(subjectType, subjectID, currency)).Err()
	})
}

func (c *Client) EnqueueBillingEvent(ctx context.Context, payload []byte) (string, error) {
	var id string
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		id, err = c.rdb.XAdd(ctx, &redis.XAddArgs{Stream: c.key(c.billingStream), Values: map[string]any{"payload": string(payload)}}).Result()
		return err
	})
	return id, err
}

func (c *Client) EnsureBillingGroup(ctx context.Context) error {
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		return c.rdb.XGroupCreateMkStream(ctx, c.key(c.billingStream), c.billingGroup, "0").Err()
	})
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *Client) ReadBilling(ctx context.Context, count int, block time.Duration) ([]StreamMessage, error) {
	return c.readBilling(ctx, count, block, ">")
}

func (c *Client) ReadPendingBilling(ctx context.Context, count int) ([]StreamMessage, error) {
	return c.readBilling(ctx, count, 0, "0")
}

func (c *Client) readBilling(ctx context.Context, count int, block time.Duration, id string) ([]StreamMessage, error) {
	var messages []redis.XStream
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		messages, err = c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.billingGroup, Consumer: c.billingConsumer, Streams: []string{c.key(c.billingStream), id}, Count: int64(count), Block: block}).Result()
		return err
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []StreamMessage
	for _, stream := range messages {
		for _, message := range stream.Messages {
			values := map[string]string{}
			for key, value := range message.Values {
				values[key] = fmt.Sprint(value)
			}
			out = append(out, StreamMessage{ID: message.ID, Values: values})
		}
	}
	return out, nil
}

func (c *Client) AckBilling(ctx context.Context, ids ...string) error {
	return c.withTimeout(ctx, func(ctx context.Context) error {
		return c.rdb.XAck(ctx, c.key(c.billingStream), c.billingGroup, ids...).Err()
	})
}

func (c *Client) BillingMaxAttempts() int {
	if c == nil || c.billingMaxAttempts <= 0 {
		return 10
	}
	return c.billingMaxAttempts
}

// BillingConsumerName returns the effective consumer name used by this client.
func (c *Client) BillingConsumerName() string {
	if c == nil {
		return ""
	}
	return c.billingConsumer
}

func (c *Client) BillingStats(ctx context.Context) (pending, deadLetter int64, err error) {
	err = c.withTimeout(ctx, func(ctx context.Context) error {
		summary, err := c.rdb.XPending(ctx, c.key(c.billingStream), c.billingGroup).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		if summary != nil {
			pending = summary.Count
		}
		deadLetter, err = c.rdb.XLen(ctx, c.key(c.billingStream, "dead-letter")).Result()
		return err
	})
	return pending, deadLetter, err
}

func (c *Client) AutoClaimBilling(ctx context.Context, minIdle time.Duration, count int) ([]StreamMessage, error) {
	var messages []redis.XMessage
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		messages, _, err = c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: c.key(c.billingStream), Group: c.billingGroup, Consumer: c.billingConsumer, MinIdle: minIdle, Start: "0-0", Count: int64(count)}).Result()
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]StreamMessage, 0, len(messages))
	for _, message := range messages {
		values := map[string]string{}
		for key, value := range message.Values {
			values[key] = fmt.Sprint(value)
		}
		out = append(out, StreamMessage{ID: message.ID, Values: values})
	}
	return out, nil
}

func (c *Client) IncrementBillingAttempt(ctx context.Context, streamID string) (int64, error) {
	var attempts int64
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		attempts, err = c.rdb.HIncrBy(ctx, c.key(c.billingStream, "attempts"), streamID, 1).Result()
		return err
	})
	return attempts, err
}

func (c *Client) DeadLetterBilling(ctx context.Context, streamID string, payload []byte) error {
	return c.withTimeout(ctx, func(ctx context.Context) error {
		pipe := c.rdb.TxPipeline()
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: c.key(c.billingStream, "dead-letter"), Values: map[string]any{"source_id": streamID, "payload": string(payload)}})
		pipe.XAck(ctx, c.key(c.billingStream), c.billingGroup, streamID)
		pipe.XDel(ctx, c.key(c.billingStream), streamID)
		pipe.HDel(ctx, c.key(c.billingStream, "attempts"), streamID)
		_, err := pipe.Exec(ctx)
		return err
	})
}

func (c *Client) AcquireBalanceRefreshLock(ctx context.Context, subjectType, subjectID, currency, token string, ttl time.Duration) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, errors.New("balance refresh lock token is required")
	}
	var acquired bool
	err := c.withTimeout(ctx, func(ctx context.Context) error {
		var err error
		acquired, err = c.rdb.SetNX(ctx, c.key("balance", "lock", subjectType, subjectID, currency), token, ttl).Result()
		return err
	})
	return acquired, err
}

func (c *Client) ReleaseBalanceRefreshLock(ctx context.Context, subjectType, subjectID, currency, token string) error {
	const releaseScript = `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`
	return c.withTimeout(ctx, func(ctx context.Context) error {
		return c.rdb.Eval(ctx, releaseScript, []string{c.key("balance", "lock", subjectType, subjectID, currency)}, token).Err()
	})
}

func NewAccessKeySecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "ak_" + base64.RawURLEncoding.EncodeToString(buf), nil
}
