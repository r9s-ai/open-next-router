package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen         string `yaml:"listen"`
		ReadTimeoutMs  int    `yaml:"read_timeout_ms"`
		WriteTimeoutMs int    `yaml:"write_timeout_ms"`
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

	TrafficDump struct {
		Enabled     bool   `yaml:"enabled"`
		Dir         string `yaml:"dir"`
		FilePath    string `yaml:"file_path"`
		MaxBytes    int    `yaml:"max_bytes"`
		MaskSecrets bool   `yaml:"mask_secrets"`
	} `yaml:"traffic_dump"`

	Logging struct {
		Level     string `yaml:"level"`
		AccessLog bool   `yaml:"access_log"`
	} `yaml:"logging"`
}

func Load(path string) (*Config, error) {
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
	if strings.TrimSpace(cfg.Providers.Dir) == "" {
		cfg.Providers.Dir = "./providers"
	}
	if strings.TrimSpace(cfg.Keys.File) == "" {
		cfg.Keys.File = "./keys.yaml"
	}
	if strings.TrimSpace(cfg.Models.File) == "" {
		cfg.Models.File = "./models.yaml"
	}
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
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Auth.APIKey) == "" {
		return errors.New("auth.api_key is required (or set ONR_API_KEY)")
	}
	if cfg.TrafficDump.MaxBytes < 0 {
		return errors.New("traffic_dump.max_bytes must be non-negative")
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
