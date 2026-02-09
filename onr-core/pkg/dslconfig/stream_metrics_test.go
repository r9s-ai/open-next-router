package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestStreamMetricsAggregator_OpenAIUsageLast_FinishFirst(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}
	agg := NewStreamMetricsAggregator(meta,
		UsageExtractConfig{Mode: "openai"},
		FinishReasonExtractConfig{Mode: "openai"},
	)

	_ = agg.OnSSEDataJSON([]byte(`{"choices":[{"finish_reason":"stop"}]}`))
	_ = agg.OnSSEDataJSON([]byte(`{"choices":[{"finish_reason":"length"}]}`))
	_ = agg.OnSSEDataJSON([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	_ = agg.OnSSEDataJSON([]byte(`{"usage":{"prompt_tokens":9,"completion_tokens":8,"total_tokens":17}}`))

	u, _, fr, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.TotalTokens != 17 {
		t.Fatalf("unexpected total tokens: %d", u.TotalTokens)
	}
	// finish_reason: first non-empty
	if fr != "stop" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_AnthropicSnapshot(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	agg := NewStreamMetricsAggregator(meta,
		UsageExtractConfig{Mode: "anthropic"},
		FinishReasonExtractConfig{Mode: "anthropic"},
	)

	// message_start: usage under message.usage
	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":0}}}`))
	// message_delta: usage under top-level usage
	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_delta","usage":{"input_tokens":0,"output_tokens":7,"cache_read_input_tokens":2,"cache_creation_input_tokens":5}}`))
	// stop_reason appears in delta for anthropic
	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`))

	u, cached, fr, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 3 || u.OutputTokens != 7 || u.TotalTokens != 10 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("unexpected cached tokens: %d", cached)
	}
	if fr != "end_turn" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_GeminiUsage(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.streamGenerateContent", IsStream: true}
	agg := NewStreamMetricsAggregator(meta,
		UsageExtractConfig{Mode: "gemini"},
		FinishReasonExtractConfig{Mode: "gemini"},
	)
	_ = agg.OnSSEDataJSON([]byte(`{"candidates":[{"finishReason":"STOP"}]}`))
	_ = agg.OnSSEDataJSON([]byte(`{"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2,"thoughtsTokenCount":3,"totalTokenCount":6}}`))
	u, _, fr, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 1 || u.OutputTokens != 5 || u.TotalTokens != 6 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if fr != "STOP" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_CustomMerge_DoesNotOverrideWithZero(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}

	inExpr, err := ParseUsageExpr("$.usage.input_tokens + $.message.usage.input_tokens")
	if err != nil {
		t.Fatalf("ParseUsageExpr input: %v", err)
	}
	outExpr, err := ParseUsageExpr("$.usage.output_tokens + $.message.usage.output_tokens")
	if err != nil {
		t.Fatalf("ParseUsageExpr output: %v", err)
	}

	agg := NewStreamMetricsAggregator(meta,
		UsageExtractConfig{
			Mode:             "custom",
			InputTokensExpr:  inExpr,
			OutputTokensExpr: outExpr,
		},
		FinishReasonExtractConfig{Mode: "anthropic"},
	)

	// event 1: input tokens appear under message.usage
	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":9,"output_tokens":1}}}`))
	// event 2: output tokens appear under usage, but input is missing (zero) in this event
	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_delta","usage":{"output_tokens":18}}`))

	u, _, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 9 || u.OutputTokens != 18 || u.TotalTokens != 27 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
}
