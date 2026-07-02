package ssecollect

import (
	"errors"
	"testing"
)

func TestChunkCollectorAnthropicMessagesTextAndToolUse(t *testing.T) {
	c, err := NewChunkCollector("anthropic_messages")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`),
		[]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi "}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"there"}}`),
		[]byte(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`),
		[]byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"city\""}}`),
		[]byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":":\"SF\"}"}}`),
		[]byte(`{"type":"content_block_stop","index":1}`),
		[]byte(`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":8}}`),
		[]byte(`{"type":"message_stop"}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	content, _ := got["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content len=%d want=2", len(content))
	}
	textBlock, _ := content[0].(map[string]any)
	if textBlock["text"] != "Hi there" {
		t.Fatalf("text block=%#v", textBlock)
	}
	toolBlock, _ := content[1].(map[string]any)
	input, _ := toolBlock["input"].(map[string]any)
	if input["city"] != "SF" || got["stop_reason"] != "tool_use" {
		t.Fatalf("snapshot=%#v", got)
	}
	usage, _ := got["usage"].(map[string]any)
	if int(usage["output_tokens"].(float64)) != 8 {
		t.Fatalf("usage=%#v", usage)
	}
}

func TestChunkCollectorOpenAIChatCompletionsTextToolAndUsage(t *testing.T) {
	c, err := NewChunkCollector("openai_chat_completions")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"id":"chatcmpl_1","object":"chat.completion.chunk","created":1,"model":"gpt","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"},"finish_reason":null}]}`),
		[]byte(`{"choices":[{"index":0,"delta":{"content":"lo","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"city\""}}]}}]}`),
		[]byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"SF\"}"}}]},"finish_reason":"tool_calls"}]}`),
		[]byte(`{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`),
		[]byte(`[DONE]`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	choices, _ := got["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	msg, _ := choice["message"].(map[string]any)
	if msg["content"] != "Hello" || choice["finish_reason"] != "tool_calls" {
		t.Fatalf("choice=%#v", choice)
	}
	toolCalls, _ := msg["tool_calls"].([]any)
	call, _ := toolCalls[0].(map[string]any)
	fn, _ := call["function"].(map[string]any)
	if fn["name"] != "lookup" || fn["arguments"] != `{"city":"SF"}` {
		t.Fatalf("tool call=%#v", call)
	}
	usage, _ := got["usage"].(map[string]any)
	if int(usage["total_tokens"].(float64)) != 7 {
		t.Fatalf("usage=%#v", usage)
	}
}

func TestChunkCollectorOpenAIChatCompletionsRefusalAndFunctionCall(t *testing.T) {
	c, err := NewChunkCollector("openai_chat_completions")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"choices":[{"index":0,"delta":{"role":"assistant","refusal":"No"},"finish_reason":null}]}`),
		[]byte(`{"choices":[{"index":0,"delta":{"refusal":" thanks"},"finish_reason":"content_filter"}]}`),
		[]byte(`{"choices":[{"index":1,"delta":{"role":"assistant","function_call":{"name":"lookup","arguments":"{\"city\""}},"finish_reason":null}]}`),
		[]byte(`{"choices":[{"index":1,"delta":{"function_call":{"arguments":":\"SF\"}"}},"finish_reason":"function_call"}]}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	choices, _ := got["choices"].([]any)
	refusalChoice, _ := choices[0].(map[string]any)
	refusalMsg, _ := refusalChoice["message"].(map[string]any)
	if refusalMsg["refusal"] != "No thanks" || refusalChoice["finish_reason"] != "content_filter" {
		t.Fatalf("refusal choice=%#v", refusalChoice)
	}
	functionChoice, _ := choices[1].(map[string]any)
	functionMsg, _ := functionChoice["message"].(map[string]any)
	functionCall, _ := functionMsg["function_call"].(map[string]any)
	if functionCall["name"] != "lookup" || functionCall["arguments"] != `{"city":"SF"}` || functionChoice["finish_reason"] != "function_call" {
		t.Fatalf("function choice=%#v", functionChoice)
	}
}

func TestChunkCollectorOpenAIChatCompletionsMultipleChoices(t *testing.T) {
	c, err := NewChunkCollector("openai_chat_completions")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"id":"chatcmpl_multi","choices":[{"index":1,"delta":{"role":"assistant","content":"B"}},{"index":0,"delta":{"role":"assistant","content":"A"}}]}`),
		[]byte(`{"choices":[{"index":1,"delta":{"content":"2"},"finish_reason":"stop"},{"index":0,"delta":{"content":"1"},"finish_reason":"length"}]}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	choices, _ := got["choices"].([]any)
	if len(choices) != 2 {
		t.Fatalf("choices len=%d want=2", len(choices))
	}
	first, _ := choices[0].(map[string]any)
	firstMsg, _ := first["message"].(map[string]any)
	second, _ := choices[1].(map[string]any)
	secondMsg, _ := second["message"].(map[string]any)
	if first["index"] != 0 || firstMsg["content"] != "A1" || first["finish_reason"] != "length" {
		t.Fatalf("first choice=%#v", first)
	}
	if second["index"] != 1 || secondMsg["content"] != "B2" || second["finish_reason"] != "stop" {
		t.Fatalf("second choice=%#v", second)
	}
}

func TestChunkCollectorOpenAIResponsesTextFunctionAndCompleted(t *testing.T) {
	c, err := NewChunkCollector("openai_responses")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress","model":"gpt"}}`),
		[]byte(`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"Hi"}`),
		[]byte(`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"city\""}`),
		[]byte(`{"type":"response.function_call_arguments.delta","output_index":1,"delta":":\"SF\"}"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	if got["status"] != "completed" {
		t.Fatalf("status=%#v", got["status"])
	}
	output, _ := got["output"].([]any)
	if len(output) != 2 {
		t.Fatalf("output len=%d want=2", len(output))
	}
	msg, _ := output[0].(map[string]any)
	content, _ := msg["content"].([]any)
	text, _ := content[0].(map[string]any)
	if text["text"] != "Hi" {
		t.Fatalf("message output=%#v", msg)
	}
	call, _ := output[1].(map[string]any)
	if call["arguments"] != `{"city":"SF"}` {
		t.Fatalf("function output=%#v", call)
	}
}

func TestChunkCollectorOpenAIResponsesOfficialToolAndContentEvents(t *testing.T) {
	c, err := NewChunkCollector("openai_responses")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress"}}`),
		[]byte(`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"Hello"}`),
		[]byte(`{"type":"response.output_text.annotation.added","output_index":0,"content_index":0,"annotation_index":0,"annotation":{"type":"url_citation","url":"https://example.com"}}`),
		[]byte(`{"type":"response.refusal.delta","output_index":0,"content_index":1,"delta":"No"}`),
		[]byte(`{"type":"response.refusal.done","output_index":0,"content_index":1,"refusal":"No thanks"}`),
		[]byte(`{"type":"response.custom_tool_call_input.delta","output_index":1,"delta":"*** Begin"}`),
		[]byte(`{"type":"response.custom_tool_call_input.delta","output_index":1,"delta":" Patch"}`),
		[]byte(`{"type":"response.custom_tool_call_input.done","output_index":1,"input":"*** Begin Patch"}`),
		[]byte(`{"type":"response.mcp_call_arguments.delta","output_index":2,"delta":"{\"city\""}`),
		[]byte(`{"type":"response.mcp_call_arguments.done","output_index":2,"arguments":"{\"city\":\"SF\"}"}`),
		[]byte(`{"type":"response.code_interpreter_call_code.delta","output_index":3,"delta":"print"}`),
		[]byte(`{"type":"response.code_interpreter_call_code.done","output_index":3,"code":"print(1)"}`),
		[]byte(`{"type":"response.audio.delta","output_index":0,"content_index":2,"delta":"AAAA"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed"}}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	output, _ := got["output"].([]any)
	if len(output) != 4 {
		t.Fatalf("output len=%d want=4: %#v", len(output), output)
	}
	msg, _ := output[0].(map[string]any)
	content, _ := msg["content"].([]any)
	text, _ := content[0].(map[string]any)
	annotations, _ := text["annotations"].([]any)
	refusal, _ := content[1].(map[string]any)
	if text["text"] != "Hello" || len(annotations) != 1 || refusal["refusal"] != "No thanks" {
		t.Fatalf("message=%#v", msg)
	}
	custom, _ := output[1].(map[string]any)
	if custom["type"] != "custom_tool_call" || custom["input"] != "*** Begin Patch" {
		t.Fatalf("custom tool=%#v", custom)
	}
	mcp, _ := output[2].(map[string]any)
	if mcp["type"] != "mcp_call" || mcp["arguments"] != `{"city":"SF"}` {
		t.Fatalf("mcp call=%#v", mcp)
	}
	code, _ := output[3].(map[string]any)
	if code["type"] != "code_interpreter_call" || code["code"] != "print(1)" {
		t.Fatalf("code call=%#v", code)
	}
}

func TestChunkCollectorOpenAIResponsesContentPartEvents(t *testing.T) {
	c, err := NewChunkCollector("openai_responses")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`),
		[]byte(`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"Hi"}`),
		[]byte(`{"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":"output_text","text":"Hi!"}}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}
	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	output, _ := got["output"].([]any)
	msg, _ := output[0].(map[string]any)
	content, _ := msg["content"].([]any)
	part, _ := content[0].(map[string]any)
	if part["text"] != "Hi!" {
		t.Fatalf("part=%#v", part)
	}
}

func TestChunkCollectorOpenAIResponsesReasoningDeltas(t *testing.T) {
	c, err := NewChunkCollector("openai_responses")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"response.reasoning_text.delta","output_index":0,"delta":"I should "}`),
		[]byte(`{"type":"response.reasoning_text.delta","output_index":0,"delta":"check"}`),
		[]byte(`{"type":"response.reasoning_summary_part.added","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":""}}`),
		[]byte(`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"Need "}`),
		[]byte(`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"weather"}`),
		[]byte(`{"type":"response.reasoning_summary_text.done","output_index":0,"summary_index":0,"text":"Need weather."}`),
		[]byte(`{"type":"response.reasoning_text.done","output_index":0,"text":"I should check."}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	output, _ := got["output"].([]any)
	if len(output) != 1 {
		t.Fatalf("output len=%d want=1", len(output))
	}
	reasoning, _ := output[0].(map[string]any)
	if reasoning["type"] != "reasoning" || reasoning["text"] != "I should check." {
		t.Fatalf("reasoning item=%#v", reasoning)
	}
	summary, _ := reasoning["summary"].([]any)
	part, _ := summary[0].(map[string]any)
	if part["type"] != "summary_text" || part["text"] != "Need weather." {
		t.Fatalf("summary part=%#v", part)
	}
}

func TestChunkCollectorAnthropicMessagesThinkingAndSignature(t *testing.T) {
	c, err := NewChunkCollector("anthropic_messages")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"message_start","message":{"id":"msg_thinking","type":"message","role":"assistant","content":[]}}`),
		[]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"I should "}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"check"}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig_123"}}`),
		[]byte(`{"type":"content_block_stop","index":0}`),
		[]byte(`{"type":"message_stop"}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	content, _ := got["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content len=%d want=1", len(content))
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "thinking" || block["thinking"] != "I should check" || block["signature"] != "sig_123" {
		t.Fatalf("thinking block=%#v", block)
	}
}

func TestChunkCollectorAnthropicMessagesRedactedThinking(t *testing.T) {
	c, err := NewChunkCollector("anthropic_messages")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"message_start","message":{"id":"msg_redacted","type":"message","role":"assistant","content":[]}}`),
		[]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"redacted_thinking","data":"encrypted-thinking-data"}}`),
		[]byte(`{"type":"content_block_stop","index":0}`),
		[]byte(`{"type":"message_stop"}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	content, _ := got["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content len=%d want=1", len(content))
	}
	block, _ := content[0].(map[string]any)
	if block["type"] != "redacted_thinking" || block["data"] != "encrypted-thinking-data" {
		t.Fatalf("redacted thinking block=%#v", block)
	}
}

