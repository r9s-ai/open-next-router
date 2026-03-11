package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const DefaultBaseURL = "http://localhost:3300"

type ClientConfig struct {
	APIKey  string
	BaseURL string
}

func FromEnv() (ClientConfig, error) {
	cfg := ClientConfig{
		APIKey:  strings.TrimSpace(os.Getenv("ONR_API_KEY")),
		BaseURL: strings.TrimSpace(os.Getenv("ONR_BASE_URL")),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return cfg, cfg.Validate()
}

func (c ClientConfig) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("ONR_API_KEY environment variable is not set")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("base URL is empty")
	}
	return nil
}
