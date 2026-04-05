package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleProvidersPath_FileSource(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	modesDir := filepath.Join(root, "modes")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll providers: %v", err)
	}
	if err := os.MkdirAll(modesDir, 0o750); err != nil {
		t.Fatalf("MkdirAll modes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modesDir, "usage.conf"), []byte(`
usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.prompt_tokens";
  usage_fact output token path="$.usage.completion_tokens";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile usage.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.openai.com";
    }
    metrics {
      usage_extract shared_tokens;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile openai.conf: %v", err)
	}
	onrPath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(onrPath, []byte(`
syntax "next-router/0.1";
include modes/*.conf;
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}

	content, err := BundleProvidersPath(onrPath)
	if err != nil {
		t.Fatalf("BundleProvidersPath: %v", err)
	}
	if !strings.Contains(content, `usage_mode "shared_tokens"`) {
		t.Fatalf("bundled content should include usage mode, got=%q", content)
	}
	if !strings.Contains(content, `provider "openai"`) {
		t.Fatalf("bundled content should include provider, got=%q", content)
	}

	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		t.Fatalf("MkdirAll out: %v", err)
	}
	bundledPath := filepath.Join(outDir, "providers.conf")
	if err := os.WriteFile(bundledPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile bundled: %v", err)
	}
	res, err := ValidateProvidersFile(bundledPath)
	if err != nil {
		t.Fatalf("ValidateProvidersFile: %v", err)
	}
	if len(res.LoadedProviders) != 1 || res.LoadedProviders[0] != "openai" {
		t.Fatalf("LoadedProviders=%v want=[openai]", res.LoadedProviders)
	}
}

func TestBundleProvidersPath_DirectorySourceSkipsDuplicateProviderBody(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll providers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "openai_local" {
  usage_extract custom;
  usage_fact input token path="$.usage.prompt_tokens";
  usage_fact output token path="$.usage.completion_tokens";
}

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.openai.com";
    }
    metrics {
      usage_extract openai_local;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile openai.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

include providers/*.conf;

provider "ignored" {
  defaults {
    upstream_config {
      base_url = "https://ignored.example.com";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}

	content, err := BundleProvidersPath(providersDir)
	if err != nil {
		t.Fatalf("BundleProvidersPath: %v", err)
	}
	if strings.Count(content, `usage_mode "openai_local"`) != 1 {
		t.Fatalf("bundled content should contain one openai_local mode, got=%q", content)
	}
	if strings.Count(content, `provider "openai"`) != 1 {
		t.Fatalf("bundled content should contain one openai provider block, got=%q", content)
	}
	if strings.Contains(content, `provider "ignored"`) {
		t.Fatalf("bundled content should not include ignored provider from global file, got=%q", content)
	}

	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		t.Fatalf("MkdirAll out: %v", err)
	}
	bundledPath := filepath.Join(outDir, "providers.conf")
	if err := os.WriteFile(bundledPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile bundled: %v", err)
	}
	res, err := ValidateProvidersFile(bundledPath)
	if err != nil {
		t.Fatalf("ValidateProvidersFile: %v", err)
	}
	if len(res.LoadedProviders) != 1 || res.LoadedProviders[0] != "openai" {
		t.Fatalf("LoadedProviders=%v want=[openai]", res.LoadedProviders)
	}
}
