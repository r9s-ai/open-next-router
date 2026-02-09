package dslconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUsageExprAndEval(t *testing.T) {
	expr, err := ParseUsageExpr("$.usage.input_tokens + $.usage.output_tokens - 2")
	if err != nil {
		t.Fatalf("ParseUsageExpr: %v", err)
	}
	root := map[string]any{
		"usage": map[string]any{
			"input_tokens":  float64(11),
			"output_tokens": float64(9),
		},
	}
	if got := expr.Eval(root); got != 18 {
		t.Fatalf("Eval got %d, want %d", got, 18)
	}
}

func TestParseUsageExprInvalid(t *testing.T) {
	if _, err := ParseUsageExpr("$.usage.x * 2"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateProviderFile_UsageAssign(t *testing.T) {
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
      input_tokens = $.usage.prompt_tokens + $.usage.input_tokens;
      output_tokens = $.usage.output_tokens;
      total_tokens = $.usage.total_tokens - $.usage.cached_tokens;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if pf.Usage.Defaults.InputTokensExpr == nil {
		t.Fatalf("expected InputTokensExpr to be set")
	}
	if pf.Usage.Defaults.TotalTokensExpr == nil {
		t.Fatalf("expected TotalTokensExpr to be set")
	}
}
