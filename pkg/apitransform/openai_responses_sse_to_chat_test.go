package apitransform

import (
	"bytes"
	"strings"
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

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_NoEventLineUsesType(t *testing.T) {
	in := "" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hi\"}]}]}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "\"delta\":{\"content\":\"Hi\"}", "data: [DONE]") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_MultilineData(t *testing.T) {
	in := "" +
		"data: {\"type\":\"response.output_text.delta\",\n" +
		"data: \"delta\":\"Hi\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\"}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "\"delta\":{\"content\":\"Hi\"}", "data: [DONE]") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_ToolCallsThenTextFinishesStop(t *testing.T) {
	in := "" +
		"event: response.output_item.added\n" +
		"data: {\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"get_weather\",\"arguments\":\"{}\"}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"delta\":\"Hello\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}]}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "\"tool_calls\"", "\"delta\":{\"content\":\"Hello\"}", "\"finish_reason\":\"stop\"") {
		t.Fatalf("unexpected output: %s", s)
	}
	if strings.Contains(s, "\"finish_reason\":\"tool_calls\"") {
		t.Fatalf("should not finish with tool_calls when text exists: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_ArgsDeltaFromPrefix(t *testing.T) {
	in := "" +
		"event: response.output_item.added\n" +
		"data: {\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"f\",\"arguments\":\"{\\\"a\\\":\"}}\n\n" +
		"event: response.output_item.done\n" +
		"data: {\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"f\",\"arguments\":\"{\\\"a\\\":1}\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\"}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	// First tool call delta may include name; the second should include the tail "1}".
	if !containsAll(s, "\"arguments\":\"{\\\"a\\\":\"", "\"arguments\":\"1}\"") {
		t.Fatalf("unexpected args delta behavior: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_IgnoresMalformedJSON(t *testing.T) {
	in := "" +
		"event: response.output_text.delta\n" +
		"data: {not-json}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"delta\":\"OK\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\"}}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "\"delta\":{\"content\":\"OK\"}", "data: [DONE]") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_FlushOnEOF(t *testing.T) {
	// No trailing blank line after the last event.
	in := "" +
		"event: response.output_text.delta\n" +
		"data: {\"delta\":\"Hi\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\"}}"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if !containsAll(s, "\"delta\":{\"content\":\"Hi\"}", "data: [DONE]") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformOpenAIResponsesSSEToChatCompletionsSSE_DoneShouldBeLastWhenCompletedComesEarly(t *testing.T) {
	in := "" +
		"event: response.completed\n" +
		"data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-4o-mini\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello\"}]}]}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"delta\":\"Hello\"}\n\n"

	var buf bytes.Buffer
	if err := TransformOpenAIResponsesSSEToChatCompletionsSSE(bytes.NewBufferString(in), &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := buf.String()
	if strings.Count(s, "data: [DONE]") != 1 {
		t.Fatalf("DONE should appear exactly once, got output: %s", s)
	}
	deltaPos := strings.Index(s, "\"delta\":{\"content\":\"Hello\"}")
	donePos := strings.LastIndex(s, "data: [DONE]")
	if deltaPos < 0 || donePos < 0 || deltaPos > donePos {
		t.Fatalf("DONE should be emitted after content delta, got output: %s", s)
	}
}
