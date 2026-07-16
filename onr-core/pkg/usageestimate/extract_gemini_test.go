package usageestimate

import "testing"

func TestExtractGeminiRequest_FullRequest(t *testing.T) {
	body := mustJSONMap(t, `{
		"system_instruction": {"parts": [{"text": "Follow policy."}]},
		"generationConfig": {
			"maxOutputTokens": 256,
			"thinkingConfig": {"thinkingBudget": 128, "includeThoughts": true},
			"responseSchema": {
				"type": "object",
				"properties": {"answer": {"type": "string", "description": "Final answer."}},
				"required": ["answer"]
			}
		},
		"contents": [
			{"role": "user", "parts": [
				{"text": "Weather in Shanghai?"},
				{"inlineData": {"mimeType": "image/png", "data": "AA=="}}
			]},
			{"role": "model", "parts": [
				{"functionCall": {"name": "get_weather", "args": {"city": "Shanghai", "days": 2}}}
			]},
			{"role": "user", "parts": [
				{"functionResponse": {"name": "get_weather", "response": {"forecast": "Cloudy."}}}
			]}
		],
		"tools": [
			{"function_declarations": [
				{
					"name": "get_weather",
					"description": "Get weather by city.",
					"parameters": {
						"type": "object",
						"properties": {
							"city": {"type": "string", "description": "City name."}
						},
						"required": ["city"]
					}
				}
			]},
			{"googleSearch": {}}
		]
	}`)

	ctx := NewEstimateContext("gemini-2.5-pro", apiGeminiGenerateContent, EstimateInput)
	extractGeminiRequest(ctx, body)

	if got, want := ctx.MaxTokens, 256; got != want {
		t.Fatalf("MaxTokens=%d want=%d", got, want)
	}
	if got, want := ctx.MaxThinkingTokens, 128; got != want {
		t.Fatalf("MaxThinkingTokens=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages), 4; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Role, "system"; got != want {
		t.Fatalf("system role=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Text, "Follow policy."; got != want {
		t.Fatalf("system text=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[1].Content[0].Text, "Weather in Shanghai?"; got != want {
		t.Fatalf("user text=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[2].Role, "assistant"; got != want {
		t.Fatalf("function call role=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[2].ToolCalls[0].Name, "get_weather"; got != want {
		t.Fatalf("function call name=%q want=%q", got, want)
	}
	args, ok := ctx.Messages[2].ToolCalls[0].Arguments.(map[string]any)
	if !ok {
		t.Fatalf("function call args=%T want map", ctx.Messages[2].ToolCalls[0].Arguments)
	}
	if got, want := args["city"], "Shanghai"; got != want {
		t.Fatalf("function call city=%#v want=%q", got, want)
	}
	if got, want := ctx.Messages[3].Role, "tool"; got != want {
		t.Fatalf("function response role=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[3].Name, "get_weather"; got != want {
		t.Fatalf("function response name=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[3].Content[0].Text, "forecast Cloudy."; got != want {
		t.Fatalf("function response text=%q want=%q", got, want)
	}
	if got, want := len(ctx.Tools), 2; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	if got, want := ctx.Tools[0].Name, "get_weather"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	if got, want := ctx.Tools[1].Name, "googleSearch"; got != want {
		t.Fatalf("builtin tool name=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemPromptBase, 1)
	assertOverHead(t, ctx, ItemRoleSystem, 1)
	assertOverHead(t, ctx, ItemRoleUser, 1)
	assertOverHead(t, ctx, ItemRoleAssistant, 1)
	assertOverHead(t, ctx, ItemRoleTool, 1)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
	assertOverHead(t, ctx, ItemFunctionCallResult, 1)
	assertOverHead(t, ctx, ItemThinkingBlock, 1)
	assertOverHead(t, ctx, ItemHiddenReasoningBlock, 1)
	assertOverHead(t, ctx, ItemImageBlock, 1)
	assertOverHead(t, ctx, ItemResponseFormatJsonSchema, 1)
	assertOverHead(t, ctx, ItemToolSection, 1)
	assertOverHead(t, ctx, ItemToolDefinition, 1)
	assertOverHead(t, ctx, ItemToolDescription, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 2)
	assertOverHead(t, ctx, ItemToolPropertiesTypeString, 2)
	assertOverHead(t, ctx, ItemToolRequired, 2)
	assertOverHead(t, ctx, ItemToolRequiredItem, 2)
}

func TestExtractGeminiResponse_FullResponse(t *testing.T) {
	body := mustJSONMap(t, `{
		"candidates": [
			{
				"content": {
					"role": "model",
					"parts": [
						{"text": "Hello."},
						{"text": "Need lookup.", "thought": true, "thoughtSignature": "opaque"},
						{"functionCall": {"name": "get_weather", "args": {"city": "Shanghai"}}},
						{"inlineData": {"mimeType": "image/png", "data": "AA=="}}
					]
				}
			}
		]
	}`)

	ctx := NewEstimateContext("gemini-2.5-pro", apiGeminiGenerateContent, EstimateOutput)
	extractGeminiResponse(ctx, body)

	if got, want := len(ctx.Messages), 1; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	msg := ctx.Messages[0]
	if got, want := msg.Role, "assistant"; got != want {
		t.Fatalf("role=%q want=%q", got, want)
	}
	if got, want := msg.Content[0].Text, "Hello."; got != want {
		t.Fatalf("text=%q want=%q", got, want)
	}
	if got, want := msg.Content[1].Type, "thinking"; got != want {
		t.Fatalf("thinking type=%q want=%q", got, want)
	}
	if got, want := msg.Content[1].Text, "Need lookup."; got != want {
		t.Fatalf("thinking text=%q want=%q", got, want)
	}
	if got, want := msg.ToolCalls[0].Name, "get_weather"; got != want {
		t.Fatalf("function call name=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemRoleAssistant, 1)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
	assertOverHead(t, ctx, ItemThinkingBlock, 1)
	assertOverHead(t, ctx, ItemHiddenReasoningBlock, 1)
	assertOverHead(t, ctx, ItemImageBlock, 1)
}

func TestExtractGeminiRequest_WebSearchOnlyDoesNotAddToolDefinitionOverheads(t *testing.T) {
	ctx := NewEstimateContext("gemini-2.5-pro", apiGeminiGenerateContent, EstimateInput)
	extractGeminiRequest(ctx, mustJSONMap(t, `{
		"tools": [
			{"googleSearch": {}}
		]
	}`))

	if got, want := len(ctx.Tools), 1; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	if got, want := ctx.Tools[0].Name, "googleSearch"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemToolSection, 0)
	assertOverHead(t, ctx, ItemToolDefinition, 0)
}

func TestGeminiTemplateAggregatesTextToolsAndCalls(t *testing.T) {
	body := mustJSONMap(t, `{
		"system_instruction": {"parts": [{"text": "Follow policy."}]},
		"contents": [
			{"role": "user", "parts": [{"text": "Weather?"}]},
			{"role": "model", "parts": [{"functionCall": {"name": "get_weather", "args": {"city": "Shanghai"}}}]},
			{"role": "user", "parts": [{"functionResponse": {"name": "get_weather", "response": {"forecast": "Cloudy."}}}]}
		],
		"tools": [
			{"function_declarations": [
				{
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
				}
			]}
		]
	}`)

	ctx := NewEstimateContext("gemini-2.5-pro", apiGeminiGenerateContent, EstimateInput)
	extractGeminiRequest(ctx, body)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	got := tokenizer.ApplyChatTemplate()
	assertContainsAll(t, got,
		"Follow policy.",
		"Weather?",
		"get_weather",
		"city",
		"Shanghai",
		"forecast",
		"Cloudy.",
		"weather_lookup_tool",
		"Get weather by city.",
		"Weather input.",
		"City name.",
	)
}
