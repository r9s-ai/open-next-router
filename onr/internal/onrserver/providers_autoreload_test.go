package onrserver

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func TestShouldTriggerProviderReload(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		if shouldTriggerProviderReload(fsnotify.Event{Name: "", Op: fsnotify.Write}) {
			t.Fatalf("expected false for empty event name")
		}
	})

	t.Run("unsupported op", func(t *testing.T) {
		if shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/a.conf", Op: 0}) {
			t.Fatalf("expected false for unsupported op")
		}
	})

	t.Run("dot file ignored", func(t *testing.T) {
		if shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/.a.conf", Op: fsnotify.Write}) {
			t.Fatalf("expected false for dotfile")
		}
	})

	t.Run("conf write", func(t *testing.T) {
		if !shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/a.conf", Op: fsnotify.Write}) {
			t.Fatalf("expected true for conf write")
		}
	})

	t.Run("remove", func(t *testing.T) {
		if !shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/a.conf", Op: fsnotify.Remove}) {
			t.Fatalf("expected true for remove")
		}
	})
}

func TestReloadProvidersRuntime_GlobalUsageModeChangeMarksProviderChanged(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()

	configDir := filepath.Join(root, "config")
	providersDir := filepath.Join(configDir, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "onr.conf"), []byte(`
syntax "next-router/0.1";

include modes/*.conf;
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	modesDir := filepath.Join(configDir, "modes")
	if err := os.MkdirAll(modesDir, 0o750); err != nil {
		t.Fatalf("MkdirAll modes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modesDir, "usage_modes.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.in_v1";
  usage_fact output token path="$.usage.out_v1";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile usage_modes.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "demo.conf"), []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    metrics { usage_extract shared_tokens; }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile demo.conf: %v", err)
	}

	reg := dslconfig.NewRegistry()
	if _, err := reg.ReloadFromPath(config.DefaultProvidersDSLFile); err != nil {
		t.Fatalf("ReloadFromPath initial: %v", err)
	}
	before, ok := reg.GetProvider("demo")
	if !ok {
		t.Fatalf("expected provider demo")
	}
	if got, want := before.Usage.Defaults.CompiledPlan(nil).Facts[0].Path, "$.usage.in_v1"; got != want {
		t.Fatalf("compiled input path=%q want=%q", got, want)
	}

	if err := os.WriteFile(filepath.Join(modesDir, "usage_modes.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.in_v2";
  usage_fact output token path="$.usage.out_v2";
}
`), 0o600); err != nil {
		t.Fatalf("Rewrite usage_modes.conf: %v", err)
	}

	logger, _ := newTestSystemLogger(t, "debug")
	res, err := reloadProvidersRuntime(&config.Config{}, reg, logger)
	if err != nil {
		t.Fatalf("reloadProvidersRuntime: %v", err)
	}
	if !reflect.DeepEqual(res.ChangedProviders, []string{"demo"}) {
		t.Fatalf("changed providers=%v want=[demo]", res.ChangedProviders)
	}

	after, ok := reg.GetProvider("demo")
	if !ok {
		t.Fatalf("expected provider demo after reload")
	}
	plan := after.Usage.Defaults.CompiledPlan(nil)
	if got, want := plan.Facts[0].Path, "$.usage.in_v2"; got != want {
		t.Fatalf("reloaded input path=%q want=%q", got, want)
	}
	if got, want := plan.Facts[1].Path, "$.usage.out_v2"; got != want {
		t.Fatalf("reloaded output path=%q want=%q", got, want)
	}
}
