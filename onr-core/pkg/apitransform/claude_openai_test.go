package apitransform

import "testing"

func TestMapClaudeMessagesToOpenAIChatCompletions_Basic(t *testing.T) {
	in := []byte(`{
  "model":"claude-3-5-sonnet",
  "system":"You are helpful",
  "messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]
}`)
	out, err := MapClaudeMessagesToOpenAIChatCompletions(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	out, err := MapOpenAIChatCompletionsToClaudeMessagesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if !containsAll(s, `"object":"chat.completion"`, `"model":"claude-3-5-sonnet-20240620"`, `"content":"hello"`, `"total_tokens":7`) {
		t.Fatalf("unexpected mapped output: %s", s)
	}
}
