package dslconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryReloadFromDir_Basic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openai-compatible.conf"), []byte(`
syntax "next-router/0.1";

provider "openai-compatible" {
  defaults { upstream_config { base_url = "https://api.example.com"; } }
  match api = "chat.completions" stream = true {
    upstream {
      set_path "/v1/chat/completions";
      set_query x "1";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	reg := NewRegistry()
	res, err := reg.ReloadFromDir(dir)
	if err != nil {
		t.Fatalf("ReloadFromDir: %v", err)
	}
	if len(res.LoadedProviders) != 1 || res.LoadedProviders[0] != "openai-compatible" {
		t.Fatalf("unexpected loaded providers: %#v", res.LoadedProviders)
	}

	p, ok := reg.GetProvider("openai-compatible")
	if !ok {
		t.Fatalf("provider not found")
	}
	if p.Routing.BaseURLExpr != "\"https://api.example.com\"" && p.Routing.BaseURLExpr != "https://api.example.com" {
		t.Fatalf("unexpected base_url expr: %q", p.Routing.BaseURLExpr)
	}
	if len(p.Headers.Defaults.Auth) != 0 {
		t.Fatalf("unexpected default auth ops: %#v", p.Headers.Defaults.Auth)
	}
	if len(p.Routing.Matches) != 1 {
		t.Fatalf("unexpected matches: %#v", p.Routing.Matches)
	}
	if p.Routing.Matches[0].API != "chat.completions" {
		t.Fatalf("unexpected api: %q", p.Routing.Matches[0].API)
	}
	if p.Routing.Matches[0].SetPath != "\"/v1/chat/completions\"" && p.Routing.Matches[0].SetPath != "/v1/chat/completions" {
		// parser keeps raw expr; tolerate both forms across future tweaks
		t.Fatalf("unexpected set_path: %q", p.Routing.Matches[0].SetPath)
	}
	if p.Routing.Matches[0].QueryPairs["x"] != "\"1\"" && p.Routing.Matches[0].QueryPairs["x"] != "1" {
		t.Fatalf("unexpected set_query: %q", p.Routing.Matches[0].QueryPairs["x"])
	}
}

func TestRegistryReloadFromDir_Include(t *testing.T) {
	dir := t.TempDir()
	includeDir := filepath.Join(dir, "_include")
	if err := os.MkdirAll(includeDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(includeDir, "common.conf"), []byte(`
# common defaults
`), 0o600); err != nil {
		t.Fatalf("write include file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai-compatible.conf"), []byte(`
include "_include/common.conf";
provider "openai-compatible" { defaults { upstream_config { base_url = "https://api.example.com"; } } }
`), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	reg := NewRegistry()
	if _, err := reg.ReloadFromDir(dir); err != nil {
		t.Fatalf("ReloadFromDir: %v", err)
	}
}

func TestRegistryReloadFromDir_ParseAuthAndHeaders(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openai-compatible.conf"), []byte(`
syntax "next-router/0.1";

provider "openai-compatible" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    auth { auth_bearer; }
    request { set_header "x-test" "1"; }
  }
  match api = "chat.completions" { request { del_header "x-test"; } }
}
`), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
	reg := NewRegistry()
	if _, err := reg.ReloadFromDir(dir); err != nil {
		t.Fatalf("ReloadFromDir: %v", err)
	}
	p, ok := reg.GetProvider("openai-compatible")
	if !ok {
		t.Fatalf("provider not found")
	}
	if len(p.Headers.Defaults.Auth) == 0 {
		t.Fatalf("expected default auth ops")
	}
	if len(p.Headers.Defaults.Request) == 0 {
		t.Fatalf("expected default request header ops")
	}
}

func TestRegistryReloadFromDir_RejectOldSyntax(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openai-compatible.conf"), []byte(`
syntax "next-router/0.1";

provider "openai-compatible" {
  defaults {
    request { header_set { name = "x-test"; value = "1"; }; }
  }
}
`), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
	reg := NewRegistry()
	res, err := reg.ReloadFromDir(dir)
	if err != nil {
		t.Fatalf("ReloadFromDir: %v", err)
	}
	if len(res.LoadedProviders) != 0 {
		t.Fatalf("expected no loaded providers, got: %#v", res.LoadedProviders)
	}
	if len(res.SkippedFiles) != 1 || res.SkippedFiles[0] != "openai-compatible.conf" {
		t.Fatalf("expected skipped openai-compatible.conf, got: %#v", res.SkippedFiles)
	}
	if _, ok := reg.GetProvider("openai-compatible"); ok {
		t.Fatalf("unexpected provider loaded")
	}
}
