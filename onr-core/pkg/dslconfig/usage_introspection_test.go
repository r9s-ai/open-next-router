package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestUsageExtractConfigDeclaredFacts(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: "custom",
		facts: []usageFactConfig{
			{
				Dimension:     "server_tool.web_search",
				Unit:          "call",
				CountPath:     "$.output[*]",
				Type:          "web_search_call",
				Status:        "completed",
				Event:         "message_delta",
				EventOptional: true,
			},
			{
				Dimension: "audio.tts",
				Unit:      "second",
				Source:    "derived",
				Path:      "$.audio_duration_seconds",
			},
		},
	}

	facts := cfg.DeclaredFacts()
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
	if facts[0].Dimension != "server_tool.web_search" || facts[0].Unit != "call" || facts[0].CountPath != "$.output[*]" {
		t.Fatalf("unexpected fact[0]: %#v", facts[0])
	}
	if !facts[0].EventOptional || facts[0].Event != "message_delta" {
		t.Fatalf("unexpected event metadata in fact[0]: %#v", facts[0])
	}
	if facts[1].Dimension != "audio.tts" || facts[1].Unit != "second" || facts[1].Source != "derived" {
		t.Fatalf("unexpected fact[1]: %#v", facts[1])
	}
}

func TestUsageExtractConfigBuiltinFacts(t *testing.T) {
	cfg := UsageExtractConfig{Mode: "gemini_generate_content"}

	facts := cfg.BuiltinFacts()
	if len(facts) != 0 {
		t.Fatalf("expected builtin facts removed, got %d", len(facts))
	}
}

func TestUsageExtractConfigCompiledFacts_OpenAIAudioSpeech(t *testing.T) {
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", "audio.speech", false)

	facts := cfg.CompiledFacts(&dslmeta.Meta{API: "audio.speech", IsStream: false})
	found := false
	for _, fact := range facts {
		if fact.Dimension == "audio.tts" && fact.Unit == "second" && fact.Source == "derived" && fact.Path == "$.audio_duration_seconds" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected derived audio.tts fact, got %#v", facts)
	}
}

func TestUsageExtractConfigCompiledPlan_CustomLegacyFields(t *testing.T) {
	inExpr, err := ParseUsageExpr("$.usage.prompt_tokens + $.usage.extra_input")
	if err != nil {
		t.Fatalf("ParseUsageExpr input: %v", err)
	}
	totalExpr, err := ParseUsageExpr("$.usage.total_tokens - $.usage.cached_tokens")
	if err != nil {
		t.Fatalf("ParseUsageExpr total: %v", err)
	}
	cfg := UsageExtractConfig{
		Mode:             "custom",
		InputTokensExpr:  inExpr,
		OutputTokensPath: "$.usage.output_tokens",
		TotalTokensExpr:  totalExpr,
	}

	plan := cfg.CompiledPlan(nil)
	if plan.Mode != usageModeCustom {
		t.Fatalf("plan mode=%q want=%q", plan.Mode, usageModeCustom)
	}
	if got, want := len(plan.Facts), 2; got != want {
		t.Fatalf("compiled facts len=%d want=%d", got, want)
	}
	if plan.Facts[0].Expr != "$.usage.prompt_tokens + $.usage.extra_input" {
		t.Fatalf("unexpected fact[0] expr: %#v", plan.Facts[0])
	}
	if plan.Facts[1].Path != "$.usage.output_tokens" {
		t.Fatalf("unexpected fact[1] path: %#v", plan.Facts[1])
	}
	if plan.TotalTokensExpr != "$.usage.total_tokens - $.usage.cached_tokens" {
		t.Fatalf("total_tokens_expr=%q", plan.TotalTokensExpr)
	}
}

func TestUsageExtractConfigCompiledFacts_PreserveEventMetadata(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{
				Dimension:     "output",
				Unit:          "token",
				Path:          "$.usage.output_tokens",
				Event:         "message_delta",
				EventOptional: true,
			},
		},
	}

	facts := cfg.CompiledFacts(&dslmeta.Meta{API: "responses", IsStream: true})
	if len(facts) != 1 {
		t.Fatalf("expected 1 compiled fact, got %d", len(facts))
	}
	if !facts[0].EventOptional || facts[0].Event != "message_delta" {
		t.Fatalf("compiled fact lost event metadata: %#v", facts[0])
	}
	plan := cfg.CompiledPlan(&dslmeta.Meta{API: "responses", IsStream: true})
	if len(plan.Facts) != 1 || !plan.Facts[0].EventOptional {
		t.Fatalf("compiled plan lost event metadata: %#v", plan)
	}
}

func TestProviderUsageCompiledPlans_MergeDefaultsIntoMatch(t *testing.T) {
	streamFalse := false
	p := ProviderUsage{
		Defaults: UsageExtractConfig{
			Mode:                usageModeCustom,
			CacheReadTokensPath: "$.usage.cached_tokens_override",
		},
		Matches: []MatchUsage{
			{
				API:    "audio.speech",
				Stream: &streamFalse,
				Extract: UsageExtractConfig{
					facts: []usageFactConfig{
						{Dimension: "audio.tts", Unit: "second", Source: "derived", Path: "$.audio_duration_seconds"},
					},
				},
			},
		},
	}

	plans := p.CompiledPlans()
	if plans.Defaults.Mode != usageModeCustom {
		t.Fatalf("defaults mode=%q want=%q", plans.Defaults.Mode, usageModeCustom)
	}
	if got, want := len(plans.Matches), 1; got != want {
		t.Fatalf("matches len=%d want=%d", got, want)
	}
	matchPlan := plans.Matches[0].Plan
	if matchPlan.Mode != usageModeCustom {
		t.Fatalf("match mode=%q want=%q", matchPlan.Mode, usageModeCustom)
	}
	var foundCacheRead bool
	var foundDerivedAudio bool
	for _, fact := range matchPlan.Facts {
		if fact.Dimension == "cache_read" && fact.Path == "$.usage.cached_tokens_override" {
			foundCacheRead = true
		}
		if fact.Dimension == "audio.tts" && fact.Source == "derived" && fact.Path == "$.audio_duration_seconds" {
			foundDerivedAudio = true
		}
	}
	if !foundCacheRead {
		t.Fatalf("expected merged cache_read override in match plan: %#v", matchPlan.Facts)
	}
	if !foundDerivedAudio {
		t.Fatalf("expected derived audio fact in match plan: %#v", matchPlan.Facts)
	}
}
