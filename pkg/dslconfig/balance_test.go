package dslconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBalanceExprAndEval(t *testing.T) {
	expr, err := ParseBalanceExpr("$.data.total_credits - $.data.total_usage + 0.5")
	if err != nil {
		t.Fatalf("ParseBalanceExpr: %v", err)
	}
	root := map[string]any{
		"data": map[string]any{
			"total_credits": 12.5,
			"total_usage":   2.0,
		},
	}
	if got := expr.Eval(root); got != 11.0 {
		t.Fatalf("Eval got %.2f, want %.2f", got, 11.0)
	}
}

func TestExtractBalance_CustomJSON(t *testing.T) {
	cfg := BalanceQueryConfig{
		Mode:        balanceModeCustom,
		BalanceExpr: "$.data.total - $.data.used",
		UsedPath:    "$.data.used",
	}
	body := []byte(`{"data": {"total": 100.5, "used": 12.25}}`)
	balance, used, err := ExtractBalance(cfg, body)
	if err != nil {
		t.Fatalf("ExtractBalance: %v", err)
	}
	if balance != 88.25 {
		t.Fatalf("balance got %.4f, want %.4f", balance, 88.25)
	}
	if used == nil || *used != 12.25 {
		t.Fatalf("used got %#v, want %.2f", used, 12.25)
	}
}

func TestValidateProviderFile_BalanceBlock(t *testing.T) {
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
      method GET;
      path "/v1/credits";
      balance = $.data.total_credits - $.data.total_usage;
      used_path "$.data.total_usage";
      balance_unit "USD";
      set_header "Authorization" concat("Bearer ", $channel.key);
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
	if pf.Balance.Defaults.Mode != "custom" {
		t.Fatalf("balance mode got %q", pf.Balance.Defaults.Mode)
	}
	if pf.Balance.Defaults.Path != "/v1/credits" {
		t.Fatalf("balance path got %q", pf.Balance.Defaults.Path)
	}
	if len(pf.Balance.Defaults.Headers) != 1 {
		t.Fatalf("balance headers len got %d", len(pf.Balance.Defaults.Headers))
	}
}

func TestValidateProviderFile_BalanceUnitInvalid(t *testing.T) {
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
      method GET;
      path "/v1/credits";
      balance_path "$.data.balance";
      balance_unit "POINTS";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected ValidateProviderFile to fail for invalid balance_unit")
	}
}
