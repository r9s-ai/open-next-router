package onrserver

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func newTestSystemLogger(t *testing.T, level string) (*logx.SystemLogger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	color := false
	l, err := logx.NewSystemLoggerWithOptions(logx.SystemLoggerOptions{
		Writer: &buf,
		Level:  level,
		Color:  &color,
	})
	if err != nil {
		t.Fatalf("new logger err=%v", err)
	}
	return l, &buf
}

func TestLogStartupSummary_ContainsRequiredFields(t *testing.T) {
	l, out := newTestSystemLogger(t, "debug")
	l.SetNowFunc(func() time.Time { return time.Date(2026, 2, 27, 9, 0, 0, 0, time.UTC) })

	cfg := &config.Config{}
	cfg.Server.Listen = ":3300"
	cfg.Providers.Dir = "./config/providers"
	cfg.Keys.File = "./keys.yaml"
	cfg.Models.File = "./models.yaml"
	cfg.TrafficDump.Enabled = true
	cfg.TrafficDump.Dir = "./dumps"
	cfg.TrafficDump.MaxBytes = 2048
	cfg.Logging.AccessLog = true
	cfg.Logging.AccessLogPath = ""
	cfg.Providers.AutoReload.Enabled = true
	cfg.Providers.AutoReload.DebounceMs = 300

	logStartupSummary(l, cfg, "./onr.yaml")
	got := out.String()
	for _, part := range []string{
		"| INFO | startup | startup config loaded",
		"| INFO | startup | startup runtime flags",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %q in log=%q", part, got)
		}
	}
	for _, part := range []string{
		"config_path=./onr.yaml",
		"providers_path=./config/providers",
		"providers_source_is_file=false",
		"keys_file=./keys.yaml",
		"models_file=./models.yaml",
		"traffic_dump_enabled=true",
		"traffic_dump_dir=./dumps",
		"traffic_dump_max_bytes=2048",
		"access_log_enabled=true",
		"access_log_target=stdout",
		"providers_auto_reload_enabled=true",
		"providers_auto_reload_debounce_ms=300",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %q in log=%q", part, got)
		}
	}
	if strings.Contains(got, "listen_url=") {
		t.Fatalf("listen_url should not be in startup summary logs: %q", got)
	}
	if strings.Contains(got, " listen=:3300") {
		t.Fatalf("unexpected listen field in startup log: %q", got)
	}
}

func TestLogSkippedProviders_WarnCategory(t *testing.T) {
	l, out := newTestSystemLogger(t, "debug")
	l.SetNowFunc(func() time.Time { return time.Date(2026, 2, 27, 9, 0, 0, 0, time.UTC) })

	logSkippedProviders(
		l,
		"./config/providers",
		[]string{"b.conf", "a.conf"},
		map[string]string{
			"a.conf": "provider \"a\" in \"a.conf\": match[0].api \"chat.bad\" is unsupported",
			"b.conf": "read b.conf: permission denied",
		},
		true,
	)
	got := out.String()
	if !strings.Contains(got, "| WARN | providers | providers skipped invalid files") {
		t.Fatalf("expected fixed warn/providers part, got=%q", got)
	}
	if !strings.Contains(got, "providers_path=./config/providers") {
		t.Fatalf("expected providers path, got=%q", got)
	}
	if !strings.Contains(got, "phase=reload") {
		t.Fatalf("expected reload phase, got=%q", got)
	}
	if !strings.Contains(got, "skipped_invalid_files=a.conf,b.conf") {
		t.Fatalf("expected skipped files, got=%q", got)
	}
	if !strings.Contains(got, "skip_reasons=\"a.conf: provider \\\"a\\\" in \\\"a.conf\\\": match[0].api \\\"chat.bad\\\" is unsupported | b.conf: read b.conf: permission denied\"") {
		t.Fatalf("expected skipped reasons, got=%q", got)
	}
}

func TestReloadLogs_CategoryAndSource(t *testing.T) {
	l, out := newTestSystemLogger(t, "debug")
	l.SetNowFunc(func() time.Time { return time.Date(2026, 2, 27, 9, 0, 0, 0, time.UTC) })

	cfg := &config.Config{}
	cfg.Providers.Dir = "./config/providers"
	cfg.Keys.File = "./keys.yaml"
	cfg.Models.File = "./models.yaml"
	cfg.Pricing.File = "./price.yaml"
	cfg.Pricing.OverridesFile = "./price_overrides.yaml"

	logReloadOK(l, "signal", cfg, providersReloadResult{ChangedProviders: []string{"openai"}})
	logReloadFailed(l, "providers_auto", errors.New("watch failed"))

	got := out.String()
	if !strings.Contains(got, "| INFO | reload | reload ok") || !strings.Contains(got, "source=signal") {
		t.Fatalf("expected reload ok with source=signal, got=%q", got)
	}
	if !strings.Contains(got, "providers_path=./config/providers") || !strings.Contains(got, "providers_source_is_file=false") {
		t.Fatalf("expected providers source fields, got=%q", got)
	}
	if !strings.Contains(got, "| ERROR | reload | reload failed") || !strings.Contains(got, "source=providers_auto") {
		t.Fatalf("expected reload failed with source=providers_auto, got=%q", got)
	}
}

func TestResolveListenURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: ":3300", want: "http://0.0.0.0:3300"},
		{in: "0.0.0.0:3300", want: "http://0.0.0.0:3300"},
		{in: "127.0.0.1:3300", want: "http://127.0.0.1:3300"},
		{in: "https://a.b", want: "https://a.b"},
	}
	for _, tc := range cases {
		got := resolveListenURL(tc.in)
		if got != tc.want {
			t.Fatalf("resolveListenURL(%q)=%q want=%q", tc.in, got, tc.want)
		}
	}
}
