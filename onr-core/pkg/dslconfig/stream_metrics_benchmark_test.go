package dslconfig

import (
	"bytes"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func BenchmarkStreamMetricsAggregator_OpenAIEvent_New(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	payload := []byte(`{"choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":123,"completion_tokens":45,"total_tokens":168,"prompt_tokens_details":{"cached_tokens":11}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)
		if err := agg.OnSSEDataJSON(payload); err != nil {
			b.Fatalf("OnSSEDataJSON: %v", err)
		}
	}
}

func BenchmarkStreamMetricsAggregator_OpenAIEvent_Old(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	payload := []byte(`{"choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":123,"completion_tokens":45,"total_tokens":168,"prompt_tokens_details":{"cached_tokens":11}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)
		if err := benchmarkLegacyOnSSEDataJSON(agg, payload); err != nil {
			b.Fatalf("benchmarkLegacyOnSSEDataJSON: %v", err)
		}
	}
}

func BenchmarkStreamMetricsAggregator_AnthropicEvent_New(b *testing.B) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigsTB(b, "anthropic.conf", meta.API, meta.IsStream)
	payload := []byte(`{"type":"message_start","message":{"stop_reason":null,"usage":{"input_tokens":321,"output_tokens":0,"cache_read_input_tokens":7,"cache_creation":{"ephemeral_5m_input_tokens":13},"cache_creation_input_tokens":13}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)
		if err := agg.OnSSEDataJSON(payload); err != nil {
			b.Fatalf("OnSSEDataJSON: %v", err)
		}
	}
}

func BenchmarkStreamMetricsAggregator_AnthropicEvent_Old(b *testing.B) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigsTB(b, "anthropic.conf", meta.API, meta.IsStream)
	payload := []byte(`{"type":"message_start","message":{"stop_reason":null,"usage":{"input_tokens":321,"output_tokens":0,"cache_read_input_tokens":7,"cache_creation":{"ephemeral_5m_input_tokens":13},"cache_creation_input_tokens":13}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)
		if err := benchmarkLegacyOnSSEDataJSON(agg, payload); err != nil {
			b.Fatalf("benchmarkLegacyOnSSEDataJSON: %v", err)
		}
	}
}

func BenchmarkStreamMetricsAggregator_CustomLegacyEvent_New(b *testing.B) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	inExpr, err := ParseUsageExpr("$.usage.input_tokens + $.message.usage.input_tokens")
	if err != nil {
		b.Fatalf("ParseUsageExpr input: %v", err)
	}
	outExpr, err := ParseUsageExpr("$.usage.output_tokens + $.message.usage.output_tokens")
	if err != nil {
		b.Fatalf("ParseUsageExpr output: %v", err)
	}
	usageCfg := UsageExtractConfig{
		Mode:             "custom",
		InputTokensExpr:  inExpr,
		OutputTokensExpr: outExpr,
	}
	_, finishCfg := mustLoadProviderMatchConfigsTB(b, "anthropic.conf", meta.API, meta.IsStream)
	payload := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":9,"output_tokens":1}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, &usageCfg, finishCfg)
		if err := agg.OnSSEDataJSON(payload); err != nil {
			b.Fatalf("OnSSEDataJSON: %v", err)
		}
	}
}

func BenchmarkStreamMetricsAggregator_CustomLegacyEvent_Old(b *testing.B) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	inExpr, err := ParseUsageExpr("$.usage.input_tokens + $.message.usage.input_tokens")
	if err != nil {
		b.Fatalf("ParseUsageExpr input: %v", err)
	}
	outExpr, err := ParseUsageExpr("$.usage.output_tokens + $.message.usage.output_tokens")
	if err != nil {
		b.Fatalf("ParseUsageExpr output: %v", err)
	}
	usageCfg := UsageExtractConfig{
		Mode:             "custom",
		InputTokensExpr:  inExpr,
		OutputTokensExpr: outExpr,
	}
	_, finishCfg := mustLoadProviderMatchConfigsTB(b, "anthropic.conf", meta.API, meta.IsStream)
	payload := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":9,"output_tokens":1}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, &usageCfg, finishCfg)
		if err := benchmarkLegacyOnSSEDataJSON(agg, payload); err != nil {
			b.Fatalf("benchmarkLegacyOnSSEDataJSON: %v", err)
		}
	}
}

