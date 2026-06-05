package ssecollect

import (
	"context"
	"strings"
	"testing"
)

func TestParseMultilineDataAndTypeFallback(t *testing.T) {
	src := "id: 1\n" +
		"data: {\"type\":\"ping\",\n" +
		"data: \"ok\":true}\n\n" +
		"data: [DONE]\n\n"
	events, err := Parse(context.Background(), strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len=%d want=2", len(events))
	}
	if events[0].Event != "ping" || events[0].ID != "1" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if !events[1].Done {
		t.Fatalf("second event Done=false")
	}
}

func TestCollectOpenAIResponses(t *testing.T) {
	src := `event: response.output_item.done
data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","name":"lookup","arguments":"{}"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","model":"gpt-5-mini"}}

data: [DONE]

`
	got, err := CollectByMode(context.Background(), "openai_responses", strings.NewReader(src), Options{})
	if err != nil {
		t.Fatalf("CollectByMode: %v", err)
	}
	output, _ := got["output"].([]any)
	if len(output) != 2 {
		t.Fatalf("output len=%d want=2", len(output))
	}
	first, _ := output[0].(map[string]any)
	if first["type"] != "message" {
		t.Fatalf("first output=%#v", first)
	}
	if got["id"] != "resp_1" || got["status"] != "completed" {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestCollectAnthropicMessagesToolUseInputJSONDelta(t *testing.T) {
	src := `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[],"stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":":\"SF\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":4}}

event: message_stop
data: {"type":"message_stop"}

`
	got, err := CollectByMode(context.Background(), "anthropic_messages", strings.NewReader(src), Options{})
	if err != nil {
		t.Fatalf("CollectByMode: %v", err)
	}
	content, _ := got["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content len=%d want=1", len(content))
	}
	block, _ := content[0].(map[string]any)
	input, _ := block["input"].(map[string]any)
	if input["city"] != "SF" {
		t.Fatalf("input=%#v", input)
	}
	if got["stop_reason"] != "tool_use" {
		t.Fatalf("stop_reason=%#v", got["stop_reason"])
	}
}

func TestCollectGeminiGenerateContent(t *testing.T) {
	src := `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Hel"}]}}],"usageMetadata":{"promptTokenCount":1}}

data: {"candidates":[{"index":0,"content":{"parts":[{"text":"lo"}]},"finishReason":"STOP"}],"usageMetadata":{"totalTokenCount":3}}

`
	got, err := CollectByMode(context.Background(), "gemini_generate_content", strings.NewReader(src), Options{})
	if err != nil {
		t.Fatalf("CollectByMode: %v", err)
	}
	candidates, _ := got["candidates"].([]any)
	c0, _ := candidates[0].(map[string]any)
	content, _ := c0["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	p0, _ := parts[0].(map[string]any)
	if p0["text"] != "Hello" || c0["finishReason"] != "STOP" {
		t.Fatalf("candidate=%#v", c0)
	}
	usage, _ := got["usageMetadata"].(map[string]any)
	if int(usage["totalTokenCount"].(float64)) != 3 {
		t.Fatalf("usage=%#v", usage)
	}
}
