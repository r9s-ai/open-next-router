package apitransform

import "testing"

func TestMapGeminiGenerateContentToOpenAIChatCompletions_Basic(t *testing.T) {
	in := []byte(`{
  "system_instruction":{"parts":[{"text":"sys"}]},
  "contents":[{"role":"user","parts":[{"text":"hi"}]}],
  "generation_config":{"temperature":0.2,"topP":0.9,"maxOutputTokens":128,"candidateCount":1}
}`)
	out, err := MapGeminiGenerateContentToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"role":"system"`, `"content":"sys"`, `"role":"user"`, `"content":"hi"`, `"max_tokens":128`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapGeminiGenerateContentToOpenAIChatCompletions_ModelAndStream(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "stream":true,
  "stream_options":{"include_usage":true},
  "contents":[{"role":"user","parts":[{"text":"hi"}]}],
  "generationConfig":{"maxOutputTokens":32}
}`)
	out, err := MapGeminiGenerateContentToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"model":"gpt-4o-mini"`, `"stream":true`, `"include_usage":true`, `"max_tokens":32`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsToGeminiGenerateContentResponse_Basic(t *testing.T) {
	in := []byte(`{
  "choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
  "usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}
}`)
	out, err := MapOpenAIChatCompletionsToGeminiGenerateContentResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"candidates"`, `"finishReason":"STOP"`, `"parts":[{"text":"hello"}]`, `"usageMetadata"`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsToGeminiGenerateContentRequest_Basic(t *testing.T) {
	in := []byte(`{
  "model":"gemini-2.0-flash",
  "messages":[
    {"role":"system","content":"be concise"},
    {"role":"user","content":"hi"}
  ],
  "max_tokens":32,
  "temperature":0.2,
  "top_p":0.9
}`)
	out, err := MapOpenAIChatCompletionsToGeminiGenerateContentRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"model":"gemini-2.0-flash"`, `"system_instruction"`, `"contents"`, `"maxOutputTokens":32`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapGeminiGenerateContentToOpenAIChatCompletionsResponse_Basic(t *testing.T) {
	in := []byte(`{
  "candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"hello"}]}}],
  "usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":4,"totalTokenCount":7},
  "modelVersion":"gemini-2.0-flash"
}`)
	out, err := MapGeminiGenerateContentToOpenAIChatCompletionsResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"object":"chat.completion"`, `"model":"gemini-2.0-flash"`, `"content":"hello"`, `"total_tokens":7`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}
