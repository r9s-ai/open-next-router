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
	assertSetContains(t, "auth directives", DirectivesByBlock("auth"), []string{"oauth_mode"})
	assertSetExcludes(t, "auth directives", DirectivesByBlock("auth"), []string{"pass_header"})
	assertSetContains(t, "metrics directives", DirectivesByBlock("metrics"), []string{"usage_fact", "usage_root"})
	assertSetExcludes(t, "upstream directives", DirectivesByBlock("upstream"), []string{"filter_header_values"})
	assertSetContains(t, "request directives", DirectivesByBlock("request"), []string{"pass_header", "filter_header_values", "json_wrap_input_text", "after_req_map"})
	assertSetContains(t, "after_req_map directives", DirectivesByBlock("after_req_map"), []string{"json_set"})
	assertSetContains(t, "usage_mode directives", DirectivesByBlock("usage_mode"), []string{"usage_fact", "usage_root"})
}

func TestDirectiveHover(t *testing.T) {
	hover, ok := DirectiveHover("response")
	if !ok || hover == "" {
		t.Fatalf("expected hover for response")
	}

	hover, ok = DirectiveHover("include")
	if !ok || hover == "" {
		t.Fatalf("expected hover for include")
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

func TestDirectiveHoverForUsageRoot(t *testing.T) {
	hover, ok := DirectiveHover("usage_root")
	if !ok || hover == "" {
		t.Fatalf("expected hover for usage_root")
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
	assertSetEqual(t, "usage_extract", ModesByDirective("usage_extract"), []string{"custom"})
	assertSetEqual(t, "finish_reason_extract", ModesByDirective("finish_reason_extract"), []string{"custom"})
	assertSetEqual(t, "error_map", ModesByDirective("error_map"), []string{"openai", "common", "passthrough"})
}

func TestMetadata_ContextualModeOptions(t *testing.T) {
	assertSetEqual(t, "models_mode.models", ModesByDirectiveInBlock("models_mode", "models"), []string{"openai", "gemini", "custom"})
	assertSetEqual(t, "models_mode.top", ModesByDirectiveInBlock("models_mode", "top"), nil)
	assertSetEqual(t, "balance_mode.balance", ModesByDirectiveInBlock("balance_mode", "balance"), []string{"openai", "custom"})
	assertSetEqual(t, "balance_mode.top", ModesByDirectiveInBlock("balance_mode", "top"), nil)
	assertSetEqual(t, "req_map.request", ModesByDirectiveInBlock("req_map", "request"), []string{"openai_chat_to_openai_responses", "openai_chat_to_anthropic_messages", "openai_chat_to_gemini_generate_content", "anthropic_to_openai_chat", "gemini_to_openai_chat"})
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

func TestModeDirectiveNamesInBlock(t *testing.T) {
	assertSetEqual(t, "mode directives top", ModeDirectiveNamesInBlock("top"), nil)
	assertSetEqual(t, "mode directives models", ModeDirectiveNamesInBlock("models"), []string{"models_mode"})
	assertSetEqual(t, "mode directives balance", ModeDirectiveNamesInBlock("balance"), []string{"balance_mode"})
	assertSetEqual(t, "mode directives request", ModeDirectiveNamesInBlock("request"), []string{"req_map"})
}

func TestDirectiveModeRegistryBlock(t *testing.T) {
	if got := DirectiveModeRegistryBlock("usage_extract", "metrics"); got != "usage_mode" {
		t.Fatalf("expected usage_extract.metrics registry block usage_mode, got %q", got)
	}
	if got := DirectiveModeRegistryBlock("finish_reason_extract", "metrics"); got != "finish_reason_mode" {
		t.Fatalf("expected finish_reason_extract.metrics registry block finish_reason_mode, got %q", got)
	}
	if got := DirectiveModeRegistryBlock("models_mode", "models"); got != "models_mode" {
		t.Fatalf("expected models.models_mode registry block models_mode, got %q", got)
	}
	if got := DirectiveModeRegistryBlock("balance_mode", "balance_mode"); got != "balance_mode" {
		t.Fatalf("expected balance_mode.balance_mode registry block balance_mode, got %q", got)
	}
	if got := DirectiveModeRegistryBlock("req_map", "request"); got != "" {
		t.Fatalf("expected req_map.request to have no registry block, got %q", got)
	}
}

func TestDirectiveModeRegistryBlockInBlock(t *testing.T) {
	if got := DirectiveModeRegistryBlockInBlock("models_mode", "models"); got != "models_mode" {
		t.Fatalf("expected models.models_mode registry block models_mode, got %q", got)
	}
	if got := DirectiveModeRegistryBlockInBlock("models_mode", "top"); got != "" {
		t.Fatalf("expected top.models_mode to have no registry block, got %q", got)
	}
	if !DirectiveHasDynamicModeRegistryInBlock("models_mode", "models") {
		t.Fatalf("expected models.models_mode to support dynamic mode registry")
	}
	if DirectiveHasDynamicModeRegistryInBlock("models_mode", "top") {
		t.Fatalf("did not expect top.models_mode to support dynamic mode registry")
	}
}

func TestDirectiveHasDynamicModeRegistry(t *testing.T) {
	for _, name := range []string{"usage_extract", "finish_reason_extract", "models_mode", "balance_mode"} {
		if !DirectiveHasDynamicModeRegistry(name) {
			t.Fatalf("expected %s to support dynamic mode registry", name)
		}
	}
	for _, name := range []string{"req_map", "resp_map", "sse_parse", "oauth_mode"} {
		if DirectiveHasDynamicModeRegistry(name) {
			t.Fatalf("did not expect %s to support dynamic mode registry", name)
		}
	}
}

func TestDirectiveAllowedBlocks(t *testing.T) {
	assertSetEqual(t, "set_header", DirectiveAllowedBlocks("set_header"), []string{"request", "balance", "models", "balance_mode", "models_mode"})
	assertSetEqual(t, "pass_header", DirectiveAllowedBlocks("pass_header"), []string{"request"})
	assertSetEqual(t, "req_map", DirectiveAllowedBlocks("req_map"), []string{"request"})
	assertSetEqual(t, "after_req_map", DirectiveAllowedBlocks("after_req_map"), []string{"request"})
	assertSetEqual(t, "filter_header_values", DirectiveAllowedBlocks("filter_header_values"), []string{"request"})
	assertSetEqual(t, "include", DirectiveAllowedBlocks("include"), []string{"top"})
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
	if IsBlockDirective("after_req_map") != true {
		t.Fatalf("after_req_map should be a block directive")
	}
	if IsBlockDirective("req_map") != false {
		t.Fatalf("req_map should not be a block directive")
	}
	for _, must := range []string{"provider", "defaults", "match", "request", "after_req_map", "response", "usage_mode", "finish_reason_mode", "models_mode", "balance_mode"} {
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

func TestDirectiveBlockShapeIsContextSensitive(t *testing.T) {
	if !DirectiveIsBlockInBlock("provider", "top") {
		t.Fatalf("provider should be a top-level block directive")
	}
	if !DirectiveBlockHasHeaderInBlock("provider", "top") {
		t.Fatalf("provider block should consume a header before '{'")
	}
	if !DirectiveIsBlockInBlock("match", "provider") {
		t.Fatalf("match should be a provider block directive")
	}
	if !DirectiveBlockHasHeaderInBlock("match", "provider") {
		t.Fatalf("match block should consume a header before '{'")
	}
	if !DirectiveIsBlockInBlock("after_req_map", "request") {
		t.Fatalf("after_req_map should be a request block directive")
	}
	if DirectiveBlockHasHeaderInBlock("after_req_map", "request") {
		t.Fatalf("after_req_map should not consume a header before '{'")
	}
	if DirectiveIsBlockInBlock("req_map", "request") {
		t.Fatalf("req_map should be a request statement directive")
	}

	if !DirectiveIsBlockInBlock("models_mode", "top") {
		t.Fatalf("top-level models_mode should be a block directive")
	}
	if !DirectiveBlockHasHeaderInBlock("models_mode", "top") {
		t.Fatalf("top-level models_mode should consume a header before '{'")
	}
	if DirectiveIsBlockInBlock("models_mode", "models") {
		t.Fatalf("models.models_mode should be a statement directive")
	}
	if DirectiveBlockHasHeaderInBlock("models_mode", "models") {
		t.Fatalf("models.models_mode should not consume a block header")
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

func assertSetContains(t *testing.T, name string, got, want []string) {
	t.Helper()
	gotSet := make(map[string]struct{}, len(got))
	for _, v := range got {
		gotSet[strings.TrimSpace(v)] = struct{}{}
	}
	for _, v := range want {
		key := strings.TrimSpace(v)
		if _, ok := gotSet[key]; !ok {
			t.Fatalf("%s missing value %q, got=%v", name, key, got)
		}
	}
}

func assertSetExcludes(t *testing.T, name string, got, excluded []string) {
	t.Helper()
	gotSet := make(map[string]struct{}, len(got))
	for _, v := range got {
		gotSet[strings.TrimSpace(v)] = struct{}{}
	}
	for _, v := range excluded {
		key := strings.TrimSpace(v)
		if _, ok := gotSet[key]; ok {
			t.Fatalf("%s should not contain value %q, got=%v", name, key, got)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
