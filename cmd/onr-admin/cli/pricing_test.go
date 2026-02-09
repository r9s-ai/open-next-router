package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
)

func TestParseProviderTargets(t *testing.T) {
	out, err := parseProviderTargets("", "openai, gemini;openai")
	if err != nil {
		t.Fatalf("parseProviderTargets err=%v", err)
	}
	if len(out) != 2 || out[0] != "gemini" || out[1] != "openai" {
		t.Fatalf("out=%v", out)
	}
}

func TestParseProviderTargetsMissing(t *testing.T) {
	_, err := parseProviderTargets("", "")
	if err == nil {
		t.Fatalf("expect error")
	}
}

func TestParseCSVList(t *testing.T) {
	out := parseCSVList("a, b; c\t d\n")
	if len(out) != 4 || out[0] != "a" || out[3] != "d" {
		t.Fatalf("out=%v", out)
	}
}

func TestBuildProviderRows(t *testing.T) {
	catalog := pricing.Catalog{
		Providers: map[string]pricing.Provider{
			"openai": {ID: "openai", Name: "OpenAI", Models: map[string]pricing.Model{"gpt-4o-mini": {ID: "gpt-4o-mini"}}},
			"google": {ID: "google", Name: "Google AI", Models: map[string]pricing.Model{"gemini-2.5-flash": {ID: "gemini-2.5-flash"}}},
		},
	}
	rows := buildProviderRows(catalog, "")
	if len(rows) != 2 {
		t.Fatalf("rows=%v", rows)
	}
	rows = buildProviderRows(catalog, "goo")
	if len(rows) != 1 {
		t.Fatalf("filtered rows=%v", rows)
	}
	if got := rows[0]; got == "" || got[:15] != "provider=google" {
		t.Fatalf("row=%q", got)
	}
}

func TestResolvePricingSyncProvidersFromConfig(t *testing.T) {
	dir := t.TempDir()
	providersDir := filepath.Join(dir, "providers")
	if err := os.MkdirAll(providersDir, 0o755); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	providerConf := `
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
  }
}
`
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(providerConf), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}
	cfgPath := filepath.Join(dir, "onr.yaml")
	cfgYAML := "auth:\n  api_key: \"x\"\nproviders:\n  dir: \"" + providersDir + "\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, err := resolvePricingSyncProviders(pricingSyncOptions{cfgPath: cfgPath})
	if err != nil {
		t.Fatalf("resolvePricingSyncProviders err=%v", err)
	}
	if len(out) != 1 || out[0] != "openai" {
		t.Fatalf("providers=%v", out)
	}
}
