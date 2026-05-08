package requesttransform

import (
	"encoding/json"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestApply_JSONOpsThenReqMap(t *testing.T) {
	t.Parallel()

	meta := &dslmeta.Meta{DSLModelMapped: "gpt-4.1"}
	body := []byte(`{"model":"gpt-4o-mini","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"hello"}],"metadata":{"trace_id":"req-transform"}}`)
	value := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []any{
			map[string]any{"role": "system", "content": "You are helpful."},
			map[string]any{"role": "user", "content": "hello"},
		},
		"metadata": map[string]any{"trace_id": "req-transform"},
	}

	result, err := Apply(meta, "application/json", body, value, &dslconfig.RequestTransform{
		JSONOps: []dslconfig.JSONOp{
			{Op: "json_set", Path: "$.metadata.origin", ValueExpr: `"dsl"`},
		},
		ReqMapMode: "openai_chat_to_openai_responses",
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	root := result.Value
	if root == nil {
		t.Fatalf("Value = nil")
	}
	if _, exists := root["messages"]; exists {
		t.Fatalf("messages should be removed after req_map, got=%v", root)
	}
	if got, want := root["model"], "gpt-4.1"; got != want {
		t.Fatalf("model=%v want=%v", got, want)
	}
	metaMap, _ := root["metadata"].(map[string]any)
	if got, want := metaMap["origin"], "dsl"; got != want {
		t.Fatalf("metadata.origin=%v want=%v", got, want)
	}
}

func TestApply_AfterReqMapJSONOpsRunAfterReqMap(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}]}`)
	value := map[string]any{
		"model":    "claude-3-5-sonnet-20241022",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}

	result, err := Apply(&dslmeta.Meta{}, "application/json", body, value, &dslconfig.RequestTransform{
		ReqMapMode: "openai_chat_to_anthropic_messages",
		AfterReqMapJSONOps: []dslconfig.JSONOp{
			{Op: "json_set", Path: "$.anthropic_version", ValueExpr: `"bedrock-2023-05-31"`},
		},
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got, want := result.Root["anthropic_version"], "bedrock-2023-05-31"; got != want {
		t.Fatalf("anthropic_version=%v want=%v", got, want)
	}
	if _, exists := result.Root["messages"]; !exists {
		t.Fatalf("expected mapped anthropic messages root, got=%v", result.Root)
	}
}

func TestApply_AfterReqMapJSONOpsRunAfterJSONOpsWithoutReqMap(t *testing.T) {
	t.Parallel()

	value := map[string]any{"model": "gpt-4o-mini"}
	result, err := Apply(&dslmeta.Meta{}, "application/json", nil, value, &dslconfig.RequestTransform{
		JSONOps: []dslconfig.JSONOp{
			{Op: "json_set", Path: "$.metadata.phase", ValueExpr: `"before"`},
		},
		AfterReqMapJSONOps: []dslconfig.JSONOp{
			{Op: "json_set", Path: "$.metadata.phase", ValueExpr: `"after"`},
		},
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	metadata, ok := result.Root["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata=%T", result.Root["metadata"])
	}
	if got, want := metadata["phase"], "after"; got != want {
		t.Fatalf("metadata.phase=%v want=%v", got, want)
	}
}

func TestApply_ReqMapOnRawBodyWithoutParsedValue(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`)
	result, err := Apply(&dslmeta.Meta{}, "application/json", body, nil, &dslconfig.RequestTransform{
		ReqMapMode: "openai_chat_to_openai_responses",
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Root == nil {
		t.Fatalf("expected parsed root after req_map")
	}
	if _, exists := result.Root["messages"]; exists {
		t.Fatalf("messages should be removed after req_map, got=%v", result.Root)
	}
}

func TestApplyReqMap_RejectsEncodedClientRequest(t *testing.T) {
	t.Parallel()

	_, _, err := ApplyReqMap("openai_chat_to_openai_responses", []byte(`{}`), nil, ApplyOptions{
		ContentEncoding: "gzip",
	})
	if err == nil {
		t.Fatalf("expected encoded request error")
	}
}

func TestApply_MultipartPreservesOriginalBody(t *testing.T) {
	t.Parallel()

	original := []byte("--boundary\r\n...")
	value := map[string]any{"model": "gpt-image-1", "n": "2"}
	result, err := Apply(&dslmeta.Meta{}, "multipart/form-data; boundary=boundary", original, value, &dslconfig.RequestTransform{}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if string(result.Body) != string(original) {
		t.Fatalf("multipart body changed: %q", string(result.Body))
	}
}

func TestApply_JSONWrapInputTextMarshalsBody(t *testing.T) {
	t.Parallel()

	const prompt = "Generate an image of gray tabby cat hugging an otter with an orange scarf"
	body := []byte(`{"model":"gpt-image-1","input":"` + prompt + `"}`)
	value := map[string]any{
		"model": "gpt-image-1",
		"input": prompt,
	}

	result, err := Apply(&dslmeta.Meta{}, "application/json", body, value, &dslconfig.RequestTransform{
		JSONOps: []dslconfig.JSONOp{
			{Op: "json_wrap_input_text", Path: "$.input"},
		},
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(result.Body, &root); err != nil {
		t.Fatalf("unmarshal transformed body: %v", err)
	}
	input, ok := root["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("input=%T %#v, want one message", root["input"], root["input"])
	}
	message, ok := input[0].(map[string]any)
	if !ok || message["role"] != "user" {
		t.Fatalf("message=%#v", input[0])
	}
	content, ok := message["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content=%#v", message["content"])
	}
	block, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("block=%#v", content[0])
	}
	if got, want := block["type"], "input_text"; got != want {
		t.Fatalf("type=%v want=%v", got, want)
	}
	if got, want := block["text"], prompt; got != want {
		t.Fatalf("text=%v want=%v", got, want)
	}
}

//gocyclo:ignore
func TestApplyReqMap_OpenAIChatToAnthropicMessages_UsesTypedAnthropicConversion(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"claude-2",
		"messages":[
			{"role":"system","content":"You are helpful."},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"I'll call a tool.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_weather","arguments":"{\"city\":\"Boston\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"sunny"}
		],
		"tools":[
			{"type":"function","function":{"name":"lookup_weather","description":"Lookup weather","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}
		],
		"tool_choice":{"type":"function","function":{"name":"lookup_weather"}},
		"max_tokens":999999,
		"reasoning_effort":"high",
		"stop":["END"],
		"stream":true,
		"user":"user-123"
	}`)

	mappedBody, mappedRoot, err := ApplyReqMap("openai_chat_to_anthropic_messages", body, nil, ApplyOptions{})
	if err != nil {
		t.Fatalf("ApplyReqMap() error = %v", err)
	}
	if len(mappedBody) == 0 {
		t.Fatalf("expected mapped body")
	}

	root := mappedRoot
	if root == nil {
		t.Fatalf("mapped value = nil")
	}
	if got, want := root["model"], "claude-2.1"; got != want {
		t.Fatalf("model=%v want=%v", got, want)
	}
	if got, want := mustInt(t, root["max_tokens"]), 64*1000; got != want {
		t.Fatalf("max_tokens=%v want=%v", got, want)
	}
	if got, want := root["stream"], true; got != want {
		t.Fatalf("stream=%v want=%v", got, want)
	}

	stopSequences, ok := root["stop_sequences"].([]any)
	if !ok || len(stopSequences) != 1 || stopSequences[0] != "END" {
		t.Fatalf("stop_sequences=%v", root["stop_sequences"])
	}

	metadata, ok := root["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata=%T", root["metadata"])
	}
	if got, want := metadata["user_id"], "user-123"; got != want {
		t.Fatalf("metadata.user_id=%v want=%v", got, want)
	}

	thinking, ok := root["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking=%T", root["thinking"])
	}
	if got, want := thinking["type"], "enabled"; got != want {
		t.Fatalf("thinking.type=%v want=%v", got, want)
	}
	if got, want := mustInt(t, thinking["budget_tokens"]), 16*1024; got != want {
		t.Fatalf("thinking.budget_tokens=%v want=%v", got, want)
	}

	toolChoice, ok := root["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("tool_choice=%T", root["tool_choice"])
	}
	if got, want := toolChoice["type"], "tool"; got != want {
		t.Fatalf("tool_choice.type=%v want=%v", got, want)
	}
	if got, want := toolChoice["name"], "lookup_weather"; got != want {
		t.Fatalf("tool_choice.name=%v want=%v", got, want)
	}

	system, ok := root["system"].([]any)
	if !ok || len(system) != 1 {
		t.Fatalf("system=%v", root["system"])
	}
	systemBlock, ok := system[0].(map[string]any)
	if !ok || systemBlock["type"] != "text" || systemBlock["text"] != "You are helpful." {
		t.Fatalf("system block=%v", system[0])
	}

	messages, ok := root["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("messages=%v", root["messages"])
	}

	userMsg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("user message=%T", messages[0])
	}
	if got, want := userMsg["role"], "user"; got != want {
		t.Fatalf("user role=%v want=%v", got, want)
	}
	userContent := userMsg["content"].([]any)
	userBlock := userContent[0].(map[string]any)
	if got, want := userBlock["type"], "text"; got != want {
		t.Fatalf("user content type=%v want=%v", got, want)
	}
	if got, want := userBlock["text"], "hello"; got != want {
		t.Fatalf("user content text=%v want=%v", got, want)
	}

	assistantMsg := messages[1].(map[string]any)
	assistantContent := assistantMsg["content"].([]any)
	if len(assistantContent) != 2 {
		t.Fatalf("assistant content=%v", assistantContent)
	}
	toolUse := assistantContent[0].(map[string]any)
	if got, want := toolUse["type"], "tool_use"; got != want {
		t.Fatalf("tool_use.type=%v want=%v", got, want)
	}
	if got, want := toolUse["name"], "lookup_weather"; got != want {
		t.Fatalf("tool_use.name=%v want=%v", got, want)
	}

	toolMsg := messages[2].(map[string]any)
	toolContent := toolMsg["content"].([]any)
	toolResult := toolContent[0].(map[string]any)
	if got, want := toolResult["type"], "tool_result"; got != want {
		t.Fatalf("tool_result.type=%v want=%v", got, want)
	}
	if got, want := toolResult["content"], "sunny"; got != want {
		t.Fatalf("tool_result.content=%v want=%v", got, want)
	}
}

func mustInt(t *testing.T, v any) int {
	t.Helper()

	switch n := v.(type) {
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		t.Fatalf("expected numeric value, got %T", v)
		return 0
	}
}

func TestApplyReqMap_StructFirstWrapperModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mode      string
		body      string
		assertion func(t *testing.T, root map[string]any)
	}{
		{
			name: "openai chat to gemini",
			mode: "openai_chat_to_gemini_generate_content",
			body: `{
				"model":"gemini-3-pro",
				"messages":[
					{"role":"system","content":"You are helpful."},
					{"role":"assistant","content":"Hi there"},
					{"role":"user","content":"hello"}
				],
				"functions":[
					{"name":"lookup_weather","description":"Lookup weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}
				],
				"response_format":{"type":"json_object","json_schema":{"name":"weather","schema":{"type":"object"}}},
				"reasoning_effort":"high",
				"modalities":["text","image"],
				"max_tokens":128
			}`,
			assertion: func(t *testing.T, root map[string]any) {
				t.Helper()
				contents, ok := root["contents"].([]any)
				if !ok {
					t.Fatalf("contents=%T", root["contents"])
				}
				if len(contents) != 4 {
					t.Fatalf("contents len=%d want=4 contents=%v", len(contents), contents)
				}
				systemMsg := contents[0].(map[string]any)
				if got, want := systemMsg["role"], "user"; got != want {
					t.Fatalf("system role=%v want=%v", got, want)
				}
				dummyModel := contents[1].(map[string]any)
				if got, want := dummyModel["role"], "model"; got != want {
					t.Fatalf("dummy role=%v want=%v", got, want)
				}
				assistantMsg := contents[2].(map[string]any)
				if got, want := assistantMsg["role"], "model"; got != want {
					t.Fatalf("assistant role=%v want=%v", got, want)
				}

				tools, ok := root["tools"].([]any)
				if !ok || len(tools) != 1 {
					t.Fatalf("tools=%v", root["tools"])
				}
				generationConfig, ok := root["generationConfig"].(map[string]any)
				if !ok {
					t.Fatalf("generationConfig=%T", root["generationConfig"])
				}
				if got, want := generationConfig["responseMimeType"], "application/json"; got != want {
					t.Fatalf("responseMimeType=%v want=%v", got, want)
				}
				if got, want := mustInt(t, generationConfig["maxOutputTokens"]), 128; got != want {
					t.Fatalf("maxOutputTokens=%v want=%v", got, want)
				}
				responseModalities, ok := generationConfig["responseModalities"].([]any)
				if !ok || len(responseModalities) != 2 || responseModalities[0] != "TEXT" || responseModalities[1] != "IMAGE" {
					t.Fatalf("responseModalities=%v", generationConfig["responseModalities"])
				}
				thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]any)
				if !ok {
					t.Fatalf("thinkingConfig=%T", generationConfig["thinkingConfig"])
				}
				if got, want := thinkingConfig["thinkingLevel"], "high"; got != want {
					t.Fatalf("thinkingLevel=%v want=%v", got, want)
				}
			},
		},
		{
			name: "anthropic to openai chat",
			mode: "anthropic_to_openai_chat",
			body: `{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`,
			assertion: func(t *testing.T, root map[string]any) {
				t.Helper()
				if got, want := root["model"], "claude-sonnet-4-5"; got != want {
					t.Fatalf("model=%v want=%v", got, want)
				}
				if _, ok := root["messages"].([]any); !ok {
					t.Fatalf("messages=%T", root["messages"])
				}
			},
		},
		{
			name: "gemini to openai chat",
			mode: "gemini_to_openai_chat",
			body: `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`,
			assertion: func(t *testing.T, root map[string]any) {
				t.Helper()
				if _, ok := root["messages"].([]any); !ok {
					t.Fatalf("messages=%T", root["messages"])
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mappedBody, mappedRoot, err := ApplyReqMap(tt.mode, []byte(tt.body), nil, ApplyOptions{})
			if err != nil {
				t.Fatalf("ApplyReqMap() error = %v", err)
			}
			if len(mappedBody) == 0 {
				t.Fatalf("expected mapped body")
			}

			root := mappedRoot
			if root == nil {
				t.Fatalf("mapped value = nil")
			}
			if !json.Valid(mappedBody) {
				t.Fatalf("mapped body is not valid json: %s", string(mappedBody))
			}
			tt.assertion(t, root)
		})
	}
}
