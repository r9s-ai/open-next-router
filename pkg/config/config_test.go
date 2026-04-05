package config

import (
	"os"
	"path/filepath"
	"reflect"
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
	if cfg.Auth.TokenKey.AllowBYOKWithoutK {
		t.Fatalf("auth.token_key.allow_byok_without_k default should be false")
	}
	if cfg.Keys.File == "" || cfg.Models.File == "" {
		t.Fatalf("expected default paths")
	}
	if cfg.Providers.Dir != "" {
		t.Fatalf("providers.dir default should be empty, got %q", cfg.Providers.Dir)
	}
	if cfg.Providers.AutoReload.Enabled {
		t.Fatalf("providers.auto_reload.enabled default should be false")
	}
	if cfg.Providers.AutoReload.DebounceMs != 300 {
		t.Fatalf("providers.auto_reload.debounce_ms default=%d", cfg.Providers.AutoReload.DebounceMs)
	}
	if !cfg.TrafficDump.MaskSecrets {
		t.Fatalf("mask_secrets default should be true")
	}
	if len(cfg.TrafficDump.Sections) != 0 {
		t.Fatalf("traffic_dump.sections default should be empty, got=%v", cfg.TrafficDump.Sections)
	}
	if !cfg.Logging.AccessLog {
		t.Fatalf("access_log default should be true")
	}
	if cfg.Logging.AppNameInfer.Enabled {
		t.Fatalf("logging.appname_infer.enabled default should be false")
	}
	if cfg.Logging.AccessLogRotate.Enabled {
		t.Fatalf("logging.access_log_rotate.enabled default should be false")
	}
	if cfg.Logging.AccessLogRotate.MaxSizeMB != 100 {
		t.Fatalf("logging.access_log_rotate.max_size_mb default=%d", cfg.Logging.AccessLogRotate.MaxSizeMB)
	}
	if cfg.Logging.AccessLogRotate.MaxBackups != 14 {
		t.Fatalf("logging.access_log_rotate.max_backups default=%d", cfg.Logging.AccessLogRotate.MaxBackups)
	}
	if cfg.Logging.AccessLogRotate.MaxAgeDays != 14 {
		t.Fatalf("logging.access_log_rotate.max_age_days default=%d", cfg.Logging.AccessLogRotate.MaxAgeDays)
	}
	if cfg.Logging.AccessLogRotate.Compress {
		t.Fatalf("logging.access_log_rotate.compress default should be false")
	}
}

func TestResolveProviderDSLSource_DefaultsToOnrConfWhenPresent(t *testing.T) {
	t.Setenv("ONR_PROVIDERS_DIR", "")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "onr.conf"), []byte(`syntax "next-router/0.1";`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()

	path, isFile := ResolveProviderDSLSource(&Config{})
	if path != DefaultProvidersDSLFile || !isFile {
		t.Fatalf("ResolveProviderDSLSource got (%q,%v), want (%q,true)", path, isFile, DefaultProvidersDSLFile)
	}
}

func TestResolveProviderDSLWatchDir_DefaultsToConfigDirWhenOnrConfPresent(t *testing.T) {
	t.Setenv("ONR_PROVIDERS_DIR", "")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "onr.conf"), []byte(`syntax "next-router/0.1";`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()

	if got, want := ResolveProviderDSLWatchDir(&Config{}), "config"; got != want {
		t.Fatalf("ResolveProviderDSLWatchDir=%q want=%q", got, want)
	}
}

func TestResolveProviderDSLWatchDir_ExplicitFileUsesParentDir(t *testing.T) {
	cfg := &Config{}
	cfg.Providers.Dir = "./config/onr.conf"
	if got, want := ResolveProviderDSLWatchDir(cfg), filepath.Clean("./config"); got != want {
		t.Fatalf("ResolveProviderDSLWatchDir=%q want=%q", got, want)
	}
}

