package apitransform

import "testing"

func TestSupportsResponseMapMode(t *testing.T) {
	if !SupportsResponseMapMode(" OpenAI_Responses_To_OpenAI_Chat ") {
		t.Fatalf("expected normalized mode to be supported")
	}
	if !SupportsResponseMapMode("openai_to_gemini_generate_content") {
		t.Fatalf("expected alias mode to be supported")
	}
	if SupportsResponseMapMode("unknown_mode") {
		t.Fatalf("expected unknown mode to be unsupported")
	}
}

func TestMapResponseBodyByMode_Unsupported(t *testing.T) {
	_, _, err := MapResponseBodyByMode("unknown_mode", []byte(`{}`))
	if err == nil {
		t.Fatalf("expected unsupported mode error")
	}
	if err.Error() != `unsupported resp_map mode "unknown_mode"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMapResponseBodyByMode_OpenAIResponsesToChat(t *testing.T) {
	in := []byte(`{
  "id":"resp_123",
  "object":"response",
  "created_at":1700000000,
  "model":"gpt-4o-mini",
  "status":"completed",
  "output":[
    {"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}
  ],
  "usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}
}`)
	out, outCT, err := MapResponseBodyByMode(" openai_responses_to_openai_chat ", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outCT != contentTypeJSON {
		t.Fatalf("unexpected content type: %s", outCT)
	}
	s := string(out)
	if !containsAll(s, `"object":"chat.completion"`, `"Hello"`, `"total_tokens":3`) {
		t.Fatalf("unexpected output: %s", s)
	}
}
