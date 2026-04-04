package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func BenchmarkExtractUsage_OpenAI_Exported(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	respBody, _ := benchmarkOpenAIUsagePayload()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u, cached, err := ExtractUsage(meta, cfg, respBody)
		if err != nil || u == nil || cached != 11 {
			b.Fatalf("unexpected result usage=%+v cached=%d err=%v", u, cached, err)
		}
	}
}

func BenchmarkExtractUsage_OpenAI_FromRoot(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	respBody, root := benchmarkOpenAIUsagePayload()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u, cached, err := extractUsageFromResponseRoot(meta, cfg, root, respBody)
		if err != nil || u == nil || cached != 11 {
			b.Fatalf("unexpected result usage=%+v cached=%d err=%v", u, cached, err)
		}
	}
}

func BenchmarkExtractUsage_CustomFacts_Exported(b *testing.B) {
	meta, cfg, respBody, _ := benchmarkCustomFactsUsageCase()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u, cached, err := ExtractUsage(meta, cfg, respBody)
		if err != nil || u == nil || cached != 23 {
			b.Fatalf("unexpected result usage=%+v cached=%d err=%v", u, cached, err)
		}
	}
}

func BenchmarkExtractUsage_CustomFacts_FromRoot(b *testing.B) {
	meta, cfg, respBody, root := benchmarkCustomFactsUsageCase()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		u, cached, err := extractUsageFromResponseRoot(meta, cfg, root, respBody)
		if err != nil || u == nil || cached != 23 {
			b.Fatalf("unexpected result usage=%+v cached=%d err=%v", u, cached, err)
		}
	}
}

func BenchmarkExtractFinishReason_OpenAI_Exported(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	_, cfg := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	body := []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := ExtractFinishReason(meta, cfg, body)
		if err != nil || v != "stop" {
			b.Fatalf("unexpected result finish_reason=%q err=%v", v, err)
		}
	}
}

func BenchmarkExtractFinishReason_OpenAI_FromRoot(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	_, cfg := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	root := map[string]any{
		"choices": []any{
			map[string]any{"index": float64(0), "finish_reason": "stop"},
		},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v, err := extractFinishReasonFromRoot(meta, cfg, root)
		if err != nil || v != "stop" {
			b.Fatalf("unexpected result finish_reason=%q err=%v", v, err)
		}
	}
}

func BenchmarkProviderUsageSelect_Match(b *testing.B) {
	streamFalse := false
	streamTrue := true
	provider := ProviderUsage{
		Defaults: UsageExtractConfig{
			Mode: usageModeCustom,
			facts: []usageFactConfig{
				{Dimension: "input", Unit: "token", Source: "response", Path: "$.usage.input_tokens"},
				{Dimension: "output", Unit: "token", Source: "response", Path: "$.usage.output_tokens"},
			},
		},
		Matches: []MatchUsage{
			{API: "embeddings", Stream: &streamFalse, Extract: UsageExtractConfig{Mode: usageModeCustom}},
			{API: "responses", Stream: &streamTrue, Extract: UsageExtractConfig{Mode: usageModeCustom}},
			{API: "chat.completions", Stream: &streamTrue, Extract: UsageExtractConfig{
				facts: []usageFactConfig{
					{Dimension: "cache_read", Unit: "token", Source: "response", Path: "$.usage.cached_tokens"},
				},
			}},
		},
	}
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cfg, ok := provider.Select(meta)
		if !ok || cfg.Mode != usageModeCustom || len(cfg.facts) != 3 {
			b.Fatalf("unexpected cfg=%+v ok=%v", cfg, ok)
		}
	}
}

func BenchmarkProviderFinishReasonSelect_Match(b *testing.B) {
	streamTrue := true
	provider := ProviderFinishReason{
		Defaults: FinishReasonExtractConfig{Mode: "custom", FinishReasonPath: "$.choices[0].finish_reason"},
		Matches: []MatchFinishReason{
			{API: "embeddings", Extract: FinishReasonExtractConfig{Mode: "custom", FinishReasonPath: "$.x"}},
			{API: "chat.completions", Stream: &streamTrue, Extract: FinishReasonExtractConfig{Mode: "custom", FinishReasonPath: "$.choices[0].finish_reason"}},
		},
	}
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cfg, ok := provider.Select(meta)
		if !ok || cfg.Mode != "custom" || cfg.FinishReasonPath == "" {
			b.Fatalf("unexpected cfg=%+v ok=%v", cfg, ok)
		}
	}
}

