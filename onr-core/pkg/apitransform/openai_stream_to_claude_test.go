package apitransform

import (
	"encoding/json"
	"testing"
)

func TestMapOpenAIChatCompletionsChunkToClaudeEvents_Text(t *testing.T) {
	in := []byte(`{
  "choices":[{"index":0,"delta":{"content":"hello"}}]
}`)
	var obj map[string]any
	if err := json.Unmarshal(in, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	events, err := MapOpenAIChatCompletionsChunkToClaudeEventsObject(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
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
	var obj map[string]any
	if err := json.Unmarshal(in, &obj); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	events, err := MapOpenAIChatCompletionsChunkToClaudeEventsObject(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"content_block_start"`, `"tool_use"`, `"message_delta"`, `"stop_reason":"tool_use"`) {
		t.Fatalf("unexpected output: %s", s)
	}
}
