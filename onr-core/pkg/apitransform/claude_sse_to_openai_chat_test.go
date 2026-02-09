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