func TestChunkCollectorAnthropicMessagesUnknownEventsDoNotFail(t *testing.T) {
	c, err := NewChunkCollector("anthropic_messages")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`),
		[]byte(`{"type":"unknown_event","field":"value"}`),
		[]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"unknown_delta","custom":"value"}}`),
		[]byte(`{"type":"message_stop"}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}
	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	content, _ := got["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["custom"] != "value" {
		t.Fatalf("block=%#v", block)
	}
}

func TestChunkCollectorGeminiGenerateContent(t *testing.T) {
	c, err := NewChunkCollector("gemini_generate_content")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Hel"}]}}],"usageMetadata":{"promptTokenCount":1}}`),
		[]byte(`{"candidates":[{"index":0,"content":{"parts":[{"text":"lo"},{"functionCall":{"name":"lookup","args":{"city":"SF"}}}]},"finishReason":"STOP"}],"usageMetadata":{"totalTokenCount":3}}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%s): %v", payload, err)
		}
	}

	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	candidates, _ := got["candidates"].([]any)
	c0, _ := candidates[0].(map[string]any)
	content, _ := c0["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	p0, _ := parts[0].(map[string]any)
	if p0["text"] != "Hello" || c0["finishReason"] != "STOP" {
		t.Fatalf("candidate=%#v", c0)
	}
	if len(parts) != 2 {
		t.Fatalf("parts len=%d want=2", len(parts))
	}
	usage, _ := got["usageMetadata"].(map[string]any)
	if int(usage["totalTokenCount"].(float64)) != 3 {
		t.Fatalf("usage=%#v", usage)
	}
}

func TestChunkCollectorCommonErrors(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		payload []byte
	}{
		{name: "invalid json", mode: "anthropic_messages", payload: []byte(`{`)},
		{name: "top array", mode: "anthropic_messages", payload: []byte(`[]`)},
		{name: "upstream error", mode: "openai_responses", payload: []byte(`{"type":"error","error":{"message":"bad"}}`)},
		{name: "anthropic tool delta before start", mode: "anthropic_messages", payload: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`)},
		{name: "chat content wrong type", mode: "openai_chat_completions", payload: []byte(`{"choices":[{"index":0,"delta":{"content":123}}]}`)},
		{name: "gemini candidates wrong type", mode: "gemini_generate_content", payload: []byte(`{"candidates":{}}`)},
		{name: "responses sparse content index", mode: "openai_responses", payload: []byte(`{"type":"response.output_text.delta","output_index":0,"content_index":101,"delta":"x"}`)},
		{name: "anthropic sparse block index", mode: "anthropic_messages", payload: []byte(`{"type":"content_block_start","index":101,"content_block":{"type":"text","text":""}}`)},
		{name: "gemini sparse candidate index", mode: "gemini_generate_content", payload: []byte(`{"candidates":[{"index":101,"content":{"parts":[{"text":"x"}]}}]}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewChunkCollector(tt.mode)
			if err != nil {
				t.Fatalf("NewChunkCollector: %v", err)
			}
			if err := c.AddEvent(tt.payload); err == nil {
				t.Fatalf("expected AddEvent error")
			}
		})
	}
}

func TestChunkCollectorSoftChunksDoNotFail(t *testing.T) {
	c, err := NewChunkCollector("openai_chat_completions")
	if err != nil {
		t.Fatalf("NewChunkCollector: %v", err)
	}
	for _, payload := range [][]byte{
		[]byte(``),
		[]byte(`[DONE]`),
		[]byte(`{"choices":[],"usage":{"total_tokens":3}}`),
	} {
		if err := c.AddEvent(payload); err != nil {
			t.Fatalf("AddEvent(%q): %v", payload, err)
		}
	}
	got, ok := c.Snapshot()
	if !ok {
		t.Fatalf("Snapshot ok=false")
	}
	usage, _ := got["usage"].(map[string]any)
	if int(usage["total_tokens"].(float64)) != 3 {
		t.Fatalf("usage=%#v", usage)
	}
}

func TestNewChunkCollectorUnsupportedMode(t *testing.T) {
	_, err := NewChunkCollector("missing")
	if !errors.Is(err, ErrUnsupportedMode) {
		t.Fatalf("err=%v want ErrUnsupportedMode", err)
	}
}
