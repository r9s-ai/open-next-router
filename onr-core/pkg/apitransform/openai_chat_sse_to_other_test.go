package apitransform

import (
	"bytes"
	"strings"
	"testing"
)

func TestTransformOpenAIChatCompletionsSSEToGeminiSSE(t *testing.T) {
	in := "" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: [DONE]\n\n"
	var out bytes.Buffer
	if err := TransformOpenAIChatCompletionsSSEToGeminiSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !containsAll(s, "data: {", "\"candidates\"", "\"text\":\"hi\"") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformOpenAIChatCompletionsSSEToClaudeMessagesSSE(t *testing.T) {
	in := "" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: [DONE]\n\n"
	var out bytes.Buffer
	if err := TransformOpenAIChatCompletionsSSEToClaudeMessagesSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !containsAll(s, "data: {", "\"content_block_delta\"", "\"text_delta\"", "\"hello\"") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformOpenAIChatCompletionsSSEToClaudeMessagesSSE_IgnoresUsageOnlyTerminalChunk(t *testing.T) {
	in := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}],"usage":null}`,
		"",
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`,
		"",
		`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":1,"total_tokens":13}}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")
	var out bytes.Buffer
	if err := TransformOpenAIChatCompletionsSSEToClaudeMessagesSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !containsAll(s, "\"content_block_delta\"", "\"hello\"", "\"message_delta\"", "\"end_turn\"") {
		t.Fatalf("unexpected output: %s", s)
	}
}
