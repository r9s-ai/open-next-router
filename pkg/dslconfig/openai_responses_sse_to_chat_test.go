package dslconfig

import (
	"bytes"
	"testing"
)

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_Basic(t *testing.T) {
	in := "" +
		"event: response.output_text.delta\n" +
		"data: {\"delta\":\"Hel\"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"delta\":\"lo\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "data: {", "\"chat.completion.chunk\"", "\"delta\":{\"content\":\"Hel\"}", "\"delta\":{\"content\":\"lo\"}") {
		t.Fatalf("unexpected chunks: %s", s)
	}
	if !containsAll(s, "\"finish_reason\":\"stop\"", "data: [DONE]") {
		t.Fatalf("missing terminal chunk/DONE: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_ToolCalls(t *testing.T) {
	in := "" +
		"event: response.output_item.added\n" +
		"data: {\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"SF\\\"}\"}}\n\n" +
		"event: response.function_call_arguments.delta\n" +
		"data: {\"item_id\":\"item_1\",\"delta\":\"\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\",\"output\":[{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"SF\\\"}\"}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "\"tool_calls\"", "\"get_weather\"") {
		t.Fatalf("missing tool_calls delta: %s", s)
	}
	if !containsAll(s, "\"finish_reason\":\"tool_calls\"", "data: [DONE]") {
		t.Fatalf("missing tool_calls finish/DONE: %s", s)
	}
}
