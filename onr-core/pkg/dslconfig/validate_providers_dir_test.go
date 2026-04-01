package dslconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProvidersDir_ConfigProviders(t *testing.T) {
	candidates := []string{
		filepath.Join("..", "..", "..", "config", "providers"),
		filepath.Join("..", "..", "config", "providers"),
	}
	for _, dir := range candidates {
		if _, err := ValidateProvidersDir(dir); err == nil {
			return
		}
	}
	t.Fatalf("validate providers dir failed for all candidates: %v", candidates)
}

func TestValidateProvidersDir_RejectsLegacyUsageDirectiveAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      input_tokens = $.usage.input_tokens;
      output_tokens = $.usage.output_tokens;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProvidersDir(dir); err == nil {
		t.Fatalf("expected legacy usage alias validation error")
	}
}

func TestValidateProvidersDir_RejectsLegacyBalanceDirectiveAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      balance_mode custom;
      path "/v1/credits";
      balance_expr = $.data.total;
      used = $.data.used;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProvidersDir(dir); err == nil {
		t.Fatalf("expected legacy balance alias validation error")
	}
}

func TestValidateProvidersDir_NoDeprecatedDirectiveWarnings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      input_tokens_expr = $.usage.input_tokens;
      output_tokens_expr = $.usage.output_tokens;
    }
    balance {
      balance_mode custom;
      path "/v1/credits";
      balance_expr = $.data.total;
      used_expr = $.data.used;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := ValidateProvidersDir(dir)
	if err != nil {
		t.Fatalf("ValidateProvidersDir: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %d: %#v", len(res.Warnings), res.Warnings)
	}
}
