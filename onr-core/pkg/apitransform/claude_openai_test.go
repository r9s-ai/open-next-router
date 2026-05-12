package apitransform

import (
	"encoding/json"
	"testing"
)

func TestMapClaudeMessagesToOpenAIChatCompletions_Basic(t *testing.T) {
	in := []byte(`{
  "model":"claude-3-5-sonnet",
  "system":"You are helpful",
  "messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]
}`)
	var obj map[string]any
	if err := json.Unmarshal(in, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	outObj, err := MapClaudeMessagesToOpenAIChatCompletionsObject(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := json.Marshal(outObj)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"model":"claude-3-5-sonnet"`, `"role":"system"`, `"You are helpful"`, `"role":"user"`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsToClaudeMessagesResponse_Basic(t *testing.T) {
	in := []byte(`{
  "id":"chatcmpl_x",
  "model":"gpt-4o-mini",
  "choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
  "usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}
}`)
	out, err := MapOpenAIChatCompletionsToClaudeMessagesResponse(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"type":"message"`, `"role":"assistant"`, `"content":[{"text":"hello","type":"text"}]`, `"stop_reason":"end_turn"`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsToClaudeMessagesRequest_Basic(t *testing.T) {
	in := []byte(`{
  "model":"claude-3-5-sonnet-20240620",
  "messages":[{"role":"system","content":"be concise"},{"role":"user","content":"hi"}],
  "max_tokens":128
}`)
	var obj map[string]any
	if err := json.Unmarshal(in, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	outObj, err := MapOpenAIChatCompletionsToClaudeMessagesRequestObject(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := json.Marshal(outObj)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"model":"claude-3-5-sonnet-20240620"`, `"system":"be concise"`, `"messages"`, `"max_tokens":128`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapClaudeMessagesResponseToOpenAIChatCompletions_Basic(t *testing.T) {
	in := []byte(`{
  "id":"msg_123",
  "type":"message",
  "role":"assistant",
  "model":"claude-3-5-sonnet-20240620",
  "content":[{"type":"text","text":"hello"}],
  "stop_reason":"end_turn",
  "usage":{"input_tokens":3,"output_tokens":4}
}`)
	out, err := MapClaudeMessagesResponseToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"object":"chat.completion"`, `"model":"claude-3-5-sonnet-20240620"`, `"content":"hello"`, `"finish_reason":"stop"`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}

func TestMapClaudeMessagesResponseToOpenAIChatCompletions_PreservesCacheUsage(t *testing.T) {
	in := []byte(`{
  "id":"msg_123",
  "type":"message",
  "role":"assistant",
  "model":"claude-3-5-sonnet-20240620",
  "content":[{"type":"text","text":"hello"}],
  "stop_reason":"end_turn",
  "usage":{"input_tokens":10,"output_tokens":4,"cache_creation_input_tokens":7,"cache_read_input_tokens":2}
}`)
	out, err := MapClaudeMessagesResponseToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	usage, ok := obj["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage must be object, got %#v", obj["usage"])
	}
	if got, want := usage["prompt_tokens"], float64(19); got != want {
		t.Fatalf("prompt_tokens=%#v want %#v", got, want)
	}
	if got, want := usage["total_tokens"], float64(23); got != want {
		t.Fatalf("total_tokens=%#v want %#v", got, want)
	}
	promptDetails, ok := usage["prompt_tokens_details"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_tokens_details must be object, got %#v", usage["prompt_tokens_details"])
	}
	if got, want := promptDetails["cached_tokens"], float64(2); got != want {
		t.Fatalf("cached_tokens=%#v want %#v", got, want)
	}
	if got, want := promptDetails["cache_write_tokens"], float64(7); got != want {
		t.Fatalf("cache_write_tokens=%#v want %#v", got, want)
	}
}

func TestMapClaudeMessagesResponseToOpenAIChatCompletions_SingleChoiceAggregatesContentAndToolCalls(t *testing.T) {
	in := []byte(`{
  "id":"msg_123",
  "type":"message",
  "role":"assistant",
  "model":"claude-3-5-sonnet-20240620",
  "content":[
    {"type":"text","text":"hello "},
    {"type":"thinking","thinking":"internal"},
    {"type":"text","text":"world"},
    {"type":"tool_use","id":"tool_1","name":"search","input":{"q":"x"}}
  ],
  "stop_reason":"tool_use",
  "usage":{"input_tokens":3,"output_tokens":4}
}`)
	out, err := MapClaudeMessagesResponseToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	choices, ok := obj["choices"].([]any)
	if !ok || len(choices) != 1 {
		t.Fatalf("choices must contain a single item, got %#v", obj["choices"])
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		t.Fatalf("choice must be object, got %#v", choices[0])
	}
	msg, ok := choice["message"].(map[string]any)
	if !ok {
		t.Fatalf("choice.message must be object, got %#v", choice["message"])
	}
	if msg["content"] != "hello internalworld" {
		t.Fatalf("unexpected aggregated content: %#v", msg["content"])
	}
	toolCalls, ok := msg["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("tool_calls must contain one item, got %#v", msg["tool_calls"])
	}
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("unexpected finish_reason: %#v", choice["finish_reason"])
	}
}

func TestMapClaudeMessagesResponseToOpenAIChatCompletions_FinishReasonMaxTokensToLength(t *testing.T) {
	in := []byte(`{
  "id":"msg_123",
  "type":"message",
  "role":"assistant",
  "model":"claude-3-5-sonnet-20240620",
  "content":[{"type":"text","text":"hello"}],
  "stop_reason":"max_tokens",
  "usage":{"input_tokens":3,"output_tokens":4}
}`)
	out, err := MapClaudeMessagesResponseToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	choices, ok := obj["choices"].([]any)
	if !ok || len(choices) != 1 {
		t.Fatalf("choices must contain a single item, got %#v", obj["choices"])
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		t.Fatalf("choice must be object, got %#v", choices[0])
	}
	if choice["finish_reason"] != "length" {
		t.Fatalf("unexpected finish_reason: %#v", choice["finish_reason"])
	}
}
