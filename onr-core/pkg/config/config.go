package config

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen         string `yaml:"listen"`
		ReadTimeoutMs  int    `yaml:"read_timeout_ms"`
		WriteTimeoutMs int    `yaml:"write_timeout_ms"`
		PidFile        string `yaml:"pid_file"`
	} `yaml:"server"`

	Auth struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"auth"`

	Providers struct {
		Dir string `yaml:"dir"`
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
		Enabled     bool   `yaml:"enabled"`
		Dir         string `yaml:"dir"`
		FilePath    string `yaml:"file_path"`
		MaxBytes    int    `yaml:"max_bytes"`
		MaskSecrets bool   `yaml:"mask_secrets"`
	} `yaml:"traffic_dump"`

	Logging struct {
		Level         string `yaml:"level"`
		AccessLog     bool   `yaml:"access_log"`
		AccessLogPath string `yaml:"access_log_path"`
	} `yaml:"logging"`
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
		cfg.Server.Listen = ":3000"
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
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	// default true for local debugging
	if !cfg.Logging.AccessLog {
		cfg.Logging.AccessLog = true
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("ONR_LISTEN")); v != "" {
		cfg.Server.Listen = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_API_KEY")); v != "" {
		cfg.Auth.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_PROVIDERS_DIR")); v != "" {
		cfg.Providers.Dir = v
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

	applyProviderProxyEnvOverrides(cfg)

	cfg.UsageEstimation.Enabled = envBool("ONR_USAGE_ESTIMATION_ENABLED", cfg.UsageEstimation.Enabled)

	cfg.TrafficDump.Enabled = envBool("ONR_TRAFFIC_DUMP_ENABLED", cfg.TrafficDump.Enabled)
	if v := strings.TrimSpace(os.Getenv("ONR_TRAFFIC_DUMP_DIR")); v != "" {
		cfg.TrafficDump.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_TRAFFIC_DUMP_FILE_PATH")); v != "" {
		cfg.TrafficDump.FilePath = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_TRAFFIC_DUMP_MAX_BYTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.TrafficDump.MaxBytes = n
		}
	}
	cfg.TrafficDump.MaskSecrets = envBool("ONR_TRAFFIC_DUMP_MASK_SECRETS", cfg.TrafficDump.MaskSecrets)
	if v := strings.TrimSpace(os.Getenv("ONR_READ_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Server.ReadTimeoutMs = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONR_WRITE_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Server.WriteTimeoutMs = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONR_PID_FILE")); v != "" {
		cfg.Server.PidFile = v
	}
	if v := strings.TrimSpace(os.Getenv("ONR_ACCESS_LOG_PATH")); v != "" {
		cfg.Logging.AccessLogPath = v
	}
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Auth.APIKey) == "" {
		return errors.New("auth.api_key is required (or set ONR_API_KEY)")
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
	if cfg.TrafficDump.MaxBytes < 0 {
		return errors.New("traffic_dump.max_bytes must be non-negative")
	}
	if cfg.OAuth.TokenPersist.Enabled && strings.TrimSpace(cfg.OAuth.TokenPersist.Dir) == "" {
		return errors.New("oauth.token_persist.dir is required when oauth.token_persist.enabled=true")
	}
	return nil
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
