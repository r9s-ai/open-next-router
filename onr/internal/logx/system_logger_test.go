package logx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseSystemLogLevel(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		cases := []struct {
			in   string
			want SystemLogLevel
		}{
			{in: "", want: SystemLogLevelInfo},
			{in: "debug", want: SystemLogLevelDebug},
			{in: "INFO", want: SystemLogLevelInfo},
			{in: "Warn", want: SystemLogLevelWarn},
			{in: "error", want: SystemLogLevelError},
		}
		for _, tc := range cases {
			got, err := ParseSystemLogLevel(tc.in)
			if err != nil {
				t.Fatalf("ParseSystemLogLevel(%q) err=%v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseSystemLogLevel(%q)=%v want=%v", tc.in, got, tc.want)
			}
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := ParseSystemLogLevel("verbose"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestSystemLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	color := false
	l, err := NewSystemLoggerWithOptions(SystemLoggerOptions{
		Writer: &buf,
		Level:  "info",
		Color:  &color,
	})
	if err != nil {
		t.Fatalf("NewSystemLoggerWithOptions err=%v", err)
	}
	l.now = func() time.Time { return time.Date(2026, 2, 27, 8, 0, 0, 0, time.UTC) }

	l.Debug(SystemCategoryServer, "debug hidden", nil)
	l.Info(SystemCategoryServer, "info shown", nil)
	l.Warn(SystemCategoryServer, "warn shown", nil)

	out := buf.String()
	if strings.Contains(out, "debug hidden") {
		t.Fatalf("debug should be filtered, got=%q", out)
	}
	if !strings.Contains(out, "| INFO | server | info shown") {
		t.Fatalf("missing info line, got=%q", out)
	}
	if !strings.Contains(out, "| WARN | server | warn shown") {
		t.Fatalf("missing warn line, got=%q", out)
	}
}

func TestSystemLogger_FormatKVAndEscape(t *testing.T) {
	var buf bytes.Buffer
	color := false
	l, err := NewSystemLoggerWithOptions(SystemLoggerOptions{
		Writer: &buf,
		Level:  "debug",
		Color:  &color,
	})
	if err != nil {
		t.Fatalf("NewSystemLoggerWithOptions err=%v", err)
	}
	l.now = func() time.Time { return time.Date(2026, 2, 27, 8, 1, 2, 3, time.UTC) }

	l.Info(SystemCategoryStartup, "startup ok", map[string]any{
		"listen":        ":3300",
		"listen_url":    "http://127.0.0.1:3300",
		"empty":         "  ",
		"notes":         "line1\nline2",
		"providers_dir": "./config/providers",
		"traffic_dump":  false,
		"debounce_ms":   300,
	})

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatalf("expected output")
	}
	if !strings.Contains(out, "[ONR] 2026/02/27 - 08:01:02 | INFO | startup | startup ok") {
		t.Fatalf("unexpected fixed part, got=%q", out)
	}
	if strings.Contains(out, " empty=") {
		t.Fatalf("empty string field should be omitted, got=%q", out)
	}
	if !strings.Contains(out, "notes=\"line1\\nline2\"") {
		t.Fatalf("expected escaped newline, got=%q", out)
	}
	if !strings.Contains(out, "listen=:3300") {
		t.Fatalf("expected listen field, got=%q", out)
	}
}

func TestSystemLogger_ColorizedLevel(t *testing.T) {
	var buf bytes.Buffer
	color := true
	l, err := NewSystemLoggerWithOptions(SystemLoggerOptions{
		Writer: &buf,
		Level:  "debug",
		Color:  &color,
	})
	if err != nil {
		t.Fatalf("NewSystemLoggerWithOptions err=%v", err)
	}
	l.now = func() time.Time { return time.Date(2026, 2, 27, 8, 0, 0, 0, time.UTC) }

	l.Warn(SystemCategoryServer, "warn", nil)
	out := buf.String()
	if !strings.Contains(out, "\x1b[33mWARN\x1b[0m") {
		t.Fatalf("expected ANSI warn level, got=%q", out)
	}
}

func TestShouldEnableSystemLogColor(t *testing.T) {
	if !shouldEnableSystemLogColor(true, "") {
		t.Fatalf("expected color enabled on terminal without NO_COLOR")
	}
	if shouldEnableSystemLogColor(false, "") {
		t.Fatalf("expected color disabled on non-terminal")
	}
	if shouldEnableSystemLogColor(true, "1") {
		t.Fatalf("expected color disabled when NO_COLOR is set")
	}
}
