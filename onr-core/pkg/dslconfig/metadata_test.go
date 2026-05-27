package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProviderFile_MetadataExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "azure-response.conf")
	content := `syntax "next-router/0.1";

provider "azure-response" {
  metadata {
    provider_family azure-openai;
    signal_profile azure-openai;
  }
  defaults {
    upstream_config {
      base_url = "https://example.openai.azure.com";
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got, want := pf.Metadata.ProviderFamily, "azure-openai"; got != want {
		t.Fatalf("ProviderFamily=%q want %q", got, want)
	}
	if got, want := pf.Metadata.SignalProfile, "azure-openai"; got != want {
		t.Fatalf("SignalProfile=%q want %q", got, want)
	}
}

func TestValidateProviderFile_MetadataDefaults(t *testing.T) {
	dir := t.TempDir()
	writeProviderConf(t, dir, "openrouter", "https://openrouter.ai/api")

	pf, err := ValidateProviderFile(filepath.Join(dir, "openrouter.conf"))
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got, want := pf.Metadata.ProviderFamily, "openrouter"; got != want {
		t.Fatalf("ProviderFamily=%q want %q", got, want)
	}
	if got, want := pf.Metadata.SignalProfile, "openrouter"; got != want {
		t.Fatalf("SignalProfile=%q want %q", got, want)
	}
}

func TestValidateProviderFile_MetadataSignalProfileDefaultsToFamily(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partner-x.conf")
	content := `provider "partner-x" {
  metadata {
    provider_family partner-aggregator;
  }
  defaults {
    upstream_config {
      base_url = "https://api.partner.example";
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got, want := pf.Metadata.ProviderFamily, "partner-aggregator"; got != want {
		t.Fatalf("ProviderFamily=%q want %q", got, want)
	}
	if got, want := pf.Metadata.SignalProfile, "partner-aggregator"; got != want {
		t.Fatalf("SignalProfile=%q want %q", got, want)
	}
}

func TestValidateProviderFile_MetadataRejectsInvalidToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	content := `provider "demo" {
  metadata {
    provider_family "Bad Value";
  }
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ValidateProviderFile(path)
	if err == nil || !strings.Contains(err.Error(), "invalid metadata provider_family") {
		t.Fatalf("ValidateProviderFile err=%v, want invalid provider_family", err)
	}
}

func TestRegistryReloadFromFile_MetadataPerProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.conf")
	content := `provider "alpha" {
  metadata {
    provider_family openai-compatible;
    signal_profile generic;
  }
  defaults {
    upstream_config {
      base_url = "https://alpha.example.com";
    }
  }
}

provider "beta" {
  defaults {
    upstream_config {
      base_url = "https://beta.example.com";
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg := NewRegistry()
	if _, err := reg.ReloadFromFile(path); err != nil {
		t.Fatalf("ReloadFromFile: %v", err)
	}
	alpha, ok := reg.GetProvider("alpha")
	if !ok {
		t.Fatalf("alpha provider missing")
	}
	if got, want := alpha.Metadata.ProviderFamily, "openai-compatible"; got != want {
		t.Fatalf("alpha ProviderFamily=%q want %q", got, want)
	}
	if got, want := alpha.Metadata.SignalProfile, "generic"; got != want {
		t.Fatalf("alpha SignalProfile=%q want %q", got, want)
	}
	beta, ok := reg.GetProvider("beta")
	if !ok {
		t.Fatalf("beta provider missing")
	}
	if got, want := beta.Metadata.ProviderFamily, "beta"; got != want {
		t.Fatalf("beta ProviderFamily=%q want %q", got, want)
	}
	if got, want := beta.Metadata.SignalProfile, "beta"; got != want {
		t.Fatalf("beta SignalProfile=%q want %q", got, want)
	}
}
