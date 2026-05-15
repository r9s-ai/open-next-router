package usageestimate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestEstimate_WhenMissingUsage_EstimateBoth(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:   "chat.completions",
		Model: "gpt-4o-mini",
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"hello"}]
		}`),
		ResponseBody: []byte(`{
			"id":"x",
			"choices":[{"index":0,"message":{"role":"assistant","content":"world"}}]
		}`),
	})

	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage = %q, want %q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.TotalTokens <= 0 {
		t.Fatalf("total_tokens = %d, want > 0", out.Usage.TotalTokens)
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens = %d, want > 0", out.Usage.InputTokens)
	}
}

func TestEstimate_UsesProvidedRequestRootWithoutParsingBody(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:         "chat.completions",
		Model:       "gpt-4o-mini",
		RequestBody: []byte("{"),
		RequestRoot: map[string]any{
			"messages": []any{
				map[string]any{"role": "user", "content": "hello"},
			},
		},
		ResponseBody: []byte(`{
			"id":"x",
			"choices":[{"index":0,"message":{"role":"assistant","content":"world"}}]
		}`),
	})

	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage = %q, want %q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens = %d, want > 0", out.Usage.InputTokens)
	}
}

func TestEstimate_WhenUpstreamUsagePresent_Upstream(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 12 {
		t.Fatalf("usage total_tokens = %#v, want 12", out.Usage)
	}
}

func TestEstimate_WhenUpstreamFactsPresent_PreservesFactLevelFields(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:   "claude.messages",
		Model: "claude-haiku-4-5",
		UpstreamUsage: &dslconfig.Usage{
			InputTokens:  10,
			OutputTokens: 2,
			TotalTokens:  12,
			FlatFields: map[string]any{
				"cache_write_ttl_5m_tokens": 6802,
			},
			DebugFacts: []dslconfig.UsageFact{
				{
					Dimension: "cache_write",
					Unit:      "token",
					Quantity:  6802,
					Attributes: map[string]string{
						"ttl": "5m",
					},
				},
			},
		},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if got, want := out.Usage.FlatFields["cache_write_ttl_5m_tokens"], 6802; got != want {
		t.Fatalf("flat field = %#v, want %d", got, want)
	}
	if len(out.Usage.DebugFacts) != 1 {
		t.Fatalf("debug facts len = %d, want 1", len(out.Usage.DebugFacts))
	}
	if out.Usage.DebugFacts[0].Dimension != "cache_write" {
		t.Fatalf("debug fact dimension = %q, want cache_write", out.Usage.DebugFacts[0].Dimension)
	}
}

func TestEstimate_NormalizeTotalTokens(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 11, OutputTokens: 9, TotalTokens: 0},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 20 {
		t.Fatalf("total_tokens = %v, want 20", out.Usage)
	}
}

func TestEstimate_WhenAllZeroUsage_Estimates(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 0, TotalTokens: 0},
		RequestBody:   []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody:  []byte(`{"choices":[{"message":{"role":"assistant","content":"world"}}]}`),
	})
	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil || out.Usage.TotalTokens <= 0 {
		t.Fatalf("expected estimated usage, got %#v", out.Usage)
	}
}

func TestEstimate_WhenUpstreamMissingOutputTokens_EstimateCompletion(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	sse := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		StreamTail:    []byte(sse),
	})
	if out.Stage != StageEstimateCompletion {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateCompletion)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.InputTokens != 6 {
		t.Fatalf("input_tokens=%d want=6", out.Usage.InputTokens)
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatalf("output_tokens=%d want > 0", out.Usage.OutputTokens)
	}
	if out.Usage.TotalTokens != out.Usage.InputTokens+out.Usage.OutputTokens {
		t.Fatalf("total_tokens=%d want=%d", out.Usage.TotalTokens, out.Usage.InputTokens+out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenEstimatingMissingScalarFields_DoesNotSynthesizeFactLevelFields(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	sse := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	out := Estimate(cfg, Input{
		API:   "claude.messages",
		Model: "claude-haiku-4-5",
		UpstreamUsage: &dslconfig.Usage{
			InputTokens:  6,
			OutputTokens: 0,
			TotalTokens:  6,
			FlatFields: map[string]any{
				"cache_write_ttl_5m_tokens": 6802,
			},
			DebugFacts: []dslconfig.UsageFact{
				{
					Dimension: "cache_write",
					Unit:      "token",
					Quantity:  6802,
					Attributes: map[string]string{
						"ttl": "5m",
					},
				},
			},
		},
		StreamTail: []byte(sse),
	})
	if out.Stage != StageEstimateCompletion {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateCompletion)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatalf("output_tokens=%d want > 0", out.Usage.OutputTokens)
	}
	if got, want := out.Usage.FlatFields["cache_write_ttl_5m_tokens"], 6802; got != want {
		t.Fatalf("flat field = %#v, want %d", got, want)
	}
	if len(out.Usage.FlatFields) != 1 {
		t.Fatalf("flat fields len = %d, want 1", len(out.Usage.FlatFields))
	}
	if len(out.Usage.DebugFacts) != 1 {
		t.Fatalf("debug facts len = %d, want 1", len(out.Usage.DebugFacts))
	}
	if out.Usage.DebugFacts[0].Dimension != "cache_write" {
		t.Fatalf("debug fact dimension = %q, want cache_write", out.Usage.DebugFacts[0].Dimension)
	}
}

func TestEstimate_WhenUpstreamMissingInputTokens_EstimatePrompt(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 8, TotalTokens: 8},
		RequestBody:   []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
	})
	if out.Stage != StageEstimatePrompt {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimatePrompt)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens != 8 {
		t.Fatalf("output_tokens=%d want=8", out.Usage.OutputTokens)
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens=%d want > 0", out.Usage.InputTokens)
	}
	if out.Usage.TotalTokens != out.Usage.InputTokens+out.Usage.OutputTokens {
		t.Fatalf("total_tokens=%d want=%d", out.Usage.TotalTokens, out.Usage.InputTokens+out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenMissingOutputTokensButNoText_DontEstimateCompletion(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		StreamTail:    []byte("data: [DONE]\n\n"),
	})
	if out.Stage != StageUpstream {
		t.Fatalf("stage=%q want=%q", out.Stage, StageUpstream)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens != 0 {
		t.Fatalf("output_tokens=%d want=0", out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenEstimationDisabled_ReturnsNilOnMissing(t *testing.T) {
	cfg := &Config{
		Enabled:                   true,
		EstimateWhenMissingOrZero: false,
		Strategy:                  "heuristic",
		MaxRequestBytes:           1024,
		MaxResponseBytes:          1024,
		MaxStreamCollectBytes:     1024,
		APIs:                      []string{"chat.completions"},
	}

	out := Estimate(cfg, Input{
		API:   "chat.completions",
		Model: "gpt-4o-mini",
	})
	if out.Stage != "" || out.Usage != nil {
		t.Fatalf("expected empty output, got stage=%q usage=%#v", out.Stage, out.Usage)
	}
}

// Test Anthropic SSE extraction and token estimation. This test is for development use only.
func TestEstimate_AnthropicStreamOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)
	streamTail := []byte(``)
	esc := `
