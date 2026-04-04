package dslconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationIssue_UnwrapAndError(t *testing.T) {
	base := errors.New("boom")
	v := &ValidationIssue{Directive: "req_map", Scope: "defaults.request", Err: base}
	if !errors.Is(v, base) {
		t.Fatalf("errors.Is should match wrapped err")
	}
	if got := v.Error(); got != "boom" {
		t.Fatalf("Error=%q want=boom", got)
	}

	var nilIssue *ValidationIssue
	if nilIssue.Unwrap() != nil {
		t.Fatalf("nil issue Unwrap should return nil")
	}
	if nilIssue.Error() != "" {
		t.Fatalf("nil issue Error should be empty string")
	}
}

func TestRegistryListProviderNames_Sorted(t *testing.T) {
	dir := t.TempDir()
	writeProviderConf(t, dir, "zeta", "https://zeta.example.com")
	writeProviderConf(t, dir, "alpha", "https://alpha.example.com")

	reg := NewRegistry()
	if _, err := reg.ReloadFromDir(dir); err != nil {
		t.Fatalf("ReloadFromDir: %v", err)
	}
	got := reg.ListProviderNames()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("unexpected sorted names: %#v", got)
	}
}

func TestDefaultRegistryAndReloadDefault(t *testing.T) {
	dir := t.TempDir()
	writeProviderConf(t, dir, "openai-compatible", "https://api.example.com")

	r1 := DefaultRegistry()
	r2 := DefaultRegistry()
	if r1 != r2 {
		t.Fatalf("DefaultRegistry should return singleton instance")
	}

	if err := ReloadDefault(dir); err != nil {
		t.Fatalf("ReloadDefault: %v", err)
	}
	if _, ok := DefaultRegistry().GetProvider("openai-compatible"); !ok {
		t.Fatalf("provider should exist in default registry after reload")
	}
}

func TestPreprocessIncludes_UnquotedDirInclude(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeProviderConf(t, providersDir, "alpha", "https://alpha.example.com")
	writeProviderConf(t, providersDir, "beta", "https://beta.example.com")

	content, err := preprocessIncludes(filepath.Join(root, "onr.conf"), "include providers;\n")
	if err != nil {
		t.Fatalf("preprocessIncludes: %v", err)
	}
	if !strings.Contains(content, `provider "alpha"`) || !strings.Contains(content, `provider "beta"`) {
		t.Fatalf("unexpected expanded content: %q", content)
	}
}

func TestPreprocessIncludes_UnquotedGlobInclude(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "parts")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.conf"), []byte("syntax \"next-router/0.1\";\n"), 0o600); err != nil {
		t.Fatalf("WriteFile a.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.conf"), []byte("provider \"demo\" { defaults { upstream_config { base_url = \"https://api.example.com\"; } } }\n"), 0o600); err != nil {
		t.Fatalf("WriteFile b.conf: %v", err)
	}

	content, err := preprocessIncludes(filepath.Join(root, "onr.conf"), "include parts/*.conf;\n")
	if err != nil {
		t.Fatalf("preprocessIncludes: %v", err)
	}
	if !strings.Contains(content, `syntax "next-router/0.1";`) || !strings.Contains(content, `provider "demo"`) {
		t.Fatalf("unexpected expanded content: %q", content)
	}
}

func TestResolvedOAuthConfigCacheIdentity_StableOrder(t *testing.T) {
	a := ResolvedOAuthConfig{
		Mode:              "openai",
		Method:            "POST",
		ContentType:       "form",
		TokenURL:          "https://token.example.com",
		TokenPath:         "$.access_token",
		ExpiresInPath:     "$.expires_in",
		TokenTypePath:     "$.token_type",
		TimeoutMs:         5000,
		RefreshSkewSec:    60,
		FallbackTTLSec:    1800,
		BasicAuthUsername: "u",
		BasicAuthPassword: "p",
		Form: map[string]string{
			"b": "2",
			"a": "1",
		},
	}
	b := a
	b.Form = map[string]string{
		"a": "1",
		"b": "2",
	}

	if a.CacheIdentity() != b.CacheIdentity() {
		t.Fatalf("cache identity should be stable regardless of map insertion order")
	}

	b.Form["b"] = "3"
	if a.CacheIdentity() == b.CacheIdentity() {
		t.Fatalf("cache identity should change when form value changes")
	}
}

func writeProviderConf(t *testing.T, dir, name, baseURL string) {
	t.Helper()
	content := `syntax "next-router/0.1";

provider "` + name + `" {
  defaults {
    upstream_config {
      base_url = "` + baseURL + `";
    }
  }
}`
	path := filepath.Join(dir, name+".conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
}