func benchmarkOpenAIUsagePayload() ([]byte, map[string]any) {
	root := map[string]any{
		"usage": map[string]any{
			"prompt_tokens":     float64(123),
			"completion_tokens": float64(45),
			"total_tokens":      float64(168),
			"prompt_tokens_details": map[string]any{
				"cached_tokens": float64(11),
			},
		},
	}
	body := []byte(`{"usage":{"prompt_tokens":123,"completion_tokens":45,"total_tokens":168,"prompt_tokens_details":{"cached_tokens":11}}}`)
	return body, root
}

func benchmarkCustomFactsUsageCase() (*dslmeta.Meta, UsageExtractConfig, []byte, map[string]any) {
	inExpr, err := ParseUsageExpr("$.usage.input_tokens + $.request_usage.text_tokens")
	if err != nil {
		panic(err)
	}
	totalExpr, err := ParseUsageExpr("$.usage.total_tokens + $.summary.extra_total")
	if err != nil {
		panic(err)
	}
	cfg := prepareUsageExtractConfig(UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Source: "response", Path: "$.usage.input_tokens"},
			{Dimension: "output", Unit: "token", Source: "response", SumPath: "$.usage.output_breakdown[*].tokens"},
			{Dimension: "cache_read", Unit: "token", Source: "response", Path: "$.usage.input_tokens_details.cached_tokens"},
			{Dimension: "cache_write", Unit: "token", Source: "response", Path: "$.usage.cache_creation.ephemeral_5m_input_tokens", Attrs: map[string]string{"ttl": "5m"}},
			{Dimension: "cache_write", Unit: "token", Source: "response", Path: "$.usage.cache_creation.ephemeral_1h_input_tokens", Attrs: map[string]string{"ttl": "1h"}},
			{Dimension: "server_tool.web_search", Unit: "call", Source: "response", CountPath: "$.output[*]", Type: "web_search_call", Status: "completed"},
			{Dimension: "input", Unit: "token", Source: "request", Expr: inExpr, Fallback: true},
		},
		TotalTokensExpr: totalExpr,
	})
	meta := &dslmeta.Meta{
		API:          "responses",
		IsStream:     false,
		RequestBody:  []byte(`{"usage":{"input_tokens":0},"request_usage":{"text_tokens":7}}`),
		DerivedUsage: map[string]any{"derived_tokens": 3},
	}
	root := map[string]any{
		"usage": map[string]any{
			"input_tokens": float64(101),
			"output_breakdown": []any{
				map[string]any{"tokens": float64(50)},
				map[string]any{"tokens": float64(25)},
			},
			"input_tokens_details": map[string]any{
				"cached_tokens": float64(23),
			},
			"cache_creation": map[string]any{
				"ephemeral_5m_input_tokens": float64(13),
				"ephemeral_1h_input_tokens": float64(29),
			},
			"total_tokens": float64(176),
		},
		"output": []any{
			map[string]any{"type": "web_search_call", "status": "completed"},
			map[string]any{"type": "web_search_call", "status": "completed"},
			map[string]any{"type": "web_search_call", "status": "failed"},
			map[string]any{"type": "message", "status": "completed"},
		},
		"summary": map[string]any{
			"extra_total": float64(9),
		},
	}
	body := []byte(`{
		"usage":{
			"input_tokens":101,
			"output_breakdown":[{"tokens":50},{"tokens":25}],
			"input_tokens_details":{"cached_tokens":23},
			"cache_creation":{"ephemeral_5m_input_tokens":13,"ephemeral_1h_input_tokens":29},
			"total_tokens":176
		},
		"output":[
			{"type":"web_search_call","status":"completed"},
			{"type":"web_search_call","status":"completed"},
			{"type":"web_search_call","status":"failed"},
			{"type":"message","status":"completed"}
		],
		"summary":{"extra_total":9}
	}`)
	return meta, cfg, body, root
}