用户指出可能是 tool_use 而不是文本的问题。我已经看到了：
1. 'meta.StreamText' 在 native_passthrough.go 中被追加文本内容
2. 注释说 "tool_use input_json_delta is not appended to StreamText"

这很关键！说明 tool_use 内容**不会**被添加到 StreamText。

我需要进一步排查：
1. 'onrusageestimate.Estimate()' 函数如何根据 StreamText 和 ResponseBody 来估计 completion_tokens
2. 当是 tool_use 输出时，ResponseBody 和 StreamText 的内容是什么
3. 估计逻辑是否对 tool_use 有特殊处理

我应该读：
1. 'relay/internal/onrusageestimate' 包中的估计逻辑
2. 确认 tool_use 时 ResponseBody 和 StreamText 的填充情况

初步判断：如果 tool_use 不被追加到 StreamText，且 ResponseBody 可能为空（因为是流式），那么估计器没有足够的输出内容来计算 completion_tokens，导致为 0。基于目前的发现，我需要读以下函数来完整判断：

1. **'onrusageestimate.Estimate()' 函数** - 看它如何从 StreamText 和 ResponseBody 来估计 completion_tokens
2. **native_passthrough.go 中的 tool_use 处理部分** - 确认 tool_use 块是否被正确存储

**初步判断：**
根据注释 '// tool_use input_json_delta is not appended to StreamText'，我怀疑问题根源是：
- 当输出是 'tool_use' 块而非文本时，**tool_use 内容没有被追加到 'meta.StreamText'**
- 而流式响应的 'meta.ResponseBody' 可能为空（因为是流式，没有完整响应体）
- 导致估计器接收到的 'StreamText' 为空，无法估算 'completion_tokens'，最终为 0

