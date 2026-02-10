package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "onr.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoad_Defaults(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  api_key: "k"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Server.Listen != ":3300" {
		t.Fatalf("default listen=%q", cfg.Server.Listen)
	}
	if cfg.Keys.File == "" || cfg.Models.File == "" || cfg.Providers.Dir == "" {
		t.Fatalf("expected default paths")
	}
	if !cfg.TrafficDump.MaskSecrets {
		t.Fatalf("mask_secrets default should be true")
	}
	if !cfg.Logging.AccessLog {
		t.Fatalf("access_log default should be true")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  api_key: "k"
upstream_proxies:
  by_provider:
    openai: "http://127.0.0.1:8080"
    qwen: "http://127.0.0.1:8081"
`)
	t.Setenv("ONR_API_KEY", "k2")
	t.Setenv("ONR_LISTEN", ":9999")
	t.Setenv("ONR_READ_TIMEOUT_MS", "1234")
	t.Setenv("ONR_WRITE_TIMEOUT_MS", "2345")
	t.Setenv("ONR_OAUTH_TOKEN_PERSIST_ENABLED", "true")
	t.Setenv("ONR_OAUTH_TOKEN_PERSIST_DIR", "/tmp/oauth")
	t.Setenv("ONR_TRAFFIC_DUMP_ENABLED", "1")
	t.Setenv("ONR_TRAFFIC_DUMP_MAX_BYTES", "1024")
	t.Setenv("ONR_TRAFFIC_DUMP_MASK_SECRETS", "off")
	t.Setenv("ONR_UPSTREAM_PROXY_OPENAI", "http://127.0.0.1:8888")
	t.Setenv("ONR_UPSTREAM_PROXY_QWEN", "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Auth.APIKey != "k2" {
		t.Fatalf("api key not overridden")
	}
	if cfg.Server.Listen != ":9999" {
		t.Fatalf("listen not overridden: %q", cfg.Server.Listen)
	}
	if cfg.Server.ReadTimeoutMs != 1234 || cfg.Server.WriteTimeoutMs != 2345 {
		t.Fatalf("timeout not overridden: %d,%d", cfg.Server.ReadTimeoutMs, cfg.Server.WriteTimeoutMs)
	}
	if !cfg.OAuth.TokenPersist.Enabled || cfg.OAuth.TokenPersist.Dir != "/tmp/oauth" {
		t.Fatalf("oauth token persist not overridden")
	}
	if !cfg.TrafficDump.Enabled || cfg.TrafficDump.MaxBytes != 1024 || cfg.TrafficDump.MaskSecrets {
		t.Fatalf("traffic_dump not overridden: %+v", cfg.TrafficDump)
	}
	if cfg.UpstreamProxies.ByProvider["openai"] != "http://127.0.0.1:8888" {
		t.Fatalf("openai proxy not overridden")
	}
	if _, ok := cfg.UpstreamProxies.ByProvider["qwen"]; ok {
		t.Fatalf("qwen proxy should be removed by empty env override")
	}
}

func TestValidate(t *testing.T) {
	t.Run("missing api key", func(t *testing.T) {
		cfg := &Config{}
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid proxy url", func(t *testing.T) {
		cfg := &Config{}
		cfg.Auth.APIKey = "k"
		cfg.UpstreamProxies.ByProvider = map[string]string{"openai": "127.0.0.1:7890"}
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid traffic max bytes", func(t *testing.T) {
		cfg := &Config{}
		cfg.Auth.APIKey = "k"
		cfg.UpstreamProxies.ByProvider = map[string]string{}
		cfg.TrafficDump.MaxBytes = -1
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("oauth enabled requires dir", func(t *testing.T) {
		cfg := &Config{}
		cfg.Auth.APIKey = "k"
		cfg.UpstreamProxies.ByProvider = map[string]string{}
		cfg.OAuth.TokenPersist.Enabled = true
		cfg.OAuth.TokenPersist.Dir = "  "
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestHelpers(t *testing.T) {
	t.Setenv("ONR_BOOL_TRUE", "yes")
	t.Setenv("ONR_BOOL_FALSE", "off")
	t.Setenv("ONR_BOOL_UNKNOWN", "maybe")
	if !envBool("ONR_BOOL_TRUE", false) {
		t.Fatalf("expected true")
	}
	if envBool("ONR_BOOL_FALSE", true) {
		t.Fatalf("expected false")
	}
	if !envBool("ONR_BOOL_UNKNOWN", true) {
		t.Fatalf("expected default for unknown value")
	}

	in := map[string]string{
		" OpenAI ": " http://127.0.0.1:1 ",
		"":         "x",
		"qwen":     " ",
	}
	out := normalizeProviderStringMap(in)
	if len(out) != 1 || out["openai"] != "http://127.0.0.1:1" {
		t.Fatalf("unexpected normalized map: %#v", out)
	}
}
