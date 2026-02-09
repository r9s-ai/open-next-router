package pricing

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestResolverComputeWithOverrides(t *testing.T) {
	dir := t.TempDir()
	pricePath := filepath.Join(dir, "price.yaml")
	overridesPath := filepath.Join(dir, "price_overrides.yaml")
	priceYAML := `
version: v1
unit: usd_per_1m_tokens
entries:
  - provider: openai
    model: gpt-4o-mini
    cost:
      input: 0.15
      output: 0.60
      cache_read: 0.08
`
	overridesYAML := `
version: v1
providers:
  openai:
    multiplier: 2
    models:
      gpt-4o-mini:
        cost:
          output: 0.7
channels:
  "openai/key1":
    multiplier: 1.5
    models:
      gpt-4o-mini:
        cost:
          input: 0.2
`
	if err := os.WriteFile(pricePath, []byte(priceYAML), 0o600); err != nil {
		t.Fatalf("write price: %v", err)
	}
	if err := os.WriteFile(overridesPath, []byte(overridesYAML), 0o600); err != nil {
		t.Fatalf("write overrides: %v", err)
	}

	r, err := LoadResolver(pricePath, overridesPath)
	if err != nil {
		t.Fatalf("LoadResolver: %v", err)
	}
	if r == nil {
		t.Fatalf("resolver is nil")
	}
	c, ok := r.Compute("openai", "key1", "gpt-4o-mini", map[string]any{
		"input_tokens":       1000,
		"output_tokens":      500,
		"cache_read_tokens":  100,
		"cache_write_tokens": 50,
	})
	if !ok {
		t.Fatalf("Compute failed")
	}
	// input override 0.2, output override 0.7, then multiplier 2 * 1.5 = 3
	if math.Abs(c.InputRate-0.6) > 1e-9 {
		t.Fatalf("input rate=%v want=0.6", c.InputRate)
	}
	if math.Abs(c.OutputRate-2.1) > 1e-9 {
		t.Fatalf("output rate=%v want=2.1", c.OutputRate)
	}
	if c.BillableInputTokens != 850 {
		t.Fatalf("billable_input=%d want=850", c.BillableInputTokens)
	}
	if c.TotalCost <= 0 {
		t.Fatalf("total cost=%v want > 0", c.TotalCost)
	}
}

func TestLoadResolverMissingPriceFile(t *testing.T) {
	r, err := LoadResolver(filepath.Join(t.TempDir(), "missing.yaml"), "")
	if err != nil {
		t.Fatalf("LoadResolver err=%v", err)
	}
	if r != nil {
		t.Fatalf("resolver should be nil")
	}
}
