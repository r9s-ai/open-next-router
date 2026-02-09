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
	if !containsAll(s, `"chat.completion.chunk"`, `"role":"assistant"`, `"content":"Hi"`, `"finish_reason":"stop"`) {
		t.Fatalf("unexpected output: %s", s)
	}
	if !containsAll(s, `"choices":[]`, `"usage"`, `"prompt_tokens":1`, `"completion_tokens":2`, `"total_tokens":3`) {
		t.Fatalf("missing usage chunk: %s", s)
	}
	if strings.Count(s, "data: [DONE]") != 1 {
		t.Fatalf("expected one DONE, got: %s", s)
	}
}
