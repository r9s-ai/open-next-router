package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
	"gopkg.in/yaml.v3"
)

const (
	defaultAccessLogRotateMaxSizeMB  = 100
	defaultAccessLogRotateMaxBackups = 14
	defaultAccessLogRotateMaxAgeDays = 14
)

var allowedTrafficDumpSections = []string{
	"meta",
	"origin_request",
	"upstream_request",
	"upstream_response",
	"proxy_response",
	"stream",
}

type AccessLogRotateConfig struct {
	Enabled    bool `yaml:"enabled"`
	MaxSizeMB  int  `yaml:"max_size_mb"`
	MaxBackups int  `yaml:"max_backups"`
	MaxAgeDays int  `yaml:"max_age_days"`
	Compress   bool `yaml:"compress"`

	maxSizeMBSet  bool `yaml:"-"`
	maxBackupsSet bool `yaml:"-"`
	maxAgeDaysSet bool `yaml:"-"`
}

func (c *AccessLogRotateConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawRotate struct {
		Enabled    bool `yaml:"enabled"`
		MaxSizeMB  int  `yaml:"max_size_mb"`
		MaxBackups int  `yaml:"max_backups"`
		MaxAgeDays int  `yaml:"max_age_days"`
		Compress   bool `yaml:"compress"`
	}
	var raw rawRotate
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Enabled = raw.Enabled
	c.MaxSizeMB = raw.MaxSizeMB
	c.MaxBackups = raw.MaxBackups
	c.MaxAgeDays = raw.MaxAgeDays
	c.Compress = raw.Compress
	c.maxSizeMBSet = false
	c.maxBackupsSet = false
	c.maxAgeDaysSet = false

	if value.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := strings.TrimSpace(value.Content[i].Value)
		switch key {
		case "max_size_mb":
			c.maxSizeMBSet = true
		case "max_backups":
			c.maxBackupsSet = true
		case "max_age_days":
			c.maxAgeDaysSet = true
		}
	}
	return nil
}

type AppNameInferConfig struct {
	Enabled bool   `yaml:"enabled"`
	Unknown string `yaml:"unknown"`
}

type LoggingConfig struct {
	Level                 string                `yaml:"level"`
	AccessLog             bool                  `yaml:"access_log"`
	AccessLogPath         string                `yaml:"access_log_path"`
	AccessLogFormat       string                `yaml:"access_log_format"`
	AccessLogFormatPreset string                `yaml:"access_log_format_preset"`
	AccessLogRotate       AccessLogRotateConfig `yaml:"access_log_rotate"`
	AppNameInfer          AppNameInferConfig    `yaml:"appname_infer"`
}

type Config struct {
	Server struct {
		Listen         string `yaml:"listen"`
		ReadTimeoutMs  int    `yaml:"read_timeout_ms"`
		WriteTimeoutMs int    `yaml:"write_timeout_ms"`
		PidFile        string `yaml:"pid_file"`
	} `yaml:"server"`

	Auth struct {
		APIKey string `yaml:"api_key"`
		// TokenKey controls onr:v1 token-key auth behavior.
		TokenKey struct {
			// AllowBYOKWithoutK allows BYOK token keys that only contain uk/uk64 without k/k64.
			// Default false for safety.
			AllowBYOKWithoutK bool `yaml:"allow_byok_without_k"`
		} `yaml:"token_key"`
	} `yaml:"auth"`

	Providers struct {
		Dir string `yaml:"dir"`
		// AutoReload watches providers.dir and reloads provider DSL files at runtime.
		AutoReload struct {
			Enabled    bool `yaml:"enabled"`
			DebounceMs int  `yaml:"debounce_ms"`
		} `yaml:"auto_reload"`
	} `yaml:"providers"`

	Keys struct {
		File string `yaml:"file"`
	} `yaml:"keys"`

	Models struct {
		// File is an optional models list file. If not set or missing, /v1/models returns an empty list.
		File string `yaml:"file"`
	} `yaml:"models"`

	OAuth struct {
		// TokenPersist controls whether OAuth access tokens should be persisted to disk.
		TokenPersist struct {
			Enabled bool   `yaml:"enabled"`
			Dir     string `yaml:"dir"`
		} `yaml:"token_persist"`
	} `yaml:"oauth"`

	Pricing struct {
		Enabled bool `yaml:"enabled"`
		// File points to base model pricing data.
		File string `yaml:"file"`
		// OverridesFile points to local pricing overrides (channel/provider multipliers and model overrides).
		OverridesFile string `yaml:"overrides_file"`
	} `yaml:"pricing"`

	// UpstreamProxies configures outbound HTTP proxy by provider name.
	// Values are proxy URLs (e.g. "http://127.0.0.1:7890").
	UpstreamProxies struct {
		ByProvider map[string]string `yaml:"by_provider"`
	} `yaml:"upstream_proxies"`

	UsageEstimation usageestimate.Config `yaml:"usage_estimation"`

	TrafficDump struct {
		Enabled     bool     `yaml:"enabled"`
		Dir         string   `yaml:"dir"`
		FilePath    string   `yaml:"file_path"`
		MaxBytes    int      `yaml:"max_bytes"`
		MaskSecrets bool     `yaml:"mask_secrets"`
		Sections    []string `yaml:"sections"`
	} `yaml:"traffic_dump"`

	Logging LoggingConfig `yaml:"logging"`
}

