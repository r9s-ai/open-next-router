package streamtext

import (
	"strings"
	"testing"
)

func TestExtractDeltaText(t *testing.T) {
	tests := []struct {
		name    string
		api     string
		payload string
		want    string
	}{
		{
			name:    "chat completions content and reasoning",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"content":"Hi","reasoning_content":" there"}}]}`,
			want:    "Hi there",
		},
		{
			name:    "chat completions tool call name",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
			want:    "get_weather",
		},
		{
			name:    "chat completions tool call arguments",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Beijing\"}"}}]}}]}`,
			want:    `{"city":"Beijing"}`,
		},
		{
			name:    "chat completions compatible object arguments",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":{"city":"Beijing"}}}]}}]}`,
			want:    `{"city":"Beijing"}`,
		},
		{
			name:    "chat completions parallel tool call arguments",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Beijing\"}"}},{"index":1,"function":{"arguments":"{\"city\":\"Shanghai\"}"}}]}}]}`,
			want:    `{"city":"Beijing"}{"city":"Shanghai"}`,
		},
		{
			name:    "chat completions deprecated function call",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"function_call":{"name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"}}}]}`,
			want:    `{"city":"Hangzhou"}`,
		},
		{
			name:    "responses delta",
			api:     "responses",
			payload: `{"delta":"hello"}`,
			want:    "hello",
		},
		{
			name:    "claude delta text",
			api:     "claude.messages",
			payload: `{"delta":{"text":"bonjour"}}`,
			want:    "bonjour",
		},
		{
			name:    "claude tool partial json",
			api:     "claude.messages",
			payload: `{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"city\""}}`,
			want:    `{"city"`,
		},
		{
			name:    "claude tool block name and id",
			api:     "claude.messages",
			payload: `{"type":"content_block_start","content_block":{"type":"tool_use","id":"toolu_123","name":"get_weather","input":{}}}`,
			want:    "get_weather toolu_123",
		},
		{
			name:    "claude tool block id without name",
			api:     "claude.messages",
			payload: `{"type":"content_block_start","content_block":{"type":"tool_use","id":"toolu_123","input":{}}}`,
			want:    "toolu_123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractDeltaText(tc.api, []byte(tc.payload)); got != tc.want {
				t.Fatalf("ExtractDeltaText()=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestExtractFromSSE_ChatCompletions(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"id":"x","choices":[{"delta":{"content":"hel"}}]}`,
		"",
		`data: {"id":"x","choices":[{"delta":{"content":"lo"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	if got := strings.ReplaceAll(ExtractFromSSE("chat.completions", []byte(sse), 1024), "\n", ""); got != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}
func TestExtractDeltaText_AnthropicMessagesSSE(t *testing.T) {
	// Simulate a real Anthropic Messages API SSE stream (captured from claude-haiku-4-5-20251001).
	// Model makes a tool_use call to get_weather with JSON arguments.
	//
	// Each step feeds one SSE data payload into ExtractDeltaText.

	type step struct {
		desc    string
		payload string
		want    string
	}

	steps := []step{
		// --- Lifecycle ---
		{
			desc:    "message_start",
			payload: `{"type":"message_start","message":{"model":"claude-haiku-4-5-20251001","id":"msg_01Nb8D6SmWYQqBTKhwa2zik5","type":"message","role":"assistant","content":[],"stop_reason":null,"stop_sequence":null,"stop_details":null,"usage":{"input_tokens":747,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":0},"output_tokens":12,"service_tier":"standard","inference_geo":"not_available"}}`,
			want:    "",
		},
		{
			desc:    "content_block_start",
			payload: `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_017jmpbA8vZUd3y6kmm3KHbz","name":"get_weather","input":{},"caller":{"type":"direct"}}}`,
			want:    "get_weather toolu_017jmpbA8vZUd3y6kmm3KHbz",
		},
		{
			desc:    "ping",
			payload: `{"type":"ping"}`,
			want:    "",
		},

		// --- Tool input JSON deltas ---
		{
			desc:    "input_json_delta 1",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":""}}`,
			want:    "",
		},
		{
			desc:    "input_json_delta 2",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"locatio"}}`,
			want:    `{"locatio`,
		},
		{
			desc:    "input_json_delta 3",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"n\":"}}`,
			want:    `n":`,
		},
		{
			desc:    "input_json_delta 4",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":" \"Seattl"}}`,
			want:    ` "Seattl`,
		},
		{
			desc:    "input_json_delta 5",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"e, "}}`,
			want:    `e, `,
		},
		{
			desc:    "input_json_delta 6",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"WA\""}}`,
			want:    `WA"`,
		},
		{
			desc:    "input_json_delta 7",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":", \"unit\": "}}`,
			want:    `, "unit": `,
		},
		{
			desc:    "input_json_delta 8",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"fahrenheit"}}`,
			want:    `"fahrenheit`,
		},
		{
			desc:    "input_json_delta 9",
			payload: `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"}"}}`,
			want:    `"}`,
		},

		// --- Stream end ---
		{
			desc:    "content_block_stop",
			payload: `{"type":"content_block_stop","index":0}`,
			want:    "",
		},
		{
			desc:    "message_delta",
			payload: `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null,"stop_details":null},"usage":{"input_tokens":747,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":54}}`,
			want:    "",
		},
		{
			desc:    "message_stop",
			payload: `{"type":"message_stop"}`,
			want:    "",
		},
	}

	var b strings.Builder
	for i, st := range steps {
		t.Run(st.desc, func(t *testing.T) {
			got := ExtractDeltaText("claude.messages", []byte(st.payload))
			b.WriteString(got)
			if got != st.want {
				t.Errorf("%d %s ExtractDeltaText() = %q, want %q", i, st.desc, got, st.want)
			}
		})
	}
	t.Logf("finally get: %s", b.String())
}