让我读这两个关键位置确认：read_file{"path": "relay/internal/channel/claudemessages/native_passthrough.go", "start_line": 210, "end_line": 260}search_code{"pattern": "func Estimate", "path": "relay/internal/onrusageestimate"}`
	payload, err := json.Marshal(map[string]string{"text": esc}) // Escape special characters.
	if err == nil {
		streamTail = []byte("data: " + string(payload) + "\n\n")
	} else {
		streamTail = []byte("data: \"\"\n\n")
	}
	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6}, //in 2342 out785
		StreamTail:    streamTail,
	})
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	t.Logf("official output=785, estimated=%d", out.Usage.OutputTokens)
}

func TestExtractStreamText_ChatCompletionsDelta(t *testing.T) {
	t.Parallel()

	sse := strings.Join([]string{
		`data: {"id":"x","choices":[{"delta":{"content":"hel"}}]}`,
		"",
		`data: {"id":"x","choices":[{"delta":{"content":"lo"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	got := extractStreamText("chat.completions", []byte(sse), 1024)
	if strings.ReplaceAll(got, "\n", "") != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}

func TestStringifyAnthropicRequest_SystemOnly(t *testing.T) {
	ctx := stringifyAnthropicRequest(mustJSONMap(t, `{
		"system": [{"type": "text", "text": "You are a careful relay debugging agent."}]
	}`))

	if ctx.numTools != 0 {
		t.Fatalf("numTools=%d want=0", ctx.numTools)
	}
	assertContainsAll(t, ctx.text, "system", "You are a careful relay debugging agent.")
}

func TestStringifyAnthropicRequest_ToolsOnly(t *testing.T) {
	ctx := stringifyAnthropicRequest(mustJSONMap(t, `{
		"tools": [
			{
				"name": "read_file",
				"description": "Read a repository file.",
				"input_schema": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "Repository-relative file path."}
					},
					"required": ["path"]
				}
			},
			{
				"name": "search_code",
				"description": "Search repository code.",
				"input_schema": {
					"type": "object",
					"properties": {
						"pattern": {"type": "string"}
					},
					"required": ["pattern"]
				}
			}
		]
	}`))

	if ctx.numTools != 2 {
		t.Fatalf("numTools=%d want=2", ctx.numTools)
	}
	assertContainsAll(t, ctx.text, "tools", "read_file", "Repository-relative file path", "search_code", "pattern")
}

func TestStringifyAnthropicRequest_MessagesOnly(t *testing.T) {
	ctx := stringifyAnthropicRequest(mustJSONMap(t, `{
		"messages": [
			{"role": "user", "content": "请查询上海天气。"},
			{
				"role": "assistant",
				"content": [
					{"type": "text", "text": "我会先调用天气工具。"},
					{"type": "thinking", "thinking": "需要城市 Shanghai。"},
					{"type": "tool_use", "name": "get_weather", "input": {"city": "Shanghai"}}
				]
			},
			{
				"role": "user",
				"content": [
					{"type": "tool_result", "content": [{"type": "text", "text": "上海多云。"}]}
				]
			}
		]
	}`))

	if ctx.numTools != 0 {
		t.Fatalf("numTools=%d want=0", ctx.numTools)
	}
	assertContainsAll(t, ctx.text, "messages", "role:user", "请查询上海天气。", "thinking 需要城市 Shanghai。", "tool_use get_weather", "city Shanghai", "上海多云。")
}

func TestStringifyOpenAIResponsesRequest_InstructionsOnly(t *testing.T) {
	ctx := stringfyOpenaiResponsesRequest(mustJSONMap(t, `{
		"instructions": "Answer in concise Chinese."
	}`))

	if ctx.numTools != 0 {
		t.Fatalf("numTools=%d want=0", ctx.numTools)
	}
	assertContainsAll(t, ctx.text, "instructions", "Answer in concise Chinese.")
}

func TestStringifyOpenAIResponsesRequest_ToolsOnly(t *testing.T) {
	ctx := stringfyOpenaiResponsesRequest(mustJSONMap(t, `{
		"tools": [
			{
				"type": "function",
				"name": "get_weather",
				"description": "Get weather by city.",
				"parameters": {
					"type": "object",
					"properties": {"city": {"type": "string"}}
				}
			},
			{
				"type": "function",
				"name": "search_database",
				"description": "Search records.",
				"parameters": {
					"type": "object",
					"properties": {"query": {"type": "string"}}
				}
			}
		]
	}`))

	if ctx.numTools != 2 {
		t.Fatalf("numTools=%d want=2", ctx.numTools)
	}
	assertContainsAll(t, ctx.text, "tools", "get_weather", "Get weather by city.", "city", "search_database", "Search records.", "query")
}

func TestStringifyOpenAIResponsesRequest_InputItems(t *testing.T) {
	ctx := stringfyOpenaiResponsesRequest(mustJSONMap(t, `{
		"input": [
			{"role": "user", "content": [{"type": "input_text", "text": "Analyze these two charts."}]},
			{"type": "reasoning", "summary": [{"type": "summary_text", "text": "Need compare weather trends."}]},
			{"type": "function_call", "name": "get_weather", "arguments": {"city": "Beijing"}},
			{"type": "function_call_output", "output": {"temperature": 21}},
			{"type": "custom_tool_call", "name": "run_sql", "input": "select 1"}
		]
	}`))

	if ctx.numFunctionCalls != 1 {
		t.Fatalf("numFunctionCalls=%d want=1", ctx.numFunctionCalls)
	}
	if ctx.numFunctionCallOutputs != 1 {
		t.Fatalf("numFunctionCallOutputs=%d want=1", ctx.numFunctionCallOutputs)
	}
	if ctx.numCustomToolCalls != 1 {
		t.Fatalf("numCustomToolCalls=%d want=1", ctx.numCustomToolCalls)
	}
	assertContainsAll(t, ctx.text, "Analyze these two charts.", "Need compare weather trends.", "function_call get_weather", `"city":"Beijing"`, "function_call_output", "temperature", "custom_tool_call run_sql", "select 1")
}

func TestExtractResponseText_OpenAIResponses(t *testing.T) {
	body := []byte(`{
		"output": [
			{
				"type": "message",
				"content": [{"type": "output_text", "text": "综合分析报告"}]
			},
			{
				"type": "function_call",
				"name": "get_weather",
				"arguments": {"city": "Beijing"}
			}
		]
	}`)

	got := extractResponseText("responses", body, -1)
	assertContainsAll(t, got, "综合分析报告", "function_call get_weather", `"city":"Beijing"`)
}

func TestStringifyOpenAIChatCompletionsRequest_MessagesOnly(t *testing.T) {
	ctx := stringfyOpenaiChatCompletionsRequest(mustJSONMap(t, `{
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": [
				{"type": "text", "text": "Analyze these two charts."},
				{"type": "image_url", "image_url": {"url": "https://example.com/chart.png"}}
			]},
			{"role": "assistant", "tool_calls": [
				{"type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Beijing\"}"}}
			]},
			{"role": "tool", "content": "Beijing is sunny."}
		]
	}`))

	if ctx.numTools != 0 {
		t.Fatalf("numTools=%d want=0", ctx.numTools)
	}
	if ctx.numFunctionCalls != 1 {
		t.Fatalf("numFunctionCalls=%d want=1", ctx.numFunctionCalls)
	}
	assertContainsAll(t, ctx.text, "role:system", "You are helpful.", "Analyze these two charts.", "function_call get_weather", `{"city":"Beijing"}`, "Beijing is sunny.")
}

func TestStringifyOpenAIChatCompletionsRequest_ToolsOnly(t *testing.T) {
	ctx := stringfyOpenaiChatCompletionsRequest(mustJSONMap(t, `{
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get weather.",
					"parameters": {
						"type": "object",
						"properties": {"city": {"type": "string"}}
					}
				}
			},
			{
				"type": "function",
				"function": {
					"name": "search_database",
					"description": "Search database.",
					"parameters": {
						"type": "object",
						"properties": {"query": {"type": "string"}}
					}
				}
			}
		]
	}`))

	if ctx.numTools != 2 {
		t.Fatalf("numTools=%d want=2", ctx.numTools)
	}
	assertContainsAll(t, ctx.text, "tools", "get_weather", "Get weather.", "city", "search_database", "Search database.", "query")
}

func TestExtractResponseText_OpenAIChatCompletions(t *testing.T) {
	body := []byte(`{
		"choices": [
			{
				"message": {
					"content": "综合分析报告",
					"tool_calls": [
						{"type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Beijing\"}"}}
					],
					"function_call": {"name": "legacy_lookup", "arguments": "{\"query\":\"history\"}"}
				}
			},
			{"text": "Legacy text choice fallback"}
		]
	}`)

	got := extractResponseText("chat.completions", body, -1)
	assertContainsAll(t, got, "综合分析报告", "function_call get_weather", `{"city":"Beijing"}`, "function_call legacy_lookup", `{"query":"history"}`, "Legacy text choice fallback")
}

func mustJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	return m
}

func assertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()

	if got == "" {
		t.Fatalf("got empty string, want to contain %v", wants)
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected text to contain %q, got %q", want, got)
		}
	}
}
