package dslspec

import (
	"strings"
	"testing"
)

func TestModesByDirective(t *testing.T) {
	got := ModesByDirective("req_map")
	if len(got) == 0 {
		t.Fatalf("expected req_map modes, got none")
	}
}

func TestDirectivesByBlock(t *testing.T) {
	got := DirectivesByBlock("auth")
	if len(got) == 0 {
		t.Fatalf("expected auth directives, got none")
	}
	found := false
	for _, d := range got {
		if d == "oauth_mode" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected oauth_mode in auth block directives")
	}

	metrics := DirectivesByBlock("metrics")
	found = false
	for _, d := range metrics {
		if d == "usage_fact" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected usage_fact in metrics block directives")
	}
}

func TestDirectiveHover(t *testing.T) {
	hover, ok := DirectiveHover("response")
	if !ok || hover == "" {
		t.Fatalf("expected hover for response")
	}
}

func TestDirectiveHoverForModelsMode(t *testing.T) {
	hover, ok := DirectiveHover("models_mode")
	if !ok || hover == "" {
		t.Fatalf("expected hover for models_mode")
	}
}

func TestDirectiveHoverForExprSuffixDirectives(t *testing.T) {
	for _, name := range []string{"input_tokens_expr", "used_expr"} {
		hover, ok := DirectiveHover(name)
		if !ok || hover == "" {
			t.Fatalf("expected hover for %s", name)
		}
	}
}

func TestDirectiveHoverForUsageFact(t *testing.T) {
	hover, ok := DirectiveHover("usage_fact")
	if !ok || hover == "" {
		t.Fatalf("expected hover for usage_fact")
	}
}

func TestDirectiveHoverInBlock_PrefersExactBlock(t *testing.T) {
	hover, ok := DirectiveHoverInBlock("set_header", "balance")
	if !ok || hover == "" {
		t.Fatalf("expected hover for set_header in balance block")
	}
	if !contains(hover, "balance query request") {
		t.Fatalf("expected balance-specific hover, got: %q", hover)
	}
}

func TestDirectiveMetadataList_ReturnsCopy(t *testing.T) {
	meta := DirectiveMetadataList()
	if len(meta) == 0 {
		t.Fatalf("expected metadata entries")
	}
	meta[0].Name = "mutated"
	meta[0].Modes = []string{"x"}
	meta[0].Args = []DirectiveArg{{Name: "x", Kind: "enum", Enum: []string{"A"}}}

	meta2 := DirectiveMetadataList()
	if len(meta2) == 0 {
		t.Fatalf("expected metadata entries in second read")
	}
	if meta2[0].Name == "mutated" {
		t.Fatalf("metadata list should return independent copy")
	}
}

func TestDirectiveArgEnumValuesInBlock(t *testing.T) {
	got := DirectiveArgEnumValuesInBlock("balance_unit", "balance", 0)
	if len(got) == 0 {
		t.Fatalf("expected enum values for balance_unit")
	}
	foundUSD := false
	for _, v := range got {
		if v == "USD" {
			foundUSD = true
			break
		}
	}
	if !foundUSD {
		t.Fatalf("expected USD in balance_unit enum values, got: %v", got)
	}

	got = DirectiveArgEnumValuesInBlock("method", "models", 0)
	if len(got) == 0 {
		t.Fatalf("expected enum values for method in models")
	}
}

func TestMetadata_ModeOptionsConsistency(t *testing.T) {
	assertSetEqual(t, "oauth_mode", ModesByDirective("oauth_mode"), []string{"openai", "gemini", "qwen", "claude", "iflow", "antigravity", "kimi", "custom"})
	assertSetEqual(t, "balance_mode", ModesByDirective("balance_mode"), []string{"openai", "custom"})
	assertSetEqual(t, "models_mode", ModesByDirective("models_mode"), []string{"openai", "gemini", "custom"})
	assertSetEqual(t, "usage_extract", ModesByDirective("usage_extract"), []string{"openai", "anthropic", "gemini", "custom"})
	assertSetEqual(t, "finish_reason_extract", ModesByDirective("finish_reason_extract"), []string{"openai", "anthropic", "gemini", "custom"})
	assertSetEqual(t, "error_map", ModesByDirective("error_map"), []string{"openai", "common", "passthrough"})
}

func TestMetadata_EnumArgOptionsConsistency(t *testing.T) {
	assertSetEqual(t, "oauth_method.auth", DirectiveArgEnumValuesInBlock("oauth_method", "auth", 0), []string{"GET", "POST"})
	assertSetEqual(t, "oauth_content_type.auth", DirectiveArgEnumValuesInBlock("oauth_content_type", "auth", 0), []string{"form", "json"})
	assertSetEqual(t, "method.balance", DirectiveArgEnumValuesInBlock("method", "balance", 0), []string{"GET", "POST"})
	assertSetEqual(t, "method.models", DirectiveArgEnumValuesInBlock("method", "models", 0), []string{"GET", "POST"})
	assertSetEqual(t, "balance_unit.balance", DirectiveArgEnumValuesInBlock("balance_unit", "balance", 0), []string{"USD", "CNY"})
}

func TestModeDirectiveNames(t *testing.T) {
	got := ModeDirectiveNames()
	if len(got) == 0 {
		t.Fatalf("expected non-empty mode directive names")
	}
	gotSet := make(map[string]struct{}, len(got))
	for _, name := range got {
		gotSet[name] = struct{}{}
	}
	for _, must := range []string{"req_map", "resp_map", "sse_parse", "oauth_mode", "balance_mode", "models_mode"} {
		if _, ok := gotSet[must]; !ok {
			t.Fatalf("expected mode directive %q in %v", must, got)
		}
	}
}

func TestDirectiveAllowedBlocks(t *testing.T) {
	assertSetEqual(t, "set_header", DirectiveAllowedBlocks("set_header"), []string{"request", "balance", "models"})
	assertSetEqual(t, "req_map", DirectiveAllowedBlocks("req_map"), []string{"request"})
	assertSetEqual(t, "provider", DirectiveAllowedBlocks("provider"), []string{"top"})
}

func TestBlockDirectiveNamesAndIsBlockDirective(t *testing.T) {
	blocks := BlockDirectiveNames()
	if len(blocks) == 0 {
		t.Fatalf("expected non-empty block directive names")
	}
	if IsBlockDirective("request") != true {
		t.Fatalf("request should be a block directive")
	}
	if IsBlockDirective("req_map") != false {
		t.Fatalf("req_map should not be a block directive")
	}
	for _, must := range []string{"provider", "defaults", "match", "request", "response"} {
		found := false
		for _, b := range blocks {
			if b == must {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected block directive %q in %v", must, blocks)
		}
	}
}

func assertSetEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	gotSet := make(map[string]struct{}, len(got))
	for _, v := range got {
		gotSet[strings.TrimSpace(v)] = struct{}{}
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, v := range want {
		wantSet[strings.TrimSpace(v)] = struct{}{}
	}
	if len(gotSet) != len(wantSet) {
		t.Fatalf("%s size mismatch: got=%v want=%v", name, got, want)
	}
	for v := range wantSet {
		if _, ok := gotSet[v]; !ok {
			t.Fatalf("%s missing value %q, got=%v want=%v", name, v, got, want)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
