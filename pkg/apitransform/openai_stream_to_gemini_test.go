package apitransform

import "testing"

func TestMapOpenAIChatCompletionsChunkToGeminiResponse_Text(t *testing.T) {
	in := []byte(`{
  "choices":[{"index":0,"delta":{"content":"hi"}}]
}`)
	out, emit, err := MapOpenAIChatCompletionsChunkToGeminiResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !emit {
		t.Fatalf("expected emit")
	}
	s := string(out)
	if !containsAll(s, `"candidates"`, `"parts":[{"text":"hi"}]`) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsChunkToGeminiResponse_FinishAndUsage(t *testing.T) {
	in := []byte(`{
  "choices":[{"index":0,"delta":{},"finish_reason":"length"}],
  "usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
}`)
	out, emit, err := MapOpenAIChatCompletionsChunkToGeminiResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !emit {
		t.Fatalf("expected emit")
	}
	s := string(out)
	if !containsAll(s, `"finishReason":"MAX_TOKENS"`, `"usageMetadata"`, `"totalTokenCount":7`) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsChunkToGeminiResponse_EmptySkip(t *testing.T) {
	in := []byte(`{"choices":[{"index":0,"delta":{}}]}`)
	_, emit, err := MapOpenAIChatCompletionsChunkToGeminiResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if emit {
		t.Fatalf("expected skip emit")
	}
}

func TestMapOpenAIChatCompletionsChunkToGeminiResponse_UsageOnly(t *testing.T) {
	in := []byte(`{
  "choices":[],
  "usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
}`)
	out, emit, err := MapOpenAIChatCompletionsChunkToGeminiResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !emit {
		t.Fatalf("expected emit")
	}
	s := string(out)
	if !containsAll(s, `"candidates":[]`, `"usageMetadata"`, `"totalTokenCount":7`) {
		t.Fatalf("unexpected output: %s", s)
	}
}