func Load(path string) (*Config, error) {
	// #nosec G304 -- path is provided by trusted config/flag.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.Server.Listen) == "" {
		cfg.Server.Listen = ":3300"
	}
	if cfg.Server.ReadTimeoutMs <= 0 {
		cfg.Server.ReadTimeoutMs = 60000
	}
	if cfg.Server.WriteTimeoutMs <= 0 {
		cfg.Server.WriteTimeoutMs = 60000
	}
	if strings.TrimSpace(cfg.Server.PidFile) == "" {
		cfg.Server.PidFile = "/var/run/onr.pid"
	}
	if strings.TrimSpace(cfg.Providers.Dir) == "" {
		cfg.Providers.Dir = "./config/providers"
	}
	if cfg.Providers.AutoReload.DebounceMs <= 0 {
		cfg.Providers.AutoReload.DebounceMs = 300
	}
	if strings.TrimSpace(cfg.Keys.File) == "" {
		cfg.Keys.File = "./keys.yaml"
	}
	if strings.TrimSpace(cfg.Models.File) == "" {
		cfg.Models.File = "./models.yaml"
	}
	if strings.TrimSpace(cfg.OAuth.TokenPersist.Dir) == "" {
		cfg.OAuth.TokenPersist.Dir = "./run/oauth"
	}
	if strings.TrimSpace(cfg.Pricing.File) == "" {
		cfg.Pricing.File = "./price.yaml"
	}
	if strings.TrimSpace(cfg.Pricing.OverridesFile) == "" {
		cfg.Pricing.OverridesFile = "./price_overrides.yaml"
	}

	if cfg.UpstreamProxies.ByProvider == nil {
		cfg.UpstreamProxies.ByProvider = map[string]string{}
	}
	cfg.UpstreamProxies.ByProvider = normalizeProviderStringMap(cfg.UpstreamProxies.ByProvider)

	usageestimate.ApplyDefaults(&cfg.UsageEstimation)

	if strings.TrimSpace(cfg.TrafficDump.Dir) == "" {
		cfg.TrafficDump.Dir = "./dumps"
	}
	if strings.TrimSpace(cfg.TrafficDump.FilePath) == "" {
		cfg.TrafficDump.FilePath = "{{.request_id}}.log"
	}
	if cfg.TrafficDump.MaxBytes == 0 {
		cfg.TrafficDump.MaxBytes = 1 * 1024 * 1024
	}
	// default true
	if !cfg.TrafficDump.MaskSecrets {
		cfg.TrafficDump.MaskSecrets = true
	}
	if strings.TrimSpace(cfg.Logging.Level) == "" {
		cfg.Logging.Level = "info"
	}
	// default true for local debugging
	if !cfg.Logging.AccessLog {
		cfg.Logging.AccessLog = true
	}
	if !cfg.Logging.AccessLogRotate.maxSizeMBSet {
		cfg.Logging.AccessLogRotate.MaxSizeMB = defaultAccessLogRotateMaxSizeMB
	}
	if !cfg.Logging.AccessLogRotate.maxBackupsSet {
		cfg.Logging.AccessLogRotate.MaxBackups = defaultAccessLogRotateMaxBackups
	}
	if !cfg.Logging.AccessLogRotate.maxAgeDaysSet {
		cfg.Logging.AccessLogRotate.MaxAgeDays = defaultAccessLogRotateMaxAgeDays
	}
}

func applyEnvOverrides(cfg *Config) {
	applyEnvServerAuthOverrides(cfg)
	applyEnvProviderAndDataOverrides(cfg)
	applyProviderProxyEnvOverrides(cfg)
	applyEnvTrafficDumpOverrides(cfg)
	applyEnvLoggingOverrides(cfg)
}

func applyEnvServerAuthOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("ONR_LISTEN")); v != "" {
		cfg.Server.Listen = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_API_KEY")); v != "" {
		cfg.Auth.APIKey = v
	}
	cfg.Auth.TokenKey.AllowBYOKWithoutK = envBool("ONR_TOKEN_KEY_ALLOW_BYOK_WITHOUT_K", cfg.Auth.TokenKey.AllowBYOKWithoutK)
	if n, ok := envInt("ONR_READ_TIMEOUT_MS"); ok && n > 0 {
		cfg.Server.ReadTimeoutMs = n
	}
	if n, ok := envInt("ONR_WRITE_TIMEOUT_MS"); ok && n > 0 {
		cfg.Server.WriteTimeoutMs = n
	}
	if v := strings.TrimSpace(os.Getenv("ONR_PID_FILE")); v != "" {
		cfg.Server.PidFile = v
	}
}

func applyEnvProviderAndDataOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("ONR_PROVIDERS_DIR")); v != "" {
		cfg.Providers.Dir = v
	}
	cfg.Providers.AutoReload.Enabled = envBool("ONR_PROVIDERS_AUTO_RELOAD_ENABLED", cfg.Providers.AutoReload.Enabled)
	if n, ok := envInt("ONR_PROVIDERS_AUTO_RELOAD_DEBOUNCE_MS"); ok {
		cfg.Providers.AutoReload.DebounceMs = n
	}
	if v := strings.TrimSpace(os.Getenv("ONR_KEYS_FILE")); v != "" {
		cfg.Keys.File = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_MODELS_FILE")); v != "" {
		cfg.Models.File = v
	}
	cfg.OAuth.TokenPersist.Enabled = envBool("ONR_OAUTH_TOKEN_PERSIST_ENABLED", cfg.OAuth.TokenPersist.Enabled)
	if v := strings.TrimSpace(os.Getenv("ONR_OAUTH_TOKEN_PERSIST_DIR")); v != "" {
		cfg.OAuth.TokenPersist.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_PRICE_FILE")); v != "" {
		cfg.Pricing.File = v
	}
	cfg.Pricing.Enabled = envBool("ONR_PRICING_ENABLED", cfg.Pricing.Enabled)
	if v := strings.TrimSpace(os.Getenv("ONR_PRICE_OVERRIDES_FILE")); v != "" {
		cfg.Pricing.OverridesFile = v
	}

	cfg.UsageEstimation.Enabled = envBool("ONR_USAGE_ESTIMATION_ENABLED", cfg.UsageEstimation.Enabled)
}

func applyEnvTrafficDumpOverrides(cfg *Config) {
	cfg.TrafficDump.Enabled = envBool("ONR_TRAFFIC_DUMP_ENABLED", cfg.TrafficDump.Enabled)
	if v := strings.TrimSpace(os.Getenv("ONR_TRAFFIC_DUMP_DIR")); v != "" {
		cfg.TrafficDump.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_TRAFFIC_DUMP_FILE_PATH")); v != "" {
		cfg.TrafficDump.FilePath = v
	}
	if n, ok := envInt("ONR_TRAFFIC_DUMP_MAX_BYTES"); ok {
		cfg.TrafficDump.MaxBytes = n
	}
	cfg.TrafficDump.MaskSecrets = envBool("ONR_TRAFFIC_DUMP_MASK_SECRETS", cfg.TrafficDump.MaskSecrets)
	if v := strings.TrimSpace(os.Getenv("ONR_TRAFFIC_DUMP_SECTIONS")); v != "" {
		cfg.TrafficDump.Sections = splitCommaTrim(v)
	}
}

func applyEnvLoggingOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("ONR_LOG_LEVEL")); v != "" {
		cfg.Logging.Level = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_ACCESS_LOG_PATH")); v != "" {
		cfg.Logging.AccessLogPath = v
	}
	if v := os.Getenv("ONR_ACCESS_LOG_FORMAT"); strings.TrimSpace(v) != "" {
		cfg.Logging.AccessLogFormat = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_ACCESS_LOG_FORMAT_PRESET")); v != "" {
		cfg.Logging.AccessLogFormatPreset = v
	}
	cfg.Logging.AccessLogRotate.Enabled = envBool("ONR_ACCESS_LOG_ROTATE_ENABLED", cfg.Logging.AccessLogRotate.Enabled)
	if n, ok := envInt("ONR_ACCESS_LOG_ROTATE_MAX_SIZE_MB"); ok {
		cfg.Logging.AccessLogRotate.MaxSizeMB = n
		cfg.Logging.AccessLogRotate.maxSizeMBSet = true
	}
	if n, ok := envInt("ONR_ACCESS_LOG_ROTATE_MAX_BACKUPS"); ok {
		cfg.Logging.AccessLogRotate.MaxBackups = n
		cfg.Logging.AccessLogRotate.maxBackupsSet = true
	}
	if n, ok := envInt("ONR_ACCESS_LOG_ROTATE_MAX_AGE_DAYS"); ok {
		cfg.Logging.AccessLogRotate.MaxAgeDays = n
		cfg.Logging.AccessLogRotate.maxAgeDaysSet = true
	}
	cfg.Logging.AccessLogRotate.Compress = envBool("ONR_ACCESS_LOG_ROTATE_COMPRESS", cfg.Logging.AccessLogRotate.Compress)
}

