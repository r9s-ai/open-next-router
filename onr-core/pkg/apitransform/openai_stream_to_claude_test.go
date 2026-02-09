package apitransform

import "testing"

func TestMapOpenAIChatCompletionsChunkToClaudeEvents_Text(t *testing.T) {
	in := []byte(`{
  "choices":[{"index":0,"delta":{"content":"hello"}}]
}`)
	out, err := MapOpenAIChatCompletionsChunkToClaudeEvents(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"events"`, `"content_block_delta"`, `"text_delta"`, `"hello"`) {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestMapOpenAIChatCompletionsChunkToClaudeEvents_ToolAndFinish(t *testing.T) {
	in := []byte(`{
  "choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]},"finish_reason":"tool_calls"}],
  "usage":{"prompt_tokens":3,"completion_tokens":2}
}`)
	out, err := MapOpenAIChatCompletionsChunkToClaudeEvents(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"content_block_start"`, `"tool_use"`, `"message_delta"`, `"stop_reason":"tool_use"`) {
		t.Fatalf("unexpected output: %s", s)
	}
}
