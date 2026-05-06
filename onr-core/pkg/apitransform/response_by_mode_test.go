package apitransform

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
)

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
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	s := string(encoded)
	if !containsAll(s, `"object":"chat.completion"`, `"Hello"`, `"total_tokens":3`) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestTransformNonStreamResponseBody_UnsupportedMode(t *testing.T) {
	in := map[string]any{"id": "x"}
	out, outCT, changed, err := TransformNonStreamResponseBody(200, "unknown_mode", in, "application/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false")
	}
	if out != nil || outCT != "application/json" {
		t.Fatalf("unexpected passthrough result")
	}
}

func TestTransformNonStreamResponseBody_Gzip(t *testing.T) {
	const src = `{
  "id":"resp_123",
  "object":"response",
  "created_at":1700000000,
  "model":"gpt-4o-mini",
  "status":"completed",
  "output":[
    {"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}
  ],
  "usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}
}`
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(src)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	decoded, _, err := DecodeResponseBody(buf.Bytes(), "gzip")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(decoded, &root); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	out, outCT, changed, err := TransformNonStreamResponseBody(200, "openai_responses_to_openai_chat", root, "application/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed || outCT != contentTypeJSON {
		t.Fatalf("unexpected transform flags changed=%v outCT=%q", changed, outCT)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	if !containsAll(string(encoded), `"object":"chat.completion"`, `"Hello"`) {
		t.Fatalf("unexpected output: %s", string(encoded))
	}
}

func TestMapResponseBodyByMode_AnthropicToOpenAIChat(t *testing.T) {
	in := []byte(`{
  "id":"msg_123",
  "model":"claude-sonnet-4-5",
  "stop_reason":"tool_use",
  "content":[
    {"type":"text","text":"hello from claude"},
    {"type":"tool_use","id":"toolu_1","name":"lookup","input":{"city":"Boston"}}
  ]
}`)
	out, outCT, err := MapResponseBodyByMode("anthropic_to_openai_chat", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outCT != contentTypeJSON {
		t.Fatalf("unexpected content type: %s", outCT)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	s := string(encoded)
	if !containsAll(s,
		`"object":"chat.completion"`,
		`"model":"claude-sonnet-4-5"`,
		`"content":"hello from claude"`,
		`"name":"lookup"`,
		`"arguments":"{\"city\":\"Boston\"}"`,
	) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestMapResponseBodyByMode_GeminiToOpenAIChat(t *testing.T) {
	in := []byte(`{
  "modelVersion":"gemini-2.5-pro",
  "candidates":[
    {
      "index":0,
      "finishReason":"STOP",
      "content":{
        "role":"model",
        "parts":[
          {"text":"hello"},
          {"functionCall":{"name":"lookup_weather","args":{"city":"Boston"}}},
          {"inlineData":{"mimeType":"image/png","data":"YWJj"}}
        ]
      }
    }
  ],
  "usageMetadata":{
    "promptTokenCount":11,
    "totalTokenCount":19,
    "thoughtsTokenCount":3
  }
}`)
	out, outCT, err := MapResponseBodyByMode("gemini_to_openai_chat", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outCT != contentTypeJSON {
		t.Fatalf("unexpected content type: %s", outCT)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	s := string(encoded)
	if !containsAll(s,
		`"object":"chat.completion"`,
		`"model":"gemini-2.5-pro"`,
		`"content":"hello"`,
		`"finish_reason":"STOP"`,
		`"name":"lookup_weather"`,
		`"name":"emit_media"`,
		`"prompt_tokens":11`,
		`"completion_tokens":8`,
		`"total_tokens":19`,
		`"reasoning_tokens":3`,
	) {
		t.Fatalf("unexpected output: %s", s)
	}
}
