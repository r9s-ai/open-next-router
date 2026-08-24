package tui

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

type adminService struct {
	cfg *config.Config
	cp  *controlplane.Client
}

type overviewSnapshot struct {
	RedisEnabled      bool
	RedisReachable    bool
	RedisError        string
	KeyPrefix         string
	AccessKeyMode     string
	MeterryEnabled    bool
	MeterryReachable  bool
	MeterryConfigured bool
	MeterryError      string
	ProjectID         string
	ExtractorRuleSet  string
	Pending           int64
	DeadLetter        int64
	BillingError      string
	ConsumerGroup     string
	ConsumerName      string
	MaxAttempts       int
	FailureMode       string
	BalanceCacheTTL   time.Duration
	NegativeCacheTTL  time.Duration
	RefreshedAt       time.Time
}

type migrationReport struct {
	Total        int
	WouldMigrate int
	Migrated     int
	Conflicts    []string
	Skipped      []string
}

func newAdminService(cfgPath string) (*adminService, error) {
	cfg, err := config.Load(strings.TrimSpace(cfgPath))
	if err != nil {
		return nil, err
	}
	s := &adminService{cfg: cfg}
	if !cfg.Redis.Enabled {
		return s, nil
	}
	s.cp, err = controlplane.New(controlplane.Config{
		Addr:                 cfg.Redis.Addr,
		Username:             cfg.Redis.Username,
		Password:             cfg.Redis.Password,
		TLS:                  cfg.Redis.TLS,
		KeyPrefix:            cfg.Redis.KeyPrefix,
		OperationTimeout:     time.Duration(cfg.Redis.OperationTimeoutMs) * time.Millisecond,
		AccessKeyHashSecret:  cfg.Redis.AccessKeyHashSecret,
		BillingStream:        cfg.Redis.BillingStream,
		BillingConsumerGroup: cfg.Redis.BillingConsumerGroup,
		BillingConsumerName:  cfg.Redis.BillingConsumerName,
		BillingMaxAttempts:   cfg.Redis.BillingMaxAttempts,
	})
	if err != nil {
		return nil, fmt.Errorf("init Redis control plane: %w", err)
	}
	return s, nil
}

func (s *adminService) Close() error {
	if s == nil || s.cp == nil {
		return nil
	}
	return s.cp.Close()
}

