package dslconfig

import (
	"strings"
	"testing"
)

func TestParseUsageModeResolvesNamedInheritance(t *testing.T) {
	t.Parallel()

	cfg, err := ParseUsageMode("usage_modes.conf", `
usage_mode "base" {
  usage_root path="$.usage";
  usage_fact input token path="$.input_tokens";
}
usage_mode "child" {
  usage_extract base;
  usage_fact output token path="$.output_tokens";
}
`, "child")
	if err != nil {
		t.Fatalf("ParseUsageMode: %v", err)
	}
	if cfg.Mode != usageModeCustom || cfg.SourceMode != "base" {
		t.Fatalf("unexpected resolved config: %#v", cfg)
	}
	plan := cfg.CompiledPlan(nil)
	if len(plan.UsageRoots) != 1 || plan.UsageRoots[0].Path != "$.usage" {
		t.Fatalf("unexpected roots: %#v", plan.UsageRoots)
	}
	if len(plan.Facts) != 2 {
		t.Fatalf("facts=%#v", plan.Facts)
	}
	if plan.Facts[0].Dimension != "input" || plan.Facts[1].Dimension != "output" {
		t.Fatalf("unexpected facts: %#v", plan.Facts)
	}
}

func TestParseUsageModeSelectsOnlyModeWhenNameOmitted(t *testing.T) {
	t.Parallel()

	cfg, err := ParseUsageMode("usage_modes.conf", `
usage_mode "only" {
  usage_fact input token path="$.input_tokens";
}
`, "")
	if err != nil {
		t.Fatalf("ParseUsageMode: %v", err)
	}
	if cfg.Mode != usageModeCustom || len(cfg.CompiledFacts(nil)) != 1 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestParseUsageModeRejectsAmbiguousAndInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := ParseUsageMode("usage_modes.conf", `
usage_mode "first" { usage_fact input token path="$.input_tokens"; }
usage_mode "second" { usage_fact output token path="$.output_tokens"; }
`, "")
	if err == nil || !strings.Contains(err.Error(), "usage_mode name is required") {
		t.Fatalf("error=%v", err)
	}

	_, err = ParseUsageMode("usage_modes.conf", `
usage_mode "broken" { usage_fact input token path="input_tokens"; }
`, "broken")
	if err == nil || !strings.Contains(err.Error(), "must start with $") {
		t.Fatalf("error=%v", err)
	}

	_, err = ParseUsageMode("usage_modes.conf", `
usage_mode "one" { usage_extract two; }
usage_mode "two" { usage_extract one; }
`, "one")
	if err == nil || !strings.Contains(err.Error(), "recursive usage_mode") {
		t.Fatalf("error=%v", err)
	}
}
