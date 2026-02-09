package usageestimate

import (
	"errors"
	"strings"
)

type Config struct {
	Enabled bool `yaml:"enabled"`

	// EstimateWhenMissingOrZero triggers estimation when upstream usage is missing,
	// or all token fields are zero.
	EstimateWhenMissingOrZero bool `yaml:"estimate_when_missing_or_zero"`

	// Strategy controls how tokens are estimated. Currently only "heuristic" is supported.
	Strategy string `yaml:"strategy"`

	// MaxRequestBytes limits how many request bytes are considered for estimation.
	MaxRequestBytes int `yaml:"max_request_bytes"`

	// MaxResponseBytes limits how many non-stream response bytes are considered for estimation.
	MaxResponseBytes int `yaml:"max_response_bytes"`

	// MaxStreamCollectBytes limits how many trailing stream bytes are kept for best-effort estimation.
	MaxStreamCollectBytes int `yaml:"max_stream_collect_bytes"`

	// APIs defines which API types participate in estimation (e.g. "chat.completions").
	// If empty, defaults will be applied.
	APIs []string `yaml:"apis"`
}

func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	// default enabled
	if !cfg.Enabled {
		cfg.Enabled = true
	}
	// default: estimate on missing/zero
	if !cfg.EstimateWhenMissingOrZero {
		cfg.EstimateWhenMissingOrZero = true
	}
	if strings.TrimSpace(cfg.Strategy) == "" {
		cfg.Strategy = "heuristic"
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = 1 * 1024 * 1024
	}
	if cfg.MaxResponseBytes <= 0 {
		cfg.MaxResponseBytes = 1 * 1024 * 1024
	}
	if cfg.MaxStreamCollectBytes <= 0 {
		cfg.MaxStreamCollectBytes = 256 * 1024
	}
	if len(cfg.APIs) == 0 {
		cfg.APIs = []string{
			"chat.completions",
			"responses",
			"claude.messages",
			"embeddings",
			"gemini.generateContent",
			"gemini.streamGenerateContent",
		}
	}
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.Strategy) != "" && strings.TrimSpace(cfg.Strategy) != "heuristic" {
		return errors.New("usage_estimation.strategy must be \"heuristic\"")
	}
	if cfg.MaxRequestBytes < 0 {
		return errors.New("usage_estimation.max_request_bytes must be non-negative")
	}
	if cfg.MaxResponseBytes < 0 {
		return errors.New("usage_estimation.max_response_bytes must be non-negative")
	}
	if cfg.MaxStreamCollectBytes < 0 {
		return errors.New("usage_estimation.max_stream_collect_bytes must be non-negative")
	}
	return nil
}

func (c *Config) IsAPIEnabled(api string) bool {
	if c == nil {
		return false
	}
	if !c.Enabled {
		return false
	}
	a := strings.TrimSpace(strings.ToLower(api))
	for _, x := range c.APIs {
		if strings.TrimSpace(strings.ToLower(x)) == a {
			return true
		}
	}
	return false
}
