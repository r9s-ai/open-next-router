package apitransform

import (
	"bytes"
	"strings"
	"testing"
)

func TestTransformGeminiSSEToOpenAIChatCompletionsSSE_Basic(t *testing.T) {
	in := "" +
		"data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hi\"}]}}],\"modelVersion\":\"gemini-2.0-flash\"}\n\n" +
		"data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"!\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":2,\"totalTokenCount\":3}}\n\n"

	var out bytes.Buffer
	if err := TransformGeminiSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !containsAll(s, `"chat.completion.chunk"`, `"role":"assistant"`, `"content":"Hi"`, `"finish_reason":"STOP"`) {
		t.Fatalf("unexpected output: %s", s)
	}
	if !containsAll(s, `"usage"`, `"prompt_tokens":1`, `"completion_tokens":2`, `"total_tokens":3`) {
		t.Fatalf("missing usage payload: %s", s)
	}
	if strings.Count(s, "data: [DONE]") != 1 {
		t.Fatalf("expected one DONE, got: %s", s)
	}
}

// TestTransformGeminiSSEToOpenAIChatCompletionsSSE_IgnoresDONEPayload verifies that a
// "data: [DONE]" line in the Gemini input (matched via sseDonePayload) is skipped and
// not parsed as JSON — the transformer still emits exactly one "data: [DONE]" terminator
// of its own at the end, and no chunk contains "[DONE]" as its JSON body.
func TestTransformGeminiSSEToOpenAIChatCompletionsSSE_IgnoresDONEPayload(t *testing.T) {
	in := "" +
		"data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hi\"}]},\"finishReason\":\"STOP\"}]}\n\n" +
		"data: [DONE]\n\n"

	var out bytes.Buffer
	if err := TransformGeminiSSEToOpenAIChatCompletionsSSE(bytes.NewBufferString(in), &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"content":"Hi"`) {
		t.Fatalf("expected chunk content, got: %s", s)
	}
	// The transformer emits exactly one [DONE] terminator at the end.
	if strings.Count(s, "data: [DONE]") != 1 {
		t.Fatalf("expected exactly one data: [DONE], got: %s", s)
	}
	// No individual chunk body should be the literal string "[DONE]" (would indicate
	// the input [DONE] was passed through as a JSON payload rather than being skipped).
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "data: [DONE]" {
			continue // this is the expected terminal line
		}
		if strings.Contains(line, "[DONE]") {
			t.Fatalf("input [DONE] leaked into non-terminal output line: %q", line)
		}
	}
}