func TestResolveProviderDSLSource_ExplicitFilePathMarkedAsFile(t *testing.T) {
	cfg := &Config{}
	cfg.Providers.Dir = "./config/onr.conf"
	path, isFile := ResolveProviderDSLSource(cfg)
	if path != "./config/onr.conf" || !isFile {
		t.Fatalf("ResolveProviderDSLSource got (%q,%v), want (./config/onr.conf,true)", path, isFile)
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
	t.Setenv("ONR_TOKEN_KEY_ALLOW_BYOK_WITHOUT_K", "true")
	t.Setenv("ONR_LISTEN", ":9999")
	t.Setenv("ONR_PROVIDERS_AUTO_RELOAD_ENABLED", "1")
	t.Setenv("ONR_PROVIDERS_AUTO_RELOAD_DEBOUNCE_MS", "450")
	t.Setenv("ONR_READ_TIMEOUT_MS", "1234")
	t.Setenv("ONR_WRITE_TIMEOUT_MS", "2345")
	t.Setenv("ONR_OAUTH_TOKEN_PERSIST_ENABLED", "true")
	t.Setenv("ONR_OAUTH_TOKEN_PERSIST_DIR", "/tmp/oauth")
	t.Setenv("ONR_TRAFFIC_DUMP_ENABLED", "1")
	t.Setenv("ONR_TRAFFIC_DUMP_MAX_BYTES", "1024")
	t.Setenv("ONR_TRAFFIC_DUMP_MASK_SECRETS", "off")
	t.Setenv("ONR_TRAFFIC_DUMP_SECTIONS", "META, origin_request, upstream_response")
	t.Setenv("ONR_UPSTREAM_PROXY_OPENAI", "http://127.0.0.1:8888")
	t.Setenv("ONR_UPSTREAM_PROXY_QWEN", "")
	t.Setenv("ONR_ACCESS_LOG_PATH", "/tmp/access.log")
	t.Setenv("ONR_LOG_LEVEL", "WARN")
	t.Setenv("ONR_ACCESS_LOG_FORMAT", "$method $path")
	t.Setenv("ONR_ACCESS_LOG_FORMAT_PRESET", "onr_minimal")
	t.Setenv("ONR_ACCESS_LOG_ROTATE_ENABLED", "true")
	t.Setenv("ONR_ACCESS_LOG_ROTATE_MAX_SIZE_MB", "128")
	t.Setenv("ONR_ACCESS_LOG_ROTATE_MAX_BACKUPS", "30")
	t.Setenv("ONR_ACCESS_LOG_ROTATE_MAX_AGE_DAYS", "7")
	t.Setenv("ONR_ACCESS_LOG_ROTATE_COMPRESS", "1")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if cfg.Auth.APIKey != "k2" {
		t.Fatalf("api key not overridden")
	}
	if !cfg.Auth.TokenKey.AllowBYOKWithoutK {
		t.Fatalf("token_key allow_byok_without_k not overridden")
	}
	if cfg.Server.Listen != ":9999" {
		t.Fatalf("listen not overridden: %q", cfg.Server.Listen)
	}
	if cfg.Server.ReadTimeoutMs != 1234 || cfg.Server.WriteTimeoutMs != 2345 {
		t.Fatalf("timeout not overridden: %d,%d", cfg.Server.ReadTimeoutMs, cfg.Server.WriteTimeoutMs)
	}
	if !cfg.Providers.AutoReload.Enabled || cfg.Providers.AutoReload.DebounceMs != 450 {
		t.Fatalf("providers auto_reload not overridden: %+v", cfg.Providers.AutoReload)
	}
	if !cfg.OAuth.TokenPersist.Enabled || cfg.OAuth.TokenPersist.Dir != "/tmp/oauth" {
		t.Fatalf("oauth token persist not overridden")
	}
	if !cfg.TrafficDump.Enabled || cfg.TrafficDump.MaxBytes != 1024 || cfg.TrafficDump.MaskSecrets {
		t.Fatalf("traffic_dump not overridden: %+v", cfg.TrafficDump)
	}
	if !reflect.DeepEqual(cfg.TrafficDump.Sections, []string{"meta", "origin_request", "upstream_response"}) {
		t.Fatalf("traffic_dump.sections not overridden/normalized: %v", cfg.TrafficDump.Sections)
	}
	if cfg.UpstreamProxies.ByProvider["openai"] != "http://127.0.0.1:8888" {
		t.Fatalf("openai proxy not overridden")
	}
	if _, ok := cfg.UpstreamProxies.ByProvider["qwen"]; ok {
		t.Fatalf("qwen proxy should be removed by empty env override")
	}
	if cfg.Logging.AccessLogFormat != "$method $path" {
		t.Fatalf("access_log_format not overridden: %q", cfg.Logging.AccessLogFormat)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("logging.level not overridden/normalized: %q", cfg.Logging.Level)
	}
	if cfg.Logging.AccessLogFormatPreset != "onr_minimal" {
		t.Fatalf("access_log_format_preset not overridden: %q", cfg.Logging.AccessLogFormatPreset)
	}
	if !cfg.Logging.AccessLogRotate.Enabled {
		t.Fatalf("access_log_rotate.enabled not overridden")
	}
	if cfg.Logging.AccessLogRotate.MaxSizeMB != 128 {
		t.Fatalf("access_log_rotate.max_size_mb not overridden: %d", cfg.Logging.AccessLogRotate.MaxSizeMB)
	}
	if cfg.Logging.AccessLogRotate.MaxBackups != 30 {
		t.Fatalf("access_log_rotate.max_backups not overridden: %d", cfg.Logging.AccessLogRotate.MaxBackups)
	}
	if cfg.Logging.AccessLogRotate.MaxAgeDays != 7 {
		t.Fatalf("access_log_rotate.max_age_days not overridden: %d", cfg.Logging.AccessLogRotate.MaxAgeDays)
	}
	if !cfg.Logging.AccessLogRotate.Compress {
		t.Fatalf("access_log_rotate.compress not overridden")
	}
}

func TestLoad_AccessLogRotateYAMLValidation(t *testing.T) {
	t.Run("explicit zero max_size_mb should fail", func(t *testing.T) {
		path := writeConfigFile(t, `
auth:
  api_key: "k"
logging:
  access_log: true
  access_log_path: "./logs/access.log"
  access_log_rotate:
    enabled: true
    max_size_mb: 0
`)
		if _, err := Load(path); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("explicit zero max_backups should fail", func(t *testing.T) {
		path := writeConfigFile(t, `
auth:
  api_key: "k"
logging:
  access_log: true
  access_log_path: "./logs/access.log"
  access_log_rotate:
    enabled: true
    max_backups: 0
`)
		if _, err := Load(path); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("max_age_days can be zero", func(t *testing.T) {
		path := writeConfigFile(t, `
auth:
  api_key: "k"
logging:
  access_log: true
  access_log_path: "./logs/access.log"
  access_log_rotate:
    enabled: true
    max_age_days: 0
`)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load err=%v", err)
		}
		if cfg.Logging.AccessLogRotate.MaxAgeDays != 0 {
			t.Fatalf("expected max_age_days=0, got %d", cfg.Logging.AccessLogRotate.MaxAgeDays)
		}
	})
}

func TestValidate(t *testing.T) {
	newValidConfig := func() *Config {
		cfg := &Config{}
		cfg.UpstreamProxies.ByProvider = map[string]string{}
		cfg.Logging.AccessLogRotate.MaxSizeMB = 100
		cfg.Logging.AccessLogRotate.MaxBackups = 14
		cfg.Logging.AccessLogRotate.MaxAgeDays = 14
		return cfg
	}

	t.Run("invalid proxy url", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.UpstreamProxies.ByProvider = map[string]string{"openai": "127.0.0.1:7890"}
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid traffic max bytes", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.TrafficDump.MaxBytes = -1
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("providers auto reload requires positive debounce", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Providers.AutoReload.Enabled = true
		cfg.Providers.AutoReload.DebounceMs = 0
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("oauth enabled requires dir", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.OAuth.TokenPersist.Enabled = true
		cfg.OAuth.TokenPersist.Dir = "  "
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("access log rotate enabled requires access log", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Logging.AccessLog = false
		cfg.Logging.AccessLogPath = "./logs/access.log"
		cfg.Logging.AccessLogRotate.Enabled = true
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("access log rotate enabled requires non-empty path", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Logging.AccessLog = true
		cfg.Logging.AccessLogPath = " "
		cfg.Logging.AccessLogRotate.Enabled = true
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid access log rotate max_size_mb", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Logging.AccessLogRotate.MaxSizeMB = 0
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid access log rotate max_backups", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Logging.AccessLogRotate.MaxBackups = 0
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid access log rotate max_age_days", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Logging.AccessLogRotate.MaxAgeDays = -1
		if err := validate(cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid logging level", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Logging.Level = "verbose"
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

func TestLoad_TrafficDumpSectionsYAML(t *testing.T) {
	t.Run("valid and normalized", func(t *testing.T) {
		path := writeConfigFile(t, `
auth:
  api_key: "k"
traffic_dump:
  sections: ["META", "origin_request", "origin_request", "upstream_response"]
`)
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load err=%v", err)
		}
		want := []string{"meta", "origin_request", "upstream_response"}
		if !reflect.DeepEqual(cfg.TrafficDump.Sections, want) {
			t.Fatalf("sections=%v want=%v", cfg.TrafficDump.Sections, want)
		}
	})

	t.Run("invalid section should fail", func(t *testing.T) {
		path := writeConfigFile(t, `
auth:
  api_key: "k"
traffic_dump:
  sections: ["meta", "bad_section"]
`)
		if _, err := Load(path); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestLoad_TrafficDumpSectionsEnvInvalid(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  api_key: "k"
traffic_dump:
  sections: ["meta"]
`)
	t.Setenv("ONR_TRAFFIC_DUMP_SECTIONS", "meta,not_exists")
	if _, err := Load(path); err == nil {
		t.Fatalf("expected error")
	}
}
