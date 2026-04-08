package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestProviderUsageSelect_MergeMatch(t *testing.T) {
	streamTrue := true
	p := ProviderUsage{
		Defaults: UsageExtractConfig{
			Mode:             usageModeCustom,
			InputTokensPath:  "$.usage.input",
			OutputTokensPath: "$.usage.output",
		},
		Matches: []MatchUsage{
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Extract: UsageExtractConfig{
					Mode:             usageModeCustom,
					OutputTokensPath: "$.x.out",
				},
			},
		},
	}

	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: true})
	if !ok {
		t.Fatalf("expected usage config selected")
	}
	if cfg.Mode != usageModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, usageModeCustom)
	}
	if cfg.InputTokensPath != "$.usage.input" {
		t.Fatalf("input path not merged from defaults: %q", cfg.InputTokensPath)
	}
	if cfg.OutputTokensPath != "$.x.out" {
		t.Fatalf("output path not overridden by match: %q", cfg.OutputTokensPath)
	}
}

func TestProviderUsageSelect_MergeFactsDoesNotMutateDefaults(t *testing.T) {
	streamTrue := true
	p := ProviderUsage{
		Defaults: UsageExtractConfig{
			Mode:  usageModeCustom,
			facts: []usageFactConfig{{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens"}},
		},
		Matches: []MatchUsage{
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Extract: UsageExtractConfig{
					facts: []usageFactConfig{{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"}},
				},
			},
		},
	}

	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: true})
	if !ok {
		t.Fatalf("expected usage config selected")
	}
	if got, want := len(p.Defaults.facts), 1; got != want {
		t.Fatalf("defaults facts len got %d, want %d", got, want)
	}
	if got, want := len(cfg.facts), 2; got != want {
		t.Fatalf("merged facts len got %d, want %d", got, want)
	}

	resp := []byte(`{"usage":{"input_tokens":3,"output_tokens":4}}`)
	usage, _, err := ExtractUsage(&dslmeta.Meta{API: "chat.completions", IsStream: true}, cfg, resp)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 3; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 4; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
}

func TestProviderFinishReasonSelect_MergeAndEmpty(t *testing.T) {
	p := ProviderFinishReason{
		Defaults: FinishReasonExtractConfig{Mode: "custom", FinishReasonPath: "$.a"},
		Matches: []MatchFinishReason{
			{
				API:    "chat.completions",
				Stream: nil,
				Extract: FinishReasonExtractConfig{
					Mode: "custom",
				},
			},
		},
	}
	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected finish_reason config selected")
	}
	if cfg.Mode != "custom" {
		t.Fatalf("mode=%q want=custom", cfg.Mode)
	}
	if cfg.FinishReasonPath != "$.a" {
		t.Fatalf("path should keep default when not overridden, got %q", cfg.FinishReasonPath)
	}

	if _, ok := (ProviderFinishReason{}).Select(&dslmeta.Meta{API: "chat.completions"}); ok {
		t.Fatalf("expected empty config not selected")
	}
}

func TestProviderBalanceSelect_ImplicitCustom(t *testing.T) {
	p := ProviderBalance{
		Defaults: BalanceQueryConfig{
			Path:        "/v1/credits",
			BalancePath: "$.data.balance",
		},
	}

	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if cfg.Mode != balanceModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, balanceModeCustom)
	}
	if cfg.Method != "GET" {
		t.Fatalf("method=%q want=GET", cfg.Method)
	}
}

func TestProviderModelsSelect_ImplicitCustom(t *testing.T) {
	p := ProviderModels{
		Defaults: ModelsQueryConfig{
			Path:    "/v1/models",
			IDPaths: []string{"$.items[*].name"},
		},
	}

	cfg, ok := p.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if cfg.Mode != modelsModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, modelsModeCustom)
	}
	if cfg.Method != "GET" {
		t.Fatalf("method=%q want=GET", cfg.Method)
	}
}

