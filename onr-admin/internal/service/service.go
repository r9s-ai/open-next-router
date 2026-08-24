package service

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/controlplane"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

type CreateAccessKeyInput struct {
	Name, SubjectType, SubjectID string
	ExpiresAt                    *time.Time
	Metadata                     map[string]string
}

type MigrationReport struct {
	Total, WouldMigrate, Migrated int
	Conflicts, Skipped            []string
}

type Overview struct {
	RedisEnabled, RedisReachable                        bool
	RedisError, KeyPrefix, AccessKeyMode                string
	MeterryEnabled, MeterryConfigured, MeterryReachable bool
	MeterryError, ProjectID, ExtractorRuleSet           string
	Pending, DeadLetter                                 int64
	BillingError, ConsumerGroup, ConsumerName           string
	MaxAttempts                                         int
	FailureMode                                         string
	BalanceCacheTTL, NegativeCacheTTL                   time.Duration
	RefreshedAt                                         time.Time
}

type Service struct {
	cfg *config.Config
	cp  *controlplane.Client
}

func New(cfgPath string) (*Service, error) {
	cfg, err := config.Load(strings.TrimSpace(cfgPath))
	if err != nil {
		return nil, err
	}
	s := &Service{cfg: cfg}
	if !cfg.Redis.Enabled {
		return s, nil
	}
	s.cp, err = controlplane.New(controlplane.Config{Addr: cfg.Redis.Addr, Username: cfg.Redis.Username, Password: cfg.Redis.Password, TLS: cfg.Redis.TLS, KeyPrefix: cfg.Redis.KeyPrefix, OperationTimeout: time.Duration(cfg.Redis.OperationTimeoutMs) * time.Millisecond, AccessKeyHashSecret: cfg.Redis.AccessKeyHashSecret, BillingStream: cfg.Redis.BillingStream, BillingConsumerGroup: cfg.Redis.BillingConsumerGroup, BillingConsumerName: cfg.Redis.BillingConsumerName, BillingMaxAttempts: cfg.Redis.BillingMaxAttempts})
	if err != nil {
		return nil, fmt.Errorf("init Redis control plane: %w", err)
	}
	return s, nil
}
func (s *Service) Close() error {
	if s == nil || s.cp == nil {
		return nil
	}
	return s.cp.Close()
}
func (s *Service) Config() *config.Config {
	if s == nil {
		return nil
	}
	return s.cfg
}
func (s *Service) Client() *controlplane.Client {
	if s == nil {
		return nil
	}
	return s.cp
}
func (s *Service) ListAccessKeys(ctx context.Context) ([]controlplane.AccessKeyRecord, error) {
	if s.cp == nil {
		return nil, fmt.Errorf("redis access-key management is disabled")
	}
	v, e := s.cp.ListAccessKeyRecords(ctx)
	sort.Slice(v, func(i, j int) bool { return v[i].Name < v[j].Name })
	return v, e
}
func (s *Service) GetAccessKey(ctx context.Context, name string) (*controlplane.AccessKeyRecord, error) {
	if s.cp == nil {
		return nil, fmt.Errorf("redis access-key management is disabled")
	}
	return s.cp.GetAccessKeyRecord(ctx, name)
}
func (s *Service) CreateAccessKey(ctx context.Context, in CreateAccessKeyInput) (string, error) {
	if s.cp == nil {
		return "", fmt.Errorf("redis access-key management is disabled")
	}
	secret, e := controlplane.NewAccessKeySecret()
	if e != nil {
		return "", e
	}
	rec := controlplane.AccessKeyRecord{Name: in.Name, SecretHash: s.cp.HashAccessKey(secret), Status: "active", SubjectType: in.SubjectType, SubjectID: in.SubjectID, ExpiresAt: in.ExpiresAt, Metadata: in.Metadata}
	if e = s.cp.CreateAccessKey(ctx, rec); e != nil {
		return "", e
	}
	return secret, nil
}
func (s *Service) RevokeAccessKey(ctx context.Context, name string) error {
	if s.cp == nil {
		return fmt.Errorf("redis access-key management is disabled")
	}
	return s.cp.RevokeAccessKey(ctx, name)
}
func (s *Service) RotateAccessKey(ctx context.Context, name string) (string, error) {
	if s.cp == nil {
		return "", fmt.Errorf("redis access-key management is disabled")
	}
	return s.cp.RotateAccessKey(ctx, name)
}
func (s *Service) SubjectState(ctx context.Context, t, id string) (controlplane.SubjectState, error) {
	if s.cp == nil {
		return controlplane.SubjectState{}, fmt.Errorf("redis is disabled")
	}
	return s.cp.GetSubjectState(ctx, t, id)
}
func (s *Service) RedisPing(ctx context.Context) error {
	if s.cp == nil {
		return fmt.Errorf("redis is disabled")
	}
	return s.cp.Ping(ctx)
}
func (s *Service) BillingStats(ctx context.Context) (int64, int64, error) {
	if s.cp == nil {
		return 0, 0, fmt.Errorf("redis is disabled")
	}
	return s.cp.BillingStats(ctx)
}
func (s *Service) Overview(ctx context.Context) Overview {
	o := Overview{RefreshedAt: time.Now()}
	if s == nil || s.cfg == nil {
		return o
	}
	c := s.cfg
	o.RedisEnabled = c.Redis.Enabled
	o.KeyPrefix = c.Redis.KeyPrefix
	o.AccessKeyMode = c.Redis.AccessKeyMode
	o.MeterryEnabled = c.Meterry.Enabled
	o.MeterryConfigured = strings.TrimSpace(c.Meterry.BaseURL) != "" && strings.TrimSpace(c.Meterry.ProjectID) != "" && strings.TrimSpace(c.Meterry.APIKey) != ""
	o.ProjectID = redact(c.Meterry.ProjectID)
	o.ExtractorRuleSet = redact(c.Meterry.ExtractorRuleSet)
	o.ConsumerGroup = c.Redis.BillingConsumerGroup
	o.MaxAttempts = c.Redis.BillingMaxAttempts
	o.ConsumerName = c.Redis.BillingConsumerName
	o.FailureMode = c.Meterry.BalanceEnforcement.FailureMode
	o.BalanceCacheTTL = c.Meterry.BalanceCacheTTL()
	o.NegativeCacheTTL = c.Meterry.BalanceNegativeCacheTTL()
	if s.cp != nil {
		o.ConsumerName = s.cp.BillingConsumerName()
		if e := s.cp.Ping(ctx); e != nil {
			o.RedisError = e.Error()
		} else {
			o.RedisReachable = true
		}
		o.Pending, o.DeadLetter, o.BillingError = safeBilling(ctx, s.cp, c.Meterry.Enabled)
	}
	if o.MeterryEnabled && o.MeterryConfigured {
		o.MeterryReachable, o.MeterryError = meterryReachable(ctx, c.Meterry.BaseURL)
	}
	return o
}
func safeBilling(ctx context.Context, cp *controlplane.Client, enabled bool) (int64, int64, string) {
	if !enabled {
		return 0, 0, ""
	}
	p, d, e := cp.BillingStats(ctx)
	if e != nil {
		return 0, 0, e.Error()
	}
	return p, d, ""
}
func meterryReachable(ctx context.Context, base string) (bool, string) {
	req, e := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(base, "/"), nil)
	if e != nil {
		return false, e.Error()
	}
	resp, e := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if e != nil {
		return false, e.Error()
	}
	_ = resp.Body.Close()
	return true, ""
}
func redact(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 10 {
		return v
	}
	return v[:6] + "..." + v[len(v)-4:]
}
func (s *Service) MigrateAccessKeys(ctx context.Context, path string, dry bool) (MigrationReport, error) {
	var r MigrationReport
	if s.cp == nil {
		return r, fmt.Errorf("redis access-key management is disabled")
	}
	ks, e := keystore.Load(strings.TrimSpace(path))
	if e != nil {
		return r, e
	}
	r.Total = len(ks.AccessKeys())
	for _, k := range ks.AccessKeys() {
		rec := controlplane.AccessKeyRecord{Name: k.Name, SecretHash: s.cp.HashAccessKey(k.Value), Status: "active", SubjectType: "api_key", SubjectID: k.Name, Metadata: map[string]string{"comment": k.Comment}}
		old, e := s.cp.GetAccessKeyRecord(ctx, rec.Name)
		if e != nil {
			return r, e
		}
		if old != nil && old.SecretHash != rec.SecretHash {
			r.Conflicts = append(r.Conflicts, rec.Name)
			continue
		}
		if old != nil {
			r.Skipped = append(r.Skipped, rec.Name)
			continue
		}
		if dry {
			r.WouldMigrate++
			continue
		}
		if e = s.cp.CreateAccessKey(ctx, rec); e != nil {
			return r, e
		}
		r.Migrated++
	}
	return r, nil
}