func TestExtractDeltaText_OpenAIChatCompletionsSSE(t *testing.T) {
	// Simulate OpenAI Chat Completions SSE chunks. Each step feeds one SSE data
	// payload into ExtractDeltaText.

	type step struct {
		desc    string
		payload string
		want    string
	}

	steps := []step{
		// --- Text response ---
		{
			desc:    "role chunk with empty content",
			payload: `{"id":"chatcmpl_001","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}],"usage":null}`,
			want:    "",
		},
		{
			desc:    "content delta 1",
			payload: `{"id":"chatcmpl_001","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}],"usage":null}`,
			want:    "Hello",
		},
		{
			desc:    "content delta 2",
			payload: `{"id":"chatcmpl_001","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}],"usage":null}`,
			want:    " world",
		},
		{
			desc:    "reasoning_content compatible delta",
			payload: `{"id":"chatcmpl_001","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":"thinking"},"finish_reason":null}],"usage":null}`,
			want:    "thinking",
		},
		{
			desc:    "stop chunk",
			payload: `{"id":"chatcmpl_001","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`,
			want:    "",
		},
		{
			desc:    "include_usage final chunk",
			payload: `{"id":"chatcmpl_001","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
			want:    "",
		},

		// --- Tool calls ---
		{
			desc:    "tool call start with name",
			payload: `{"id":"chatcmpl_002","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_weather","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}],"usage":null}`,
			want:    "get_weather",
		},
		{
			desc:    "tool call arguments delta 1",
			payload: `{"id":"chatcmpl_002","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Bei"}}]},"finish_reason":null}],"usage":null}`,
			want:    `{"city":"Bei`,
		},
		{
			desc:    "tool call arguments delta 2",
			payload: `{"id":"chatcmpl_002","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"jing\"}"}}]},"finish_reason":null}],"usage":null}`,
			want:    `jing"}`,
		},
		{
			desc:    "tool calls finish",
			payload: `{"id":"chatcmpl_002","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":null}`,
			want:    "",
		},
		{
			desc:    "parallel tool calls arguments",
			payload: `{"id":"chatcmpl_003","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Beijing\"}"}},{"index":1,"function":{"arguments":"{\"city\":\"Shanghai\"}"}}]},"finish_reason":null}],"usage":null}`,
			want:    `{"city":"Beijing"}{"city":"Shanghai"}`,
		},
		{
			desc:    "compatible object arguments",
			payload: `{"id":"chatcmpl_004","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":{"city":"Hangzhou"}}}]},"finish_reason":null}],"usage":null}`,
			want:    `{"city":"Hangzhou"}`,
		},

		// --- Deprecated function_call ---
		{
			desc:    "deprecated function call name",
			payload: `{"id":"chatcmpl_old","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","function_call":{"name":"get_weather","arguments":""}},"finish_reason":null}],"usage":null}`,
			want:    "get_weather",
		},
		{
			desc:    "deprecated function call arguments",
			payload: `{"id":"chatcmpl_old","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"function_call":{"arguments":"{\"city\":\"Suzhou\"}"}},"finish_reason":null}],"usage":null}`,
			want:    `{"city":"Suzhou"}`,
		},
		{
			desc:    "deprecated function call finish",
			payload: `{"id":"chatcmpl_old","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"function_call"}],"usage":null}`,
			want:    "",
		},

		// --- Multiple choices / legacy text ---
		{
			desc:    "multiple choices content",
			payload: `{"id":"chatcmpl_multi","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"A"},"finish_reason":null},{"index":1,"delta":{"content":"B"},"finish_reason":null}],"usage":null}`,
			want:    "AB",
		},
		{
			desc:    "legacy text choice",
			payload: `{"id":"cmpl_legacy","choices":[{"index":0,"text":"legacy text","finish_reason":null}]}`,
			want:    "legacy text",
		},
		{
			desc:    "content filter finish",
			payload: `{"id":"chatcmpl_filter","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"content_filter"}],"usage":null}`,
			want:    "",
		},
		{
			desc:    "length finish",
			payload: `{"id":"chatcmpl_length","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"length"}],"usage":null}`,
			want:    "",
		},
	}

	var b strings.Builder
	for i, st := range steps {
		t.Run(st.desc, func(t *testing.T) {
			got := ExtractDeltaText("chat.completions", []byte(st.payload))
			b.WriteString(got)
			if got != st.want {
				t.Errorf("%d %s ExtractDeltaText() = %q, want %q", i, st.desc, got, st.want)
			}
		})
	}
	t.Logf("finally get: %s", b.String())
}

func TestExtractDeltaText_OpenAIResponsesSSE(t *testing.T) {
	// Simulate a real Responses API SSE stream (captured from gpt-4o-mini).
	// Model first outputs reasoning text explaining which tool it will use,
	// then makes a function_call to get_horoscope.
	//
	// Each step feeds one SSE data payload into ExtractDeltaText.

	type step struct {
		desc    string
		payload string
		want    string
	}

	steps := []step{
		// --- Lifecycle ---
		{
			desc:    "response.created",
			payload: `{"type":"response.created","response":{"id":"resp_0831a33f96d9eed3006a01805a97cc81939e9d51727e332b03","object":"response","created_at":1778483290,"status":"in_progress","output":[],"model":"gpt-4o-mini","tools":[{"type":"function","name":"get_horoscope",...}]}}`,
			want:    "",
		},
		{
			desc:    "response.in_progress",
			payload: `{"type":"response.in_progress","response":{"id":"resp_0831a33f96d9eed3006a01805a97cc81939e9d51727e332b03","status":"in_progress"}}`,
			want:    "",
		},

		// --- output_item.added / content_part.added (message) ---
		{
			desc:    "output_item.added (message)",
			payload: `{"type":"response.output_item.added","item":{"id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","type":"message","status":"in_progress","content":[],"role":"assistant"},"output_index":0,"sequence_number":2}`,
			want:    "",
		},
		{
			desc:    "content_part.added (output_text)",
			payload: `{"type":"response.content_part.added","content_index":0,"item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","output_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""},"sequence_number":3}`,
			want:    "",
		},

		// --- output_text.delta stream ---
		{
			desc:    "output_text.delta 1",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":"I","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"IEJNG9po3UP16vB","output_index":0,"sequence_number":4}`,
			want:    "I",
		},
		{
			desc:    "output_text.delta 2",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":" will","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"oSYOkxrx9Nn","output_index":0,"sequence_number":5}`,
			want:    " will",
		},
		{
			desc:    "output_text.delta 3",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":" use","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"Xf6UQUn3VZA4","output_index":0,"sequence_number":6}`,
			want:    " use",
		},
		{
			desc:    "output_text.delta 4",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":" the","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"JE2kvDt1oZ9o","output_index":0,"sequence_number":7}`,
			want:    " the",
		},
		{
			desc:    "output_text.delta 5",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":" horoscope","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"Lrt3Cs","output_index":0,"sequence_number":8}`,
			want:    " horoscope",
		},
		{
			desc:    "output_text.delta 6",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":" tool","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"YE6fYy46rf1","output_index":0,"sequence_number":9}`,
			want:    " tool",
		},
		{
			desc:    "output_text.delta last",
			payload: `{"type":"response.output_text.delta","content_index":0,"delta":"!","item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"obfuscation":"oCYMOFNVozxzSCr","output_index":0,"sequence_number":64}`,
			want:    "!",
		},

		// --- output_text.done ---
		{
			desc:    "output_text.done",
			payload: `{"type":"response.output_text.done","content_index":0,"item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","logprobs":[],"output_index":0,"sequence_number":65,"text":"I will use the horoscope tool to fetch your daily horoscope as an Aquarius. This tool specializes in providing astrological insights, which are tailored to your sign. It's the most efficient way to get accurate and relevant information about what the stars have in store for you today. Let me retrieve that for you now!"}`,
			want:    "",
		},

		// --- content_part.done / output_item.done (message) ---
		{
			desc:    "content_part.done",
			payload: `{"type":"response.content_part.done","content_index":0,"item_id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","output_index":0,"part":{"type":"output_text","annotations":[],"logprobs":[],"text":"I will use the horoscope tool to fetch your daily horoscope as an Aquarius..."},"sequence_number":66}`,
			want:    "", // text nested under obj["part"]["text"], not reached
		},
		{
			desc:    "output_item.done (message)",
			payload: `{"type":"response.output_item.done","item":{"id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":"I will use the horoscope tool to fetch your daily horoscope as an Aquarius. This tool specializes in providing astrological insights, which are tailored to your sign. It's the most efficient way to get accurate and relevant information about what the stars have in store for you today. Let me retrieve that for you now!"}],"role":"assistant"},"output_index":0,"sequence_number":67}`,
			want:    "",
		},
		{
			desc:    "output_item.done (reasoning encrypted)",
			payload: `{"type":"response.output_item.done","item":{"id":"rs_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","type":"reasoning","encrypted_content":"gAAAAABencrypted","summary":[]},"output_index":0,"sequence_number":68}`,
			want:    "",
		},

		// --- function_call ---
		{
			desc:    "output_item.added (function_call)",
			payload: `{"type":"response.output_item.added","item":{"id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","type":"function_call","status":"in_progress","arguments":"","call_id":"call_JhmOrNSz6Jo5S1louMHhevgZ","name":"get_horoscope"},"output_index":1,"sequence_number":68}`,
			want:    "get_horoscope",
		},
		{
			desc:    "function_call_arguments.delta 1",
			payload: `{"type":"response.function_call_arguments.delta","delta":"{\"","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","obfuscation":"H6cHDeNwV4YQkG","output_index":1,"sequence_number":69}`,
			want:    `{"`,
		},
		{
			desc:    "function_call_arguments.delta 2",
			payload: `{"type":"response.function_call_arguments.delta","delta":"sign","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","obfuscation":"BMnfYfISv7T5","output_index":1,"sequence_number":70}`,
			want:    "sign",
		},
		{
			desc:    "function_call_arguments.delta 3",
			payload: `{"type":"response.function_call_arguments.delta","delta":"\":\"","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","obfuscation":"T5lLbk1NvpT8s","output_index":1,"sequence_number":71}`,
			want:    `":"`,
		},
		{
			desc:    "function_call_arguments.delta 4",
			payload: `{"type":"response.function_call_arguments.delta","delta":"Aqu","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","obfuscation":"pRqnTPjk3ByxR","output_index":1,"sequence_number":72}`,
			want:    "Aqu",
		},
		{
			desc:    "function_call_arguments.delta 5",
			payload: `{"type":"response.function_call_arguments.delta","delta":"arius","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","obfuscation":"4w6zOfkm1iG","output_index":1,"sequence_number":73}`,
			want:    "arius",
		},
		{
			desc:    "function_call_arguments.delta 6",
			payload: `{"type":"response.function_call_arguments.delta","delta":"\"}","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","obfuscation":"IDprKKTx0feKI5","output_index":1,"sequence_number":74}`,
			want:    `"}`,
		},
		{
			desc:    "function_call_arguments.done",
			payload: `{"type":"response.function_call_arguments.done","arguments":"{\"sign\":\"Aquarius\"}","item_id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","output_index":1,"sequence_number":75}`,
			want:    ``,
		},
		{
			desc:    "output_item.done (function_call)",
			payload: `{"type":"response.output_item.done","item":{"id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","type":"function_call","status":"completed","arguments":"{\"sign\":\"Aquarius\"}","call_id":"call_JhmOrNSz6Jo5S1louMHhevgZ","name":"get_horoscope"},"output_index":1,"sequence_number":76}`,
			want:    ``,
		},

		// --- Stream end ---
		{
			desc:    "response.completed",
			payload: `{"type":"response.completed","response":{"id":"resp_0831a33f96d9eed3006a01805a97cc81939e9d51727e332b03","object":"response","created_at":1778483290,"status":"completed","model":"gpt-4o-mini","output":[{"id":"msg_0831a33f96d9eed3006a01805b1e8c8193bd7364f72ee8940e","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":"I will use the horoscope tool to fetch your daily horoscope as an Aquarius. This tool specializes in providing astrological insights, which are tailored to your sign. It's the most efficient way to get accurate and relevant information about what the stars have in store for you today. Let me retrieve that for you now!"}],"role":"assistant"},{"id":"fc_0831a33f96d9eed3006a01805c83708193b4d9951227c5f5e0","type":"function_call","status":"completed","arguments":"{\"sign\":\"Aquarius\"}","call_id":"call_JhmOrNSz6Jo5S1louMHhevgZ","name":"get_horoscope"}],"parallel_tool_calls":true,"usage":{"input_tokens":88,"input_tokens_details":{"cached_tokens":0},"output_tokens":81,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":169}},"sequence_number":77}`,
			want:    "",
		},
	}
	var b strings.Builder
	for i, st := range steps {

		t.Run(st.desc, func(t *testing.T) {
			got := ExtractDeltaText("responses", []byte(st.payload))
			b.WriteString(got)
			if got != st.want {
				t.Errorf("%d %s ExtractDeltaText() = %q, want %q", i, st.desc, got, st.want)
			}
		})
	}
	t.Logf("finnally get:%s", b.String())
}