func TestProviderFinishReasonSelect_PathRulesOverrideDefaults(t *testing.T) {
	streamTrue := true
	defaults := FinishReasonExtractConfig{Mode: "custom"}
	defaults.addFinishReasonPath("$.defaults.stop_reason", false)

	override := FinishReasonExtractConfig{}
	override.addFinishReasonPath("$.delta.stop_reason", false)
	override.addFinishReasonPath("$.message.stop_reason", true)

	p := ProviderFinishReason{
		Defaults: defaults,
		Matches: []MatchFinishReason{
			{
				API:     "claude.messages",
				Stream:  &streamTrue,
				Extract: override,
			},
		},
	}

	cfg, ok := p.Select(&dslmeta.Meta{API: "claude.messages", IsStream: true})
	if !ok {
		t.Fatalf("expected finish_reason config selected")
	}
	if got, want := len(cfg.finishReasonPathConfigs()), 2; got != want {
		t.Fatalf("path rules len=%d want=%d", got, want)
	}
	if got := cfg.finishReasonPathConfigs()[0]; got.Path != "$.delta.stop_reason" || got.Fallback {
		t.Fatalf("unexpected primary path rule: %+v", got)
	}
	if got := cfg.finishReasonPathConfigs()[1]; got.Path != "$.message.stop_reason" || !got.Fallback {
		t.Fatalf("unexpected fallback path rule: %+v", got)
	}
}

func TestProviderResponseSelect_MergeDirective(t *testing.T) {
	streamFalse := false
	p := ProviderResponse{
		Defaults: ResponseDirective{
			Op:   "resp_map",
			Mode: "openai_responses_to_openai_chat",
			JSONOps: []JSONOp{
				{Op: "json_set", Path: "$.a", ValueExpr: "\"1\""},
			},
		},
		Matches: []MatchResponse{
			{
				API:    "chat.completions",
				Stream: &streamFalse,
				Response: ResponseDirective{
					JSONOps: []JSONOp{
						{Op: "json_del", Path: "$.b"},
					},
					SSEJSONDelIf: []SSEJSONDelIfRule{
						{CondPath: "$.type", Equals: "x", DelPath: "$.c"},
					},
				},
			},
		},
	}
	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: false})
	if !ok {
		t.Fatalf("expected response directive selected")
	}
	if cfg.Op != "resp_map" {
		t.Fatalf("unexpected op: %q", cfg.Op)
	}
	if len(cfg.JSONOps) != 2 {
		t.Fatalf("expected merged json ops len=2, got %d", len(cfg.JSONOps))
	}
	if len(cfg.SSEJSONDelIf) != 1 {
		t.Fatalf("expected merged sse rules len=1, got %d", len(cfg.SSEJSONDelIf))
	}
}

func TestProviderBalanceSelect_MergeMatch(t *testing.T) {
	streamTrue := true
	p := ProviderBalance{
		Defaults: BalanceQueryConfig{
			Mode:        balanceModeOpenAI,
			Method:      "GET",
			BalancePath: "$.default.balance",
		},
		Matches: []MatchBalance{
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Query: BalanceQueryConfig{
					Mode: balanceModeCustom,
					Path: "/v1/billing",
				},
			},
		},
	}
	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: true})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if cfg.Mode != balanceModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, balanceModeCustom)
	}
	if cfg.BalancePath != "$.default.balance" {
		t.Fatalf("balance path should keep default, got %q", cfg.BalancePath)
	}
	if cfg.Path != "/v1/billing" {
		t.Fatalf("path not overridden by match, got %q", cfg.Path)
	}
}

func TestProviderRoutingHasMatchHelpers(t *testing.T) {
	streamTrue := true
	p := ProviderRouting{
		Matches: []RoutingMatch{
			{API: "chat.completions", Stream: &streamTrue},
		},
	}
	if !p.HasMatchAPI("chat.completions") {
		t.Fatalf("HasMatchAPI should be true")
	}
	if p.HasMatchAPI("embeddings") {
		t.Fatalf("HasMatchAPI should be false")
	}
	if !p.HasMatch(&dslmeta.Meta{API: "chat.completions", IsStream: true}) {
		t.Fatalf("HasMatch should be true for stream=true")
	}
	if p.HasMatch(&dslmeta.Meta{API: "chat.completions", IsStream: false}) {
		t.Fatalf("HasMatch should be false for stream mismatch")
	}
	if p.HasMatch(nil) {
		t.Fatalf("HasMatch should be false for nil meta")
	}
}