func (s *adminService) ListAccessKeys(ctx context.Context) ([]controlplane.AccessKeyRecord, error) {
	if s.cp == nil {
		return nil, fmt.Errorf("redis access-key management is disabled")
	}
	records, err := s.cp.ListAccessKeyRecords(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	return records, nil
}

func (s *adminService) GetAccessKey(ctx context.Context, name string) (*controlplane.AccessKeyRecord, error) {
	if s.cp == nil {
		return nil, fmt.Errorf("redis access-key management is disabled")
	}
	return s.cp.GetAccessKeyRecord(ctx, name)
}

func (s *adminService) CreateAccessKey(ctx context.Context, name, subjectType, subjectID string, expiresAt *time.Time, metadata map[string]string) (string, error) {
	if s.cp == nil {
		return "", fmt.Errorf("redis access-key management is disabled")
	}
	secret, err := controlplane.NewAccessKeySecret()
	if err != nil {
		return "", err
	}
	record := controlplane.AccessKeyRecord{
		Name: name, SecretHash: s.cp.HashAccessKey(secret), Status: "active",
		SubjectType: subjectType, SubjectID: subjectID, ExpiresAt: expiresAt, Metadata: metadata,
	}
	if err := s.cp.CreateAccessKey(ctx, record); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *adminService) RevokeAccessKey(ctx context.Context, name string) error {
	if s.cp == nil {
		return fmt.Errorf("redis access-key management is disabled")
	}
	return s.cp.RevokeAccessKey(ctx, name)
}

func (s *adminService) RotateAccessKey(ctx context.Context, name string) (string, error) {
	if s.cp == nil {
		return "", fmt.Errorf("redis access-key management is disabled")
	}
	return s.cp.RotateAccessKey(ctx, name)
}

func (s *adminService) SubjectState(ctx context.Context, subjectType, subjectID string) (controlplane.SubjectState, error) {
	if s.cp == nil {
		return controlplane.SubjectState{}, nil
	}
	return s.cp.GetSubjectState(ctx, subjectType, subjectID)
}

func (s *adminService) Snapshot(ctx context.Context) overviewSnapshot {
	if s == nil || s.cfg == nil {
		return overviewSnapshot{RefreshedAt: time.Now()}
	}
	result := overviewSnapshot{
		RedisEnabled:      s.cfg.Redis.Enabled,
		KeyPrefix:         s.cfg.Redis.KeyPrefix,
		AccessKeyMode:     s.cfg.Redis.AccessKeyMode,
		MeterryEnabled:    s.cfg.Meterry.Enabled,
		MeterryConfigured: strings.TrimSpace(s.cfg.Meterry.BaseURL) != "" && strings.TrimSpace(s.cfg.Meterry.ProjectID) != "" && strings.TrimSpace(s.cfg.Meterry.APIKey) != "",
		ProjectID:         redactIdentifier(s.cfg.Meterry.ProjectID),
		ExtractorRuleSet:  redactIdentifier(s.cfg.Meterry.ExtractorRuleSet),
		ConsumerGroup:     s.cfg.Redis.BillingConsumerGroup,
		ConsumerName:      s.cfg.Redis.BillingConsumerName,
		MaxAttempts:       s.cfg.Redis.BillingMaxAttempts,
		FailureMode:       s.cfg.Meterry.BalanceEnforcement.FailureMode,
		BalanceCacheTTL:   s.cfg.Meterry.BalanceCacheTTL(),
		NegativeCacheTTL:  s.cfg.Meterry.BalanceNegativeCacheTTL(),
		RefreshedAt:       time.Now(),
	}
	if s.cp != nil {
		result.ConsumerName = s.cp.BillingConsumerName()
		if err := s.cp.Ping(ctx); err != nil {
			result.RedisError = err.Error()
		} else {
			result.RedisReachable = true
		}
		if s.cfg.Meterry.Enabled {
			var err error
			result.Pending, result.DeadLetter, err = s.cp.BillingStats(ctx)
			if err != nil {
				result.BillingError = err.Error()
			}
		}
	}
	if s.cfg.Meterry.Enabled && result.MeterryConfigured {
		result.MeterryReachable, result.MeterryError = meterryReachable(ctx, s.cfg.Meterry.BaseURL)
	}
	return result
}

func meterryReachable(ctx context.Context, baseURL string) (bool, string) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(baseURL, "/"), nil)
	if err != nil {
		return false, err.Error()
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false, err.Error()
	}
	_ = response.Body.Close()
	return true, ""
}

func redactIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 10 {
		return value
	}
	return value[:6] + "..." + value[len(value)-4:]
}

func (s *adminService) Migrate(ctx context.Context, keysPath string, dryRun bool) (migrationReport, error) {
	if s.cp == nil {
		return migrationReport{}, fmt.Errorf("redis access-key management is disabled")
	}
	keys, err := keystore.Load(strings.TrimSpace(keysPath))
	if err != nil {
		return migrationReport{}, err
	}
	report := migrationReport{Total: len(keys.AccessKeys())}
	for _, key := range keys.AccessKeys() {
		record := controlplane.AccessKeyRecord{Name: key.Name, SecretHash: s.cp.HashAccessKey(key.Value), Status: "active", SubjectType: "api_key", SubjectID: key.Name, Metadata: map[string]string{"comment": key.Comment}}
		existing, err := s.cp.GetAccessKeyRecord(ctx, record.Name)
		if err != nil {
			return report, err
		}
		if existing != nil && existing.SecretHash != record.SecretHash {
			report.Conflicts = append(report.Conflicts, record.Name)
			continue
		}
		if existing != nil {
			report.Skipped = append(report.Skipped, record.Name)
			continue
		}
		if dryRun {
			report.WouldMigrate++
			continue
		}
		if err := s.cp.CreateAccessKey(ctx, record); err != nil {
			return report, err
		}
		report.Migrated++
	}
	return report, nil
}
