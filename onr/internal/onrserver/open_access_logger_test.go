package onrserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func TestOpenAccessLogger_RotateEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	cfg := &config.Config{}
	cfg.Logging.AccessLog = true
	cfg.Logging.AccessLogPath = path
	cfg.Logging.AccessLogRotate.Enabled = true
	cfg.Logging.AccessLogRotate.MaxSizeMB = 1
	cfg.Logging.AccessLogRotate.MaxBackups = 2
	cfg.Logging.AccessLogRotate.MaxAgeDays = 14

	l, closer, color, err := openAccessLogger(cfg)
	if err != nil {
		t.Fatalf("openAccessLogger err=%v", err)
	}
	if l == nil {
		t.Fatalf("expected logger")
	}
	if l.Flags() != 0 {
		t.Fatalf("expected logger flags=0, got=%d", l.Flags())
	}
	if color {
		t.Fatalf("expected color disabled for file logger")
	}
	if _, ok := closer.(*logx.AccessRotateWriter); !ok {
		t.Fatalf("expected AccessRotateWriter closer, got %T", closer)
	}

	l.Println("hello")
	if err := closer.Close(); err != nil {
		t.Fatalf("close err=%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected active log file, stat err=%v", err)
	}
}

func TestOpenAccessLogger_RotateDisabledFileAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")
	cfg := &config.Config{}
	cfg.Logging.AccessLog = true
	cfg.Logging.AccessLogPath = path

	l, closer, color, err := openAccessLogger(cfg)
	if err != nil {
		t.Fatalf("openAccessLogger err=%v", err)
	}
	if l == nil {
		t.Fatalf("expected logger")
	}
	if l.Flags() != 0 {
		t.Fatalf("expected logger flags=0, got=%d", l.Flags())
	}
	if color {
		t.Fatalf("expected color disabled for file logger")
	}
	if _, ok := closer.(*os.File); !ok {
		t.Fatalf("expected os.File closer, got %T", closer)
	}

	l.Println("hello")
	if err := closer.Close(); err != nil {
		t.Fatalf("close err=%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected log file, stat err=%v", err)
	}
}

func TestOpenAccessLogger_RotateEnabledEmptyPath(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.AccessLog = true
	cfg.Logging.AccessLogPath = ""
	cfg.Logging.AccessLogRotate.Enabled = true
	cfg.Logging.AccessLogRotate.MaxSizeMB = 1
	cfg.Logging.AccessLogRotate.MaxBackups = 2
	cfg.Logging.AccessLogRotate.MaxAgeDays = 14

	if _, _, _, err := openAccessLogger(cfg); err == nil {
		t.Fatalf("expected error")
	}
}

func TestOpenAccessLogger_StdoutNoStdTimePrefix(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logging.AccessLog = true
	cfg.Logging.AccessLogPath = ""

	l, closer, color, err := openAccessLogger(cfg)
	if err != nil {
		t.Fatalf("openAccessLogger err=%v", err)
	}
	if closer != nil {
		t.Fatalf("expected nil closer for stdout logger")
	}
	if !color {
		t.Fatalf("expected color enabled for stdout logger")
	}
	if l == nil {
		t.Fatalf("expected logger")
	}
	if l.Flags() != 0 {
		t.Fatalf("expected logger flags=0, got=%d", l.Flags())
	}
}
