package config

import "testing"

func TestFromEnvRequiresAPIKey(t *testing.T) {
	t.Setenv("ONR_API_KEY", "")
	t.Setenv("ONR_BASE_URL", "")

	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFromEnvUsesDefaultBaseURL(t *testing.T) {
	t.Setenv("ONR_API_KEY", "test-key")
	t.Setenv("ONR_BASE_URL", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv error: %v", err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
}

func TestFromEnvUsesCustomBaseURL(t *testing.T) {
	t.Setenv("ONR_API_KEY", "test-key")
	t.Setenv("ONR_BASE_URL", "https://api.example.com")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv error: %v", err)
	}
	if cfg.BaseURL != "https://api.example.com" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
}
