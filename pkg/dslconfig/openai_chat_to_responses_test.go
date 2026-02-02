package dslconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

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
	m := mustUnmarshalObj(t, out)
	if m["model"] != "gpt-4o-mini" {
		t.Fatalf("unexpected model: %#v", m["model"])
	}
	if m["instructions"] != "You are a helpful assistant." {
		t.Fatalf("unexpected instructions: %#v", m["instructions"])
	}
	if intFromAny(m["max_output_tokens"]) != 123 {
		t.Fatalf("unexpected max_output_tokens: %#v", m["max_output_tokens"])
	}
	input := mustAnySlice(t, m["input"])
	if len(input) < 2 {
		t.Fatalf("unexpected input len: %d", len(input))
	}
	user := mustAnyMap(t, input[0])
	if user["role"] != "user" || user["content"] != "Hi" {
		t.Fatalf("unexpected first input item: %#v", user)
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
	m := mustUnmarshalObj(t, out)
	input := mustAnySlice(t, m["input"])
	if len(input) != 3 {
		t.Fatalf("expected 3 input items (assistant + function_call + function_call_output), got %d", len(input))
	}
	fc := mustAnyMap(t, input[1])
	if fc["type"] != "function_call" || fc["call_id"] != "call_1" || fc["name"] != "get_weather" {
		t.Fatalf("unexpected function_call item: %#v", fc)
	}
	fco := mustAnyMap(t, input[2])
	if fco["type"] != "function_call_output" || fco["call_id"] != "call_1" {
		t.Fatalf("unexpected function_call_output item: %#v", fco)
	}
	if strings.TrimSpace(coerceString(fco["output"])) != `{"temp":20}` {
		t.Fatalf("unexpected output: %#v", fco["output"])
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_PassthroughWhenAlreadyResponses(t *testing.T) {
	in := []byte(`{"model":"gpt-4o-mini","input":[{"role":"user","content":"hi"}]}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != strings.TrimSpace(string(in)) {
		t.Fatalf("expected passthrough, got: %s", string(out))
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_NGreaterThan1Rejected(t *testing.T) {
	in := []byte(`{"model":"gpt-4o-mini","n":2,"messages":[{"role":"user","content":"hi"}]}`)
	_, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_InstructionsSystemAndDeveloper(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[
    {"role":"system","content":"S1"},
    {"role":"developer","content":[{"type":"text","text":"D1"}]},
    {"role":"user","content":"U1"}
  ]
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := mustUnmarshalObj(t, out)
	if m["instructions"] != "S1\n\nD1" {
		t.Fatalf("unexpected instructions: %#v", m["instructions"])
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_MultimodalParts(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[
    {"role":"user","content":[
      {"type":"text","text":"hi"},
      {"type":"image_url","image_url":{"url":"https://example.com/a.png"}}
    ]}
  ]
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := mustUnmarshalObj(t, out)
	input := mustAnySlice(t, m["input"])
	user := mustAnyMap(t, input[0])
	parts := mustAnySlice(t, user["content"])
	p0 := mustAnyMap(t, parts[0])
	if p0["type"] != "input_text" || p0["text"] != "hi" {
		t.Fatalf("unexpected part0: %#v", p0)
	}
	p1 := mustAnyMap(t, parts[1])
	if p1["type"] != "input_image" || p1["image_url"] != "https://example.com/a.png" {
		t.Fatalf("unexpected part1: %#v", p1)
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_ToolChoiceAndTools(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[{"role":"user","content":"hi"}],
  "tools":[{"type":"function","function":{"name":"get_weather","description":"d","parameters":{"type":"object"}}}],
  "tool_choice":{"type":"function","function":{"name":"get_weather"}},
  "parallel_tool_calls":true
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := mustUnmarshalObj(t, out)
	tc := mustAnyMap(t, m["tool_choice"])
	if tc["type"] != "function" || tc["name"] != "get_weather" {
		t.Fatalf("unexpected tool_choice: %#v", tc)
	}
	tools := mustAnySlice(t, m["tools"])
	t0 := mustAnyMap(t, tools[0])
	if t0["type"] != "function" || t0["name"] != "get_weather" {
		t.Fatalf("unexpected tools[0]: %#v", t0)
	}
	if m["parallel_tool_calls"] != true {
		t.Fatalf("unexpected parallel_tool_calls: %#v", m["parallel_tool_calls"])
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_MaxOutputTokensPrefersMaxCompletion(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[{"role":"user","content":"hi"}],
  "max_tokens":10,
  "max_completion_tokens":20
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := mustUnmarshalObj(t, out)
	if intFromAny(m["max_output_tokens"]) != 20 {
		t.Fatalf("unexpected max_output_tokens: %#v", m["max_output_tokens"])
	}
}

func TestMapOpenAIChatCompletionsToResponsesRequest_ResponseFormatAndReasoning(t *testing.T) {
	in := []byte(`{
  "model":"gpt-4o-mini",
  "messages":[{"role":"user","content":"hi"}],
  "response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object"}}},
  "reasoning_effort":"low"
}`)
	out, err := MapOpenAIChatCompletionsToResponsesRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := mustUnmarshalObj(t, out)
	text := mustAnyMap(t, m["text"])
	if text["format"] == nil {
		t.Fatalf("missing text.format: %#v", text)
	}
	reasoning := mustAnyMap(t, m["reasoning"])
	if reasoning["effort"] != "low" {
		t.Fatalf("unexpected reasoning: %#v", reasoning)
	}
}

func mustUnmarshalObj(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var obj any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	m, _ := obj.(map[string]any)
	if m == nil {
		t.Fatalf("expected object")
	}
	return m
}

func mustAnySlice(t *testing.T, v any) []any {
	t.Helper()
	a, _ := v.([]any)
	if a == nil {
		t.Fatalf("expected array, got %T", v)
	}
	return a
}

func mustAnyMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, _ := v.(map[string]any)
	if m == nil {
		t.Fatalf("expected object, got %T", v)
	}
	return m
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case float64:
		return int(t)
	default:
		return 0
	}
}
