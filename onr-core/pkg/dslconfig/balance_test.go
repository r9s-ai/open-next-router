package dslconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
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
	balance, used, err := ExtractBalance(&cfg, body)
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
      balance_expr = $.data.total_credits - $.data.total_usage;
      used_expr = $.data.total_usage;
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
	if pf.Balance.Defaults.UsedExpr == "" {
		t.Fatalf("expected used_expr to be set")
	}
	if len(pf.Balance.Defaults.Headers) != 1 {
		t.Fatalf("balance headers len got %d", len(pf.Balance.Defaults.Headers))
	}
}

func TestValidateProviderFile_BalanceBlock_ImplicitCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      path "/v1/credits";
      balance_path "$.data.balance";
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
	cfg, ok := pf.Balance.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if got, want := cfg.Mode, balanceModeCustom; got != want {
		t.Fatalf("balance mode=%q want=%q", got, want)
	}
	if got, want := cfg.Method, "GET"; got != want {
		t.Fatalf("method=%q want=%q", got, want)
	}
}

func TestValidateProviderFile_RejectsLegacyUsedAlias(t *testing.T) {
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
      balance_expr = $.data.total_credits - $.data.total_usage;
      used = $.data.total_usage;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected legacy used alias validation error")
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
	} else {
		var issue *ValidationIssue
		if !errors.As(err, &issue) {
			t.Fatalf("expected ValidationIssue, got: %T", err)
		}
		if issue.Directive != "balance_unit" {
			t.Fatalf("unexpected directive: %q", issue.Directive)
		}
	}
}

func TestValidateProviderFile_LoadsSiblingOnrConfigBalanceMode(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

balance_mode "shared_openai_balance" {
  balance_mode openai;
  usage_path "/v9/dashboard/billing/usage";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	path := filepath.Join(providersDir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      balance_mode shared_openai_balance;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Balance.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if got, want := cfg.Mode, balanceModeOpenAI; got != want {
		t.Fatalf("balance mode=%q want=%q", got, want)
	}
	if got, want := cfg.UsagePath, "/v9/dashboard/billing/usage"; got != want {
		t.Fatalf("usage_path=%q want=%q", got, want)
	}
}

func TestValidateProviderFile_LoadsSiblingOnrConfigBalanceModeImplicitCustom(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

balance_mode "shared_custom_balance" {
  path "/v9/credits";
  balance_path "$.data.balance";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	path := filepath.Join(providersDir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      balance_mode shared_custom_balance;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Balance.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if got, want := cfg.Mode, balanceModeCustom; got != want {
		t.Fatalf("balance mode=%q want=%q", got, want)
	}
	if got, want := cfg.Path, "/v9/credits"; got != want {
		t.Fatalf("path=%q want=%q", got, want)
	}
}

func TestValidateProviderFile_UserBalanceModeOverridesBuiltinName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

balance_mode "openai" {
  balance_mode custom;
  path "/v1/credits";
  balance_path "$.data.balance";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      balance_mode openai;
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
	cfg, ok := pf.Balance.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if got, want := cfg.Mode, balanceModeCustom; got != want {
		t.Fatalf("balance mode=%q want=%q", got, want)
	}
	if got, want := cfg.Path, "/v1/credits"; got != want {
		t.Fatalf("path=%q want=%q", got, want)
	}
	if got, want := cfg.BalancePath, "$.data.balance"; got != want {
		t.Fatalf("balance_path=%q want=%q", got, want)
	}
}

func TestValidateProviderFile_ImplicitBuiltinBalanceModeByName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

balance_mode "openai" {
  usage_path "/v2/dashboard/billing/usage";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      balance_mode openai;
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
	cfg, ok := pf.Balance.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if got, want := cfg.Mode, balanceModeOpenAI; got != want {
		t.Fatalf("balance mode=%q want=%q", got, want)
	}
	if got, want := cfg.UsagePath, "/v2/dashboard/billing/usage"; got != want {
		t.Fatalf("usage_path=%q want=%q", got, want)
	}
}

func TestValidateProviderFile_BalanceModeRequiresGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    balance {
      balance_mode openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected missing global balance_mode to fail validation")
	}
}
