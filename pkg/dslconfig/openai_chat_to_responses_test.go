package dslconfig

import "testing"

func TestMapOpenAIChatCompletionsToResponsesRequest_BasicText(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[
    {"role":"system","content":"You are a helpful assistant."},
    {"role":"user","content":"Hi"},
    {"role":"assistant","content":"Hello"}
  ],
  "max_tokens":123,
  "stream":false
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"model":"gpt-4o-mini"`, `"instructions":"You are a helpful assistant."`, `"max_output_tokens":123`) {
		t.Fatalf("unexpected mapping: %s", s)
	}
	if !containsAll(s, `"input":[`, `"role":"user"`, `"content":"Hi"`) {
		t.Fatalf("missing input mapping: %s", s)
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_ToolOutput(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[
    {"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"{\"temp\":20}"}
  ]
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !containsAll(s, `"type":"function_call"`, `"call_id":"call_1"`, `"name":"get_weather"`) {
		t.Fatalf("missing function_call mapping: %s", s)
	}
	if !containsAll(s, `"type":"function_call_output"`, `"call_id":"call_1"`, `"output":"{\"temp\":20}"`) {
		t.Fatalf("missing function_call_output mapping: %s", s)
	}
}