func envInt(name string) (int, bool) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func validate(cfg *Config) error {
	if normalized, err := normalizeLogLevel(cfg.Logging.Level); err != nil {
		return err
	} else {
		cfg.Logging.Level = normalized
	}

	for prov, raw := range cfg.UpstreamProxies.ByProvider {
		p := strings.ToLower(strings.TrimSpace(prov))
		v := strings.TrimSpace(raw)
		if p == "" || v == "" {
			continue
		}
		// Keep this validation lightweight; deeper checks are done where the proxy URL is parsed.
		if !strings.Contains(v, "://") {
			return errors.New("upstream_proxies.by_provider must be a URL (e.g. http://127.0.0.1:7890)")
		}
	}
	if err := usageestimate.Validate(&cfg.UsageEstimation); err != nil {
		return err
	}
	if cfg.Providers.AutoReload.Enabled && cfg.Providers.AutoReload.DebounceMs <= 0 {
		return errors.New("providers.auto_reload.debounce_ms must be > 0 when providers.auto_reload.enabled=true")
	}
	if cfg.TrafficDump.MaxBytes < 0 {
		return errors.New("traffic_dump.max_bytes must be non-negative")
	}
	normalizedSections, err := normalizeTrafficDumpSections(cfg.TrafficDump.Sections)
	if err != nil {
		return err
	}
	cfg.TrafficDump.Sections = normalizedSections
	if cfg.OAuth.TokenPersist.Enabled && strings.TrimSpace(cfg.OAuth.TokenPersist.Dir) == "" {
		return errors.New("oauth.token_persist.dir is required when oauth.token_persist.enabled=true")
	}
	if cfg.Logging.AccessLogRotate.Enabled {
		if !cfg.Logging.AccessLog {
			return errors.New("logging.access_log must be true when logging.access_log_rotate.enabled=true")
		}
		if strings.TrimSpace(cfg.Logging.AccessLogPath) == "" {
			return errors.New("logging.access_log_path is required when logging.access_log_rotate.enabled=true")
		}
	}
	if cfg.Logging.AccessLogRotate.MaxSizeMB <= 0 {
		return errors.New("logging.access_log_rotate.max_size_mb must be > 0")
	}
	if cfg.Logging.AccessLogRotate.MaxBackups <= 0 {
		return errors.New("logging.access_log_rotate.max_backups must be > 0")
	}
	if cfg.Logging.AccessLogRotate.MaxAgeDays < 0 {
		return errors.New("logging.access_log_rotate.max_age_days must be >= 0")
	}
	return nil
}

func normalizeLogLevel(level string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return "info", nil
	case "debug":
		return "debug", nil
	case "warn":
		return "warn", nil
	case "error":
		return "error", nil
	default:
		return "", errors.New("logging.level must be one of: debug, info, warn, error")
	}
}

func envBool(name string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func normalizeTrafficDumpSections(sections []string) ([]string, error) {
	if len(sections) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(sections))
	seen := make(map[string]struct{}, len(sections))
	for _, raw := range sections {
		sec := strings.ToLower(strings.TrimSpace(raw))
		if sec == "" {
			continue
		}
		if !slices.Contains(allowedTrafficDumpSections, sec) {
			return nil, fmt.Errorf("traffic_dump.sections contains unsupported section %q (allowed: %s)", raw, strings.Join(allowedTrafficDumpSections, ","))
		}
		if _, ok := seen[sec]; ok {
			continue
		}
		seen[sec] = struct{}{}
		normalized = append(normalized, sec)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

func splitCommaTrim(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

var envProviderProxyPattern = regexp.MustCompile(`^ONR_UPSTREAM_PROXY_([A-Z0-9_]+)$`)

func applyProviderProxyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.UpstreamProxies.ByProvider == nil {
		cfg.UpstreamProxies.ByProvider = map[string]string{}
	}
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		m := envProviderProxyPattern.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(m[1]))
		if provider == "" {
			continue
		}
		// Allow unsetting by providing empty string.
		if v == "" {
			delete(cfg.UpstreamProxies.ByProvider, provider)
			continue
		}
		cfg.UpstreamProxies.ByProvider[provider] = v
	}
}

func normalizeProviderStringMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if key == "" {
			continue
		}
		if val == "" {
			delete(out, key)
			continue
		}
		out[key] = val
	}
	return out
}
