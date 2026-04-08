package dslconfig

import (
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestStreamMetricsAggregator_OpenAIUsageLast_FinishFirst(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
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
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
	)

	// message_start: usage under message.usage
	_ = agg.OnSSEEventDataJSON("message_start", []byte(`{"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":5},"cache_creation_input_tokens":5}}}`))
	// message_delta: usage under top-level usage
	_ = agg.OnSSEEventDataJSON("message_delta", []byte(`{"type":"message_delta","usage":{"input_tokens":0,"output_tokens":7,"cache_read_input_tokens":2,"cache_creation_input_tokens":5}}`))
	// stop_reason appears in delta for anthropic
	_ = agg.OnSSEEventDataJSON("message_delta", []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`))

	u, cached, fr, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 10 || u.OutputTokens != 7 || u.TotalTokens != 17 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("unexpected cached tokens: %d", cached)
	}
	if u.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := u.FlatFields["cache_write_ttl_5m_tokens"], 5; got != want {
		t.Fatalf("cache_write_ttl_5m_tokens got %v, want %v", got, want)
	}
	if len(u.DebugFacts) == 0 {
		t.Fatalf("expected DebugFacts")
	}
	var foundCacheWrite bool
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "cache_write" && fact.Quantity == 5 && fact.Attributes["ttl"] == "5m" {
			foundCacheWrite = true
			break
		}
	}
	if !foundCacheWrite {
		t.Fatalf("expected cache_write ttl debug fact, got %+v", u.DebugFacts)
	}
	if fr != "end_turn" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_AnthropicSnapshot_DoesNotOverridePositiveWithZero(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
	)

	_ = agg.OnSSEEventDataJSON("message_start", []byte(`{"type":"message_start","message":{"usage":{"input_tokens":12}}}`))
	_ = agg.OnSSEEventDataJSON("message_delta", []byte(`{"type":"message_delta","usage":{"cache_read_input_tokens":4,"cache_creation_input_tokens":9}}`))
	_ = agg.OnSSEEventDataJSON("message_delta", []byte(`{"type":"message_delta","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`))
	_ = agg.OnSSEEventDataJSON("message_delta", []byte(`{"type":"message_delta","usage":{"output_tokens":6}}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 25 || u.OutputTokens != 6 || u.TotalTokens != 31 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 4 {
		t.Fatalf("unexpected cached tokens: %d", cached)
	}
	if u.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := u.InputTokenDetails.CachedTokens, 4; got != want {
		t.Fatalf("CachedTokens got %d, want %d", got, want)
	}
	if got, want := u.InputTokenDetails.CacheWriteTokens, 9; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
}

func TestStreamMetricsAggregator_AnthropicProviderSnapshot_WebSearchProjection(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)

	_ = agg.OnSSEEventDataJSON("message_start", []byte(`{"type":"message_start","message":{"usage":{"input_tokens":3,"cache_creation":{"ephemeral_5m_input_tokens":5},"cache_creation_input_tokens":5,"server_tool_use":{"web_search_requests":1}}}}`))
	_ = agg.OnSSEEventDataJSON("message_delta", []byte(`{"type":"message_delta","usage":{"output_tokens":7,"cache_read_input_tokens":2,"cache_creation_input_tokens":5}}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 10 || u.OutputTokens != 7 || u.TotalTokens != 17 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("unexpected cached tokens: %d", cached)
	}
	if u.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["cache_write_ttl_5m_tokens"], 5; got != want {
		t.Fatalf("cache_write_ttl_5m_tokens=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "server_tool.web_search" && fact.Unit == "call" && fact.Quantity == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected server_tool.web_search call fact, got=%#v", u.DebugFacts)
	}
}

func TestStreamMetricsAggregator_GeminiUsage(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.streamGenerateContent", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
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

func TestStreamMetricsAggregator_GeminiUsageMultimodalBuiltin(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.streamGenerateContent", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
	)
	_ = agg.OnSSEDataJSON([]byte(`{"candidates":[{"finishReason":"STOP"}]}`))
	_ = agg.OnSSEDataJSON([]byte(`{
	  "usageMetadata":{
	    "promptTokenCount":81,
	    "candidatesTokenCount":40,
	    "thoughtsTokenCount":553,
	    "totalTokenCount":674,
	    "promptTokensDetails":[
	      {"modality":"TEXT","tokenCount":5},
	      {"modality":"IMAGE","tokenCount":12},
	      {"modality":"VIDEO","tokenCount":34},
	      {"modality":"AUDIO","tokenCount":76}
	    ]
	  }
	}`))
	u, _, fr, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 5 || u.OutputTokens != 593 || u.TotalTokens != 674 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if got, want := u.FlatFields["image_input_tokens"], 12; got != want {
		t.Fatalf("image_input_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["video_input_tokens"], 34; got != want {
		t.Fatalf("video_input_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["audio_input_tokens"], 76; got != want {
		t.Fatalf("audio_input_tokens=%v want=%v", got, want)
	}
	if fr != "STOP" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_GeminiSnakeCaseUsageIgnored(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.streamGenerateContent", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
	)
	_ = agg.OnSSEDataJSON([]byte(`{"usage_metadata":{"prompt_token_count":1,"candidates_token_count":2,"thoughts_token_count":3,"total_token_count":6}}`))
	u, _, _, ok := agg.Result()
	if ok || u != nil {
		t.Fatalf("did not expect snake_case gemini stream usage to be extracted: ok=%v usage=%#v", ok, u)
	}
}

func TestStreamMetricsAggregator_OpenAIResponsesStreamEnvelopeFinishReason(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta,
		usageCfg,
		finishCfg,
	)

	_ = agg.OnSSEDataJSON([]byte(`{"type":"response.created","response":{"status":"in_progress"}}`))
	_ = agg.OnSSEDataJSON([]byte(`{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`))

	_, _, fr, ok := agg.Result()
	if ok {
		t.Fatalf("did not expect usage ok for finish_reason-only events")
	}
	if fr != "max_output_tokens" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_OpenAIResponsesStreamUsageUsesSSEEventFilter(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)

	_ = agg.OnSSEEventDataJSON("response.output_text.delta", []byte(`{"delta":"Hello"}`))
	_ = agg.OnSSEEventDataJSON("response.completed", []byte(`{
	  "type":"response.completed",
	  "response":{
	    "status":"completed",
	    "usage":{"input_tokens":11,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}},
	    "output":[
	      {"type":"web_search_call","status":"completed"},
	      {"type":"web_search_call","status":"failed"}
	    ]
	  }
	}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 11 || u.OutputTokens != 5 || u.TotalTokens != 16 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("cached=%d want=2", cached)
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
}

func TestStreamMetricsAggregator_FinishReasonUsesSSEEventFilter(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	finishCfg := FinishReasonExtractConfig{Mode: usageModeCustom}
	finishCfg.addFinishReasonPathRule("$.response.incomplete_details.reason", false, "response.incomplete", true)
	finishCfg.addFinishReasonPathRule("$.response.status", true, "response.completed", true)
	agg := NewStreamMetricsAggregator(meta, UsageExtractConfig{}, finishCfg)

	_ = agg.OnSSEEventDataJSON("response.output_text.delta", []byte(`{"response":{"status":"completed","incomplete_details":{"reason":"max_output_tokens"}}}`))
	_ = agg.OnSSEEventDataJSON("response.completed", []byte(`{"response":{"status":"completed"}}`))

	_, _, fr, ok := agg.Result()
	if ok {
		t.Fatalf("did not expect usage ok for finish_reason-only events")
	}
	if fr != "completed" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}

func TestStreamMetricsAggregator_OpenAIResponsesStreamUsageFallsBackWithoutEvent(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	usageCfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.response.usage.input_tokens", Event: "response.completed", EventOptional: true},
			{Dimension: "output", Unit: "token", Path: "$.response.usage.output_tokens", Event: "response.completed", EventOptional: true},
			{Dimension: "cache_read", Unit: "token", Path: "$.response.usage.input_tokens_details.cached_tokens", Event: "response.completed", EventOptional: true},
			{Dimension: "server_tool.web_search", Unit: "call", CountPath: "$.response.output[*]", Type: "web_search_call", Status: "completed", Event: "response.completed", EventOptional: true},
		},
	}
	finishCfg := FinishReasonExtractConfig{Mode: usageModeCustom}
	agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)

	_ = agg.OnSSEDataJSON([]byte(`{
	  "response":{
	    "status":"completed",
	    "usage":{"input_tokens":11,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}},
	    "output":[{"type":"web_search_call","status":"completed"}]
	  }
	}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 11 || u.OutputTokens != 5 || u.TotalTokens != 16 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("cached=%d want=2", cached)
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
}

func TestStreamMetricsAggregator_OpenAIResponsesProviderFallbackWithoutEvent_DoesNotDoubleCount(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)

	_ = agg.OnSSEDataJSON([]byte(`{
	  "response":{
	    "status":"completed",
	    "usage":{"input_tokens":11,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}},
	    "output":[{"type":"web_search_call","status":"completed"}]
	  }
	}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 11 || u.OutputTokens != 5 || u.TotalTokens != 16 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("cached=%d want=2", cached)
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
}

func TestStreamMetricsAggregator_AnthropicStreamUsageFallsBackWithoutEvent(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	usageCfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.message.usage.input_tokens", Event: "message_start", EventOptional: true},
			{Dimension: "input", Unit: "token", Path: "$.message.usage.cache_read_input_tokens", Event: "message_start", EventOptional: true},
			{Dimension: "input", Unit: "token", Path: "$.message.usage.cache_creation_input_tokens", Event: "message_start", EventOptional: true},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "message_delta", EventOptional: true},
			{Dimension: "cache_read", Unit: "token", Path: "$.message.usage.cache_read_input_tokens", Event: "message_start", EventOptional: true},
			{Dimension: "server_tool.web_search", Unit: "call", Path: "$.message.usage.server_tool_use.web_search_requests", Event: "message_start", EventOptional: true},
			{Dimension: "cache_write", Unit: "token", Path: "$.message.usage.cache_creation_input_tokens", Event: "message_start", EventOptional: true},
		},
	}
	finishCfg := FinishReasonExtractConfig{Mode: usageModeCustom}
	agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)

	_ = agg.OnSSEDataJSON([]byte(`{"message":{"usage":{"input_tokens":8,"cache_read_input_tokens":2,"cache_creation_input_tokens":3,"server_tool_use":{"web_search_requests":1}}},"usage":{"output_tokens":5}}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 13 || u.OutputTokens != 5 || u.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 {
		t.Fatalf("cached=%d want=2", cached)
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
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
		func() FinishReasonExtractConfig {
			_, cfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
			return cfg
		}(),
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

func TestStreamMetricsAggregator_CustomUsageFactMerge_CacheFirstOutputLater(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	agg := NewStreamMetricsAggregator(meta,
		UsageExtractConfig{
			Mode: "custom",
			facts: []usageFactConfig{
				{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens"},
				{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"},
				{Dimension: "cache_read", Unit: "token", Path: "$.usage.cache_read_input_tokens"},
				{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_5m_input_tokens", Attrs: map[string]string{"ttl": "5m"}},
				{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_1h_input_tokens", Attrs: map[string]string{"ttl": "1h"}},
				{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation_input_tokens", Fallback: true},
			},
		},
		func() FinishReasonExtractConfig {
			_, cfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
			return cfg
		}(),
	)

	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_start","usage":{"input_tokens":11,"cache_read_input_tokens":3,"cache_creation":{"ephemeral_5m_input_tokens":7,"ephemeral_1h_input_tokens":0},"cache_creation_input_tokens":7}}`))
	_ = agg.OnSSEDataJSON([]byte(`{"type":"message_delta","usage":{"output_tokens":5}}`))

	u, cached, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 11 || u.OutputTokens != 5 || u.TotalTokens != 16 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 3 {
		t.Fatalf("unexpected cached tokens: %d", cached)
	}
	if u.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := u.InputTokenDetails.CacheWriteTokens, 7; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
	if u.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := u.FlatFields["cache_write_ttl_5m_tokens"], 7; got != want {
		t.Fatalf("cache_write_ttl_5m_tokens got %v, want %v", got, want)
	}
	if got, want := u.FlatFields["cache_write_ttl_1h_tokens"], 0; got != want {
		t.Fatalf("cache_write_ttl_1h_tokens got %v, want %v", got, want)
	}
}

func TestStreamMetricsAggregator_OpenAIChatCompletionsMultimodalRealStream(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: true}
	usageCfg, finishCfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	agg := NewStreamMetricsAggregator(meta, usageCfg, finishCfg)

	agg.OnSSETail(mustReadSharedTestData(t, filepath.Join("openai", "chat_completions_multimodal_real.sse")))

	u, cached, fr, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 36852 || u.OutputTokens != 1 || u.TotalTokens != 36853 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
	if fr != "stop" {
		t.Fatalf("unexpected finish_reason: %q", fr)
	}
}