func BenchmarkStreamMetricsAggregator_UsageRootFinalManyChunks(b *testing.B) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	usagePayload := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":123,"output_tokens":45,"input_tokens_details":{"cached_tokens":11}},"output":[{"type":"web_search_call","status":"completed"}]}}`)
	deltaPayload := []byte(`{"type":"response.output_text.delta","delta":"hello"}`)

	oldCfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Source: "response", Path: "$.response.usage.input_tokens", Event: "response.completed|response.incomplete", EventOptional: true},
			{Dimension: "output", Unit: "token", Source: "response", Path: "$.response.usage.output_tokens", Event: "response.completed|response.incomplete", EventOptional: true},
			{Dimension: "cache_read", Unit: "token", Source: "response", Path: "$.response.usage.input_tokens_details.cached_tokens", Event: "response.completed|response.incomplete", EventOptional: true},
			{Dimension: "server_tool.web_search", Unit: "call", Source: "response", CountPath: "$.response.output[*]", Type: "web_search_call", Status: "completed", Event: "response.completed|response.incomplete", EventOptional: true},
		},
	}
	newCfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.response.usage", Event: "response.completed|response.incomplete", EventOptional: true},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.output_tokens"},
			{Dimension: "cache_read", Unit: "token", Path: "$.input_tokens_details.cached_tokens"},
			{Dimension: "server_tool.web_search", Unit: "call", Source: "response", CountPath: "$.response.output[*]", Type: "web_search_call", Status: "completed", Event: "response.completed|response.incomplete", EventOptional: true},
		},
	}

	bench := func(b *testing.B, cfg *UsageExtractConfig) {
		b.Helper()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			agg := NewStreamMetricsAggregator(meta, cfg, nil)
			for j := 0; j < 128; j++ {
				if err := agg.OnSSEEventDataJSON("response.output_text.delta", deltaPayload); err != nil {
					b.Fatalf("OnSSEEventDataJSON delta: %v", err)
				}
			}
			if err := agg.OnSSEEventDataJSON("response.completed", usagePayload); err != nil {
				b.Fatalf("OnSSEEventDataJSON usage: %v", err)
			}
			u, cached, _, ok := agg.Result()
			if !ok || u == nil || u.InputTokens != 123 || u.OutputTokens != 45 || cached != 11 {
				b.Fatalf("unexpected result usage=%+v cached=%d ok=%v", u, cached, ok)
			}
		}
	}

	b.Run("legacy_response_facts_each_chunk", func(b *testing.B) {
		bench(b, &oldCfg)
	})
	b.Run("usage_root_final_facts", func(b *testing.B) {
		bench(b, &newCfg)
	})
}

// benchmarkLegacyOnSSEDataJSON requires a valid StreamMetricsAggregator receiver.
func benchmarkLegacyOnSSEDataJSON(a *StreamMetricsAggregator, payload []byte) error {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil
	}

	if strings.TrimSpace(a.finishReason) == "" && (strings.TrimSpace(a.finishCfg.Mode) != "" || strings.TrimSpace(a.finishCfg.FinishReasonPath) != "") {
		if v, err := ExtractFinishReason(a.meta, a.finishCfg, payload); err == nil {
			if s := strings.TrimSpace(v); s != "" {
				a.finishReason = s
			}
		}
	}

	if strings.TrimSpace(a.usageCfg.Mode) == "" {
		return nil
	}

	u, cachedTokens, err := ExtractUsage(a.meta, a.usageCfg, payload)
	if err != nil {
		return nil
	}
	if u == nil || isAllZeroUsage(u) {
		return nil
	}
	if a.lastUsage == nil {
		a.lastUsage = u
	} else {
		mergeUsagePreferNonZero(a.lastUsage, u)
	}
	if cachedTokens > 0 {
		a.lastCachedTokens = cachedTokens
	}
	if a.usageCfg != nil && shouldRecomputeMergedTotal(*a.usageCfg) && a.lastUsage != nil {
		normalizeUsageFields(a.lastUsage)
		a.lastUsage.TotalTokens = a.lastUsage.InputTokens + a.lastUsage.OutputTokens
	}
	return nil
}
