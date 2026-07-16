package usageestimate

import "testing"

func TestExtractOpenAIChatRequest_FullRequest(t *testing.T) {
	body := mustJSONMap(t, `{
		"max_completion_tokens": 128,
		"tool_choice": {"type": "function", "function": {"name": "get_weather"}},
		"messages": [
			{"role": "system", "name": "policy", "content": "You are concise."},
			{"role": "developer", "content": [{"type": "text", "text": "Use tools when helpful."}]},
			{"role": "user", "content": [
				{"type": "text", "text": "Weather in Shanghai?"},
				{"type": "image_url", "image_url": {"url": "data:image/png;base64,AA=="}},
				{"type": "input_audio", "input_audio": {"data": "AA==", "format": "wav"}}
			]},
			{"role": "assistant", "content": "I will check.", "tool_calls": [
				{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"}}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "Cloudy."},
			{"role": "function", "name": "legacy_lookup", "content": "Legacy result."}
		],
		"tools": [
			{"type": "function", "function": {
				"name": "get_weather",
				"description": "Get weather by city.",
				"parameters": {
					"type": "object",
					"properties": {
						"city": {"type": "string", "description": "City name."}
					},
					"required": ["city"]
				}
			}},
			{"type": "custom", "custom": {"name": "custom_lookup", "description": "Run custom lookup."}}
		]
	}`)

	ctx := NewEstimateContext("gpt-5", apiChatCompletions, EstimateInput)
	extractOpenAIChatRequest(ctx, body)

	if got, want := ctx.MaxTokens, 128; got != want {
		t.Fatalf("MaxTokens=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages), 6; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Name, "policy"; got != want {
		t.Fatalf("message name=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[2].Content[0].Text, "Weather in Shanghai?"; got != want {
		t.Fatalf("user text=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[2].Content[1].Type, "image"; got != want {
		t.Fatalf("image type=%q want=%q", got, want)
	}
	if got, want := len(ctx.Messages[3].ToolCalls), 1; got != want {
		t.Fatalf("tool_calls len=%d want=%d", got, want)
	}
	call := ctx.Messages[3].ToolCalls[0]
	if got, want := call.ID, ""; got != want {
		t.Fatalf("tool call id=%q want empty", got)
	}
	if got, want := call.Name, "get_weather"; got != want {
		t.Fatalf("tool call name=%q want=%q", got, want)
	}
	if got, want := call.Arguments, "{\"city\":\"Shanghai\"}"; got != want {
		t.Fatalf("tool call arguments=%#v want=%q", got, want)
	}
	if got, want := ctx.Messages[4].ToolCallID, "call_1"; got != want {
		t.Fatalf("tool_call_id=%q want=%q", got, want)
	}
	if got, want := len(ctx.Tools), 2; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	if got, want := ctx.Tools[0].Name, "get_weather"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	if got, want := ctx.Tools[1].Name, "custom_lookup"; got != want {
		t.Fatalf("custom tool name=%q want=%q", got, want)
	}
	if got, want := ctx.ToolChoice["name"], "get_weather"; got != want {
		t.Fatalf("tool_choice name=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemPromptBase, 1)
	assertOverHead(t, ctx, ItemRoleSystem, 2)
	assertOverHead(t, ctx, ItemRoleUser, 1)
	assertOverHead(t, ctx, ItemRoleAssistant, 1)
	assertOverHead(t, ctx, ItemRoleTool, 2)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
	assertOverHead(t, ctx, ItemFunctionCallResult, 2)
	assertOverHead(t, ctx, ItemImageBlock, 1)
	assertOverHead(t, ctx, ItemUnknownBlcok, 1)
	assertOverHead(t, ctx, ItemToolChoiceToolName, 1)
	assertOverHead(t, ctx, ItemToolSection, 1)
	assertOverHead(t, ctx, ItemToolDefinition, 2)
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeString, 1)
	assertOverHead(t, ctx, ItemToolRequired, 1)
	assertOverHead(t, ctx, ItemToolRequiredItem, 1)
}

func TestExtractOpenAIChatRequest_ToolChoiceOverheads(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want OverHeadItemKind
	}{
		{name: "none", raw: `{"tool_choice":"none"}`, want: ItemToolChoiceNone},
		{name: "auto", raw: `{"tool_choice":"auto"}`, want: ItemToolChoiceAuto},
		{name: "required", raw: `{"tool_choice":"required"}`, want: ItemToolChoiceAny},
		{name: "function", raw: `{"tool_choice":{"type":"function","function":{"name":"lookup"}}}`, want: ItemToolChoiceToolName},
		{name: "custom", raw: `{"tool_choice":{"type":"custom","custom":{"name":"lookup"}}}`, want: ItemToolChoiceToolName},
		{name: "allowed tools", raw: `{"tool_choice":{"type":"allowed_tools","allowed_tools":{"mode":"auto"}}}`, want: ItemToolChoiceAny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewEstimateContext("gpt-5", apiChatCompletions, EstimateInput)
			extractOpenAIChatRequest(ctx, mustJSONMap(t, tt.raw))

			assertOverHead(t, ctx, tt.want, 1)
		})
	}
}

func TestExtractOpenAIChatRequest_DeprecatedFunctionCall(t *testing.T) {
	body := mustJSONMap(t, `{
		"messages": [
			{"role": "assistant", "content": null, "function_call": {"name": "legacy_lookup", "arguments": "{\"q\":\"x\"}"}}
		]
	}`)

	ctx := NewEstimateContext("gpt-5", apiChatCompletions, EstimateInput)
	extractOpenAIChatRequest(ctx, body)

	if got, want := len(ctx.Messages[0].ToolCalls), 1; got != want {
		t.Fatalf("tool_calls len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].ToolCalls[0].Name, "legacy_lookup"; got != want {
		t.Fatalf("function_call name=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemFunctionCall, 1)
}

func TestExtractOpenAIChatResponse_FullResponse(t *testing.T) {
	body := mustJSONMap(t, `{
		"id": "chatcmpl_1",
		"object": "chat.completion",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello.",
					"tool_calls": [
						{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"}}
					]
				},
				"finish_reason": "tool_calls"
			},
			{
				"index": 1,
				"message": {
					"role": "assistant",
					"content": [{"type": "text", "text": "Second choice."}],
					"refusal": "Cannot do that."
				},
				"finish_reason": "stop"
			}
		],
		"usage": {"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3}
	}`)

	ctx := NewEstimateContext("gpt-5", apiChatCompletions, EstimateOutput)
	extractOpenAIChatResponse(ctx, body)

	if got, want := len(ctx.Messages), 2; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Text, "Hello."; got != want {
		t.Fatalf("first content=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].ToolCalls[0].Name, "get_weather"; got != want {
		t.Fatalf("tool call name=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].Content[0].Text, "Second choice."; got != want {
		t.Fatalf("second content=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].Content[1].Type, "refusal"; got != want {
		t.Fatalf("refusal type=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].Content[1].Text, "Cannot do that."; got != want {
		t.Fatalf("refusal text=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemRoleAssistant, 2)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
}

func TestOpenAIChatTemplateAggregatesMessagesToolsAndToolCalls(t *testing.T) {
	body := mustJSONMap(t, `{
		"messages": [
			{"role": "system", "name": "policy", "content": "Be concise."},
			{"role": "user", "content": [{"type": "text", "text": "Weather?"}]},
			{"role": "assistant", "content": "Checking.", "tool_calls": [
				{"id": "tool_call_id_should_not_be_aggregated", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"}}
			]},
			{"role": "tool", "tool_call_id": "tool_result_id_should_not_be_aggregated", "content": "Cloudy."}
		],
		"tools": [
			{"type": "function", "function": {
				"name": "weather_lookup_tool",
				"description": "Get weather by city.",
				"parameters": {
					"type": "object",
					"description": "Weather input.",
					"properties": {
						"city": {"type": "string", "description": "City name."}
					},
					"required": ["city"]
				}
			}}
		]
	}`)

	ctx := NewEstimateContext("gpt-5", apiChatCompletions, EstimateInput)
	extractOpenAIChatRequest(ctx, body)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	got := tokenizer.ApplyChatTemplate()
	assertContainsAll(t, got,
		"policy",
		"Be concise.",
		"Weather?",
		"Checking.",
		"get_weather",
		"{\"city\":\"Shanghai\"}",
		"Cloudy.",
		"weather_lookup_tool",
		"Get weather by city.",
		"Weather input.",
		"city",
		"City name.",
	)
	assertNotContains(t, got, "tool_call_id_should_not_be_aggregated")
	assertNotContains(t, got, "tool_result_id_should_not_be_aggregated")
}

func TestExtractOpenAIResponsesRequest_FullRequest(t *testing.T) {
	body := mustJSONMap(t, `{
		"max_output_tokens": 512,
		"instructions": "Be concise.",
		"tool_choice": "auto",
		"input": [
			{"role": "user", "content": [
				{"type": "input_text", "text": "Weather in Shanghai?"},
				{"type": "input_image", "image_url": "data:image/png;base64,AA=="},
				{"type": "input_file", "file": {"file_id": "file_1"}}
			]},
			{"role": "assistant", "content": [{"type": "output_text", "text": "I will check."}]},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "Cloudy."}
		],
		"tools": [
			{
				"type": "function",
				"name": "get_weather",
				"description": "Get weather by city.",
				"parameters": {
					"type": "object",
					"properties": {
						"city": {"type": "string", "description": "City name."}
					},
					"required": ["city"]
				}
			},
			{"type": "web_search_preview", "search_context_size": "medium"}
		]
	}`)

	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	extractOpenAIResponsesRequest(ctx, body)

	if got, want := ctx.MaxTokens, 512; got != want {
		t.Fatalf("MaxTokens=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages), 5; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Role, "system"; got != want {
		t.Fatalf("instructions role=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].Content[0].Text, "Weather in Shanghai?"; got != want {
		t.Fatalf("user content=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[2].Content[0].Text, "I will check."; got != want {
		t.Fatalf("assistant content=%q want=%q", got, want)
	}
	if got, want := len(ctx.Messages[3].ToolCalls), 1; got != want {
		t.Fatalf("function_call len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[3].ToolCalls[0].Name, "get_weather"; got != want {
		t.Fatalf("function_call name=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[3].ToolCalls[0].ID, ""; got != want {
		t.Fatalf("function_call id=%q want empty", got)
	}
	if got, want := ctx.Messages[4].ToolCallID, ""; got != want {
		t.Fatalf("function_call_output call id=%q want empty", got)
	}
	if got, want := ctx.Messages[4].Content[0].Text, "Cloudy."; got != want {
		t.Fatalf("function_call_output text=%q want=%q", got, want)
	}
	if got, want := len(ctx.Tools), 2; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	if got, want := ctx.Tools[0].Name, "get_weather"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	if got, want := ctx.Tools[1].Name, "web_search_preview"; got != want {
		t.Fatalf("builtin tool name=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemPromptBase, 1)
	assertOverHead(t, ctx, ItemRoleSystem, 1)
	assertOverHead(t, ctx, ItemRoleUser, 1)
	assertOverHead(t, ctx, ItemRoleAssistant, 2)
	assertOverHead(t, ctx, ItemRoleTool, 1)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
	assertOverHead(t, ctx, ItemFunctionCallResult, 1)
	assertOverHead(t, ctx, ItemToolChoiceAuto, 1)
	assertOverHead(t, ctx, ItemImageBlock, 1)
	assertOverHead(t, ctx, ItemUnknownBlcok, 1)
	assertOverHead(t, ctx, ItemToolSection, 1)
	assertOverHead(t, ctx, ItemToolDefinition, 1)
	assertOverHead(t, ctx, ItemToolDescription, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeString, 1)
}

func TestExtractOpenAIResponsesRequest_WebSearchOnlyDoesNotAddToolDefinitionOverheads(t *testing.T) {
	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	extractOpenAIResponsesRequest(ctx, mustJSONMap(t, `{
		"tools": [
			{"type": "web_search_preview", "search_context_size": "medium"}
		]
	}`))

	if got, want := len(ctx.Tools), 1; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	if got, want := ctx.Tools[0].Name, "web_search_preview"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemToolSection, 0)
	assertOverHead(t, ctx, ItemToolDefinition, 0)
}

func TestExtractOpenAIResponsesRequest_StringInput(t *testing.T) {
	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	extractOpenAIResponsesRequest(ctx, mustJSONMap(t, `{"input":"hello"}`))

	if got, want := len(ctx.Messages), 1; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Role, "user"; got != want {
		t.Fatalf("role=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Text, "hello"; got != want {
		t.Fatalf("text=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemRoleUser, 1)
}

func TestExtractOpenAIResponsesRequest_CustomToolCallsAndEncryptedReasoning(t *testing.T) {
	body := mustJSONMap(t, `{
		"input": [
			{"type": "custom_tool_call", "call_id": "custom_call_id_should_not_be_aggregated", "name": "apply_patch", "status": "completed", "input": "*** Begin Patch\n*** Update File: file.go\n"},
			{"type": "custom_tool_call_output", "call_id": "custom_output_id_should_not_be_aggregated", "output": "Success. Updated file.go"},
			{"type": "reasoning", "encrypted_content": "encrypted_reasoning_payload", "summary": []}
		]
	}`)

	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	extractOpenAIResponsesRequest(ctx, body)

	if got, want := len(ctx.Messages), 2; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].ToolCalls[0].Type, "custom"; got != want {
		t.Fatalf("custom call type=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].ToolCalls[0].Name, "apply_patch"; got != want {
		t.Fatalf("custom call name=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].ToolCalls[0].Arguments, "*** Begin Patch\n*** Update File: file.go\n"; got != want {
		t.Fatalf("custom call input=%#v want=%q", got, want)
	}
	if got, want := ctx.Messages[1].Content[0].Text, "Success. Updated file.go"; got != want {
		t.Fatalf("custom output text=%q want=%q", got, want)
	}

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	got := tokenizer.ApplyChatTemplate()
	assertContainsAll(t, got,
		"apply_patch",
		"*** Begin Patch\n*** Update File: file.go\n",
		"Success. Updated file.go",
	)
	assertNotContains(t, got, "custom_call_id_should_not_be_aggregated")
	assertNotContains(t, got, "custom_output_id_should_not_be_aggregated")
	assertNotContains(t, got, "completed")
	assertNotContains(t, got, "encrypted_reasoning_payload")

	assertOverHead(t, ctx, ItemRoleAssistant, 2)
	assertOverHead(t, ctx, ItemRoleTool, 1)
	assertOverHead(t, ctx, ItemCustomToolCall, 1)
	assertOverHead(t, ctx, ItemCustomToolCallOutput, 1)
	assertOverHead(t, ctx, ItemHiddenReasoningBlock, 1)
}

func TestExtractOpenAIResponsesRequest_TextFormatJsonSchema(t *testing.T) {
	body := mustJSONMap(t, `{
		"text": {
			"format": {
				"type": "json_schema",
				"name": "t",
				"strict": true,
				"schema": {
					"type": "object",
					"properties": {
						"x": {"type": "string"}
					},
					"required": ["x"],
					"additionalProperties": false
				}
			}
		},
		"input": "json it"
	}`)

	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	extractOpenAIResponsesRequest(ctx, body)

	assertOverHead(t, ctx, ItemResponseFormatJsonSchema, 1)
	assertOverHead(t, ctx, ItemResponseFormatJsonSchemaStringPropertyRequired, 1)
}

func TestExtractOpenAIResponsesResponse_FullResponse(t *testing.T) {
	body := mustJSONMap(t, `{
		"id": "resp_1",
		"status": "completed",
		"output_text": "fallback should not be used",
		"output": [
			{
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "output_text", "text": "Hello."},
					{"type": "refusal", "refusal": "Cannot do that."}
				]
			},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"},
			{"type": "reasoning", "summary": [{"type": "summary_text", "text": "Need weather lookup."}]}
		]
	}`)

	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateOutput)
	extractOpenAIResponsesResponse(ctx, body)

	if got, want := len(ctx.Messages), 3; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Text, "Hello."; got != want {
		t.Fatalf("message text=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].Content[1].Text, "Cannot do that."; got != want {
		t.Fatalf("refusal text=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].ToolCalls[0].Name, "get_weather"; got != want {
		t.Fatalf("function_call name=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].ToolCalls[0].ID, ""; got != want {
		t.Fatalf("function_call id=%q want empty", got)
	}
	if got, want := ctx.Messages[2].Content[0].Type, "thinking"; got != want {
		t.Fatalf("reasoning type=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[2].Content[0].Text, "Need weather lookup."; got != want {
		t.Fatalf("reasoning text=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemRoleAssistant, 3)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
	assertOverHead(t, ctx, ItemHiddenReasoningBlock, 1)
}

func TestExtractOpenAIResponsesResponse_OutputTextFallback(t *testing.T) {
	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateOutput)
	extractOpenAIResponsesResponse(ctx, mustJSONMap(t, `{"output_text":"Hello fallback."}`))

	if got, want := len(ctx.Messages), 1; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Text, "Hello fallback."; got != want {
		t.Fatalf("fallback text=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemRoleAssistant, 1)
}

func TestOpenAIResponsesTemplateAggregatesTextToolsAndCalls(t *testing.T) {
	body := mustJSONMap(t, `{
		"instructions": "Be concise.",
		"input": [
			{"role": "user", "content": [{"type": "input_text", "text": "Weather?"}]},
			{"type": "function_call", "call_id": "call_id_should_not_be_aggregated", "name": "get_weather", "arguments": "{\"city\":\"Shanghai\"}"},
			{"type": "function_call_output", "call_id": "call_output_id_should_not_be_aggregated", "output": "Cloudy."}
			],
			"tools": [
				{
					"type": "function",
					"name": "weather_lookup_tool",
					"description": "Get weather by city.",
					"parameters": {
						"type": "object",
						"description": "Weather input.",
						"properties": {
							"city": {"type": "string", "description": "City name."}
						},
						"required": ["city"]
					}
				},
				{
					"type": "custom",
					"name": "apply_patch",
					"description": "Apply patch text.",
					"format": {
						"type": "grammar",
						"syntax": "lark",
						"definition": "start: patch"
					}
				},
				{
					"type": "tool_search",
					"description": "Find deferred tools.",
					"parameters": {
						"type": "object",
						"properties": {
							"query": {"type": "string", "description": "Search query."}
						},
						"required": ["query"]
					}
				}
			]
		}`)

	ctx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	extractOpenAIResponsesRequest(ctx, body)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	got := tokenizer.ApplyChatTemplate()
	assertContainsAll(t, got,
		"Be concise.",
		"Weather?",
		"get_weather",
		"{\"city\":\"Shanghai\"}",
		"Cloudy.",
		"weather_lookup_tool",
		"Get weather by city.",
		"Weather input.",
		"city",
		"City name.",
		"apply_patch",
		"Apply patch text.",
		"start: patch",
		"tool_search",
		"Find deferred tools.",
		"query",
		"Search query.",
	)
	assertNotContains(t, got, "call_id_should_not_be_aggregated")
	assertNotContains(t, got, "call_output_id_should_not_be_aggregated")
}
