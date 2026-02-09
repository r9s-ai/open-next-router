package apitransform

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

func TestMapOpenAIResponsesToChatCompletions_Basic(t *testing.T) {
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
	out, err := MapOpenAIResponsesToChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Best-effort smoke assertions (exact formatting/id prefix isn't important).
	s := string(out)
	if !containsAll(s, `"object":"chat.completion"`, `"model":"gpt-4o-mini"`, `"Hello"`, `"finish_reason":"stop"`) {
		t.Fatalf("unexpected output: %s", s)
	}
	if !containsAll(s, `"usage"`, `"prompt_tokens":1`, `"completion_tokens":2`, `"total_tokens":3`) {
		t.Fatalf("missing usage mapping: %s", s)
	}
}

func TestMapOpenAIResponsesToChatCompletions_FunctionCall(t *testing.T) {
	in := []byte(`{
  "id":"resp_1",
  "created_at":1700000000,
  "model":"gpt-4o-mini",
  "status":"completed",
  "output":[
    {"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"SF\"}"},
    {"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}
  ]
}`)
	out, err := MapOpenAIResponsesToChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"tool_calls"`, `"get_weather"`, `"arguments":"{\"city\":\"SF\"}"`) {
		t.Fatalf("missing tool_calls mapping: %s", s)
	}
}

func TestMapOpenAIResponsesToChatCompletions_OnlyToolCallFinishReason(t *testing.T) {
	in := []byte(`{
  "id":"resp_1",
  "created_at":1700000000,
  "model":"gpt-4o-mini",
  "status":"completed",
  "output":[
    {"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{}"}
  ]
}`)
	out, err := MapOpenAIResponsesToChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"finish_reason":"tool_calls"`, `"tool_calls"`) {
		t.Fatalf("expected tool_calls finish_reason: %s", s)
	}
}

func TestMapOpenAIResponsesToChatCompletionsObject_Basic(t *testing.T) {
	out, err := MapOpenAIResponsesToChatCompletionsObject(apitypes.JSONObject{
		"id":         "resp_1",
		"created_at": 1700000000,
		"model":      "gpt-4o-mini",
		"status":     "completed",
		"output": []any{
			map[string]any{
				"type": "message", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": "hello"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["object"] != "chat.completion" {
		t.Fatalf("unexpected object: %#v", out["object"])
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	// tiny helper to avoid importing strings in tests (keeps it simple)
	n := len(sub)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}
