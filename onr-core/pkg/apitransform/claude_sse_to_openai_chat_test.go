package apitransform

import (
	"bytes"
	"strings"
	"testing"
)

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_123","model":"claude-haiku-4-5-20251001"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(s, `"chat.completion.chunk"`, `"role":"assistant"`, `"content":"Hi"`, `"finish_reason":"stop"`) {
		t.Fatalf("unexpected output: %s", s)
	}
	if strings.Count(s, "data: [DONE]") != 1 {
		t.Fatalf("expected single done event, got: %s", s)
	}
}

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE_ToolUse(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool_1","model":"claude-haiku-4-5-20251001"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"SF\"}"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(s, `"tool_calls"`, `"name":"get_weather"`, `"arguments":"{\"city\":\"SF\"}"`, `"finish_reason":"tool_calls"`) {
		t.Fatalf("unexpected output: %s", s)
	}
	if strings.Count(s, "data: [DONE]") != 1 {
		t.Fatalf("expected single done event, got: %s", s)
	}
}

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE_EmitsFinalUsageChunk(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_usage_1","model":"claude-haiku-4-5-20251001","usage":{"input_tokens":11,"cache_read_input_tokens":2,"cache_creation_input_tokens":7}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":19}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(
		s,
		`"finish_reason":"stop"`,
		`"choices":[]`,
		`"completion_tokens":19`,
		`"prompt_tokens":20`,
		`"cached_tokens":2`,
		`"cache_write_tokens":7`,
		`"total_tokens":39`,
		"data: [DONE]",
	) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE_PreservesUsageIterations(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_usage_iterations","model":"claude-fable-5","usage":{"input_tokens":97}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1120,"iterations":[{"model":"claude-fable-5","type":"message","input_tokens":97,"output_tokens":0},{"model":"claude-opus-4-8","type":"fallback_message","input_tokens":97,"output_tokens":1120}]}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(
		s,
		`"choices":[]`,
		`"usage"`,
		`"iterations"`,
		`"model":"claude-fable-5"`,
		`"type":"fallback_message"`,
		`"model":"claude-opus-4-8"`,
		`"completion_tokens":1120`,
		`"total_tokens":1217`,
		"data: [DONE]",
	) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE_MergesUsageAcrossEvents(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_usage_2","model":"claude-opus-4-6","usage":{"input_tokens":22}}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","usage":{"cache_creation_input_tokens":139,"output_tokens":3}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(
		s,
		`"finish_reason":"tool_calls"`,
		`"completion_tokens":10`,
		`"prompt_tokens":161`,
		`"cache_write_tokens":139`,
		`"total_tokens":171`,
		"data: [DONE]",
	) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE_PingIgnored(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_ping_1","model":"claude-sonnet-4-6"}}`,
		"",
		"event: ping",
		`data: {"type":"ping"}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: ping",
		`data: {"type":"ping"}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(s, `"content":"hello"`, `"finish_reason":"stop"`, "data: [DONE]") {
		t.Fatalf("unexpected output: %s", s)
	}
	if strings.Contains(s, `"type":"ping"`) {
		t.Fatalf("ping event must not appear in downstream output: %s", s)
	}
}

func TestTransformClaudeMessagesSSEToOpenAIChatCompletionsSSE_UnknownTypeIgnored(t *testing.T) {
	in := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_unk_1","model":"claude-sonnet-4-6"}}`,
		"",
		`data: {"type":"future_event","some_field":"value"}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	s := out.String()
	if !containsAll(s, `"content":"world"`, "data: [DONE]") {
		t.Fatalf("unexpected output: %s", s)
	}
	if strings.Contains(s, "future_event") {
		t.Fatalf("unknown event must not appear in downstream output: %s", s)
	}
}
