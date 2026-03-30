package dslconfig

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func BenchmarkStreamMetricsAggregator_OpenAIEvent_New(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}
	usageCfg := UsageExtractConfig{Mode: "openai"}
	finishCfg := FinishReasonExtractConfig{Mode: "openai"}
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
	usageCfg := UsageExtractConfig{Mode: "openai"}
	finishCfg := FinishReasonExtractConfig{Mode: "openai"}
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
	usageCfg := UsageExtractConfig{Mode: "anthropic"}
	finishCfg := FinishReasonExtractConfig{Mode: "anthropic"}
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
	usageCfg := UsageExtractConfig{Mode: "anthropic"}
	finishCfg := FinishReasonExtractConfig{Mode: "anthropic"}
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
	finishCfg := FinishReasonExtractConfig{Mode: "anthropic"}
	payload := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":9,"output_tokens":1}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)
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
	finishCfg := FinishReasonExtractConfig{Mode: "anthropic"}
	payload := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":9,"output_tokens":1}}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)
		if err := benchmarkLegacyOnSSEDataJSON(agg, payload); err != nil {
			b.Fatalf("benchmarkLegacyOnSSEDataJSON: %v", err)
		}
	}
}

func benchmarkLegacyOnSSEDataJSON(a *StreamMetricsAggregator, payload []byte) error {
	if a == nil {
		return nil
	}
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

	mode := strings.ToLower(strings.TrimSpace(a.usageCfg.Mode))
	if mode == "" {
		return nil
	}

	extractPayload := payload
	if mode == usageModeAnthropic {
		extractPayload = benchmarkNormalizeAnthropicStreamUsagePayload(payload)
	}

	u, cachedTokens, err := ExtractUsage(a.meta, a.usageCfg, extractPayload)
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
	if shouldRecomputeMergedTotal(strings.TrimSpace(a.usageCfg.Mode), a.usageCfg) && a.lastUsage != nil {
		normalizeUsageFields(a.lastUsage)
		a.lastUsage.TotalTokens = a.lastUsage.InputTokens + a.lastUsage.OutputTokens
	}
	return nil
}

func benchmarkNormalizeAnthropicStreamUsagePayload(payload []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return payload
	}
	if _, ok := obj["usage"]; ok {
		return payload
	}
	msg, ok := obj["message"].(map[string]any)
	if !ok || msg == nil {
		return payload
	}
	usage, ok := msg["usage"]
	if !ok {
		return payload
	}
	obj["usage"] = usage
	normalized, err := json.Marshal(obj)
	if err != nil || len(normalized) == 0 {
		return payload
	}
	return normalized
}
