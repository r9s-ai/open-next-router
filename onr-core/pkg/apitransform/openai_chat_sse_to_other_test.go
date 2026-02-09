package apitransform

import (
	"bytes"
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
