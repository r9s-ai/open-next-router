package usageestimate

import (
	"encoding/json"
	"testing"
)

func TestExtractAnthropicMessagesRequest_FullRequest(t *testing.T) {
	body := mustJSONMap(t, `{
		"max_tokens": 128,
		"thinking": {"type": "enabled", "budget_tokens": 256},
		"tool_choice": {"type": "any"},
		"system": [
			{"type": "text", "text": "You are a concise assistant."}
		],
		"messages": [
			{"role": "user", "content": "Find Shanghai weather."},
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "Need weather lookup."},
					{
						"type": "tool_use",
						"id": "toolu_1",
						"name": "get_weather",
						"input": {"city": "Shanghai", "days": 2, "verbose": true, "tags": ["forecast"]}
					},
					{"type": "redacted_thinking", "data": "opaque"}
				]
			},
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_1",
						"content": [{"type": "text", "text": "Shanghai is cloudy."}]
					}
				]
			}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather by city.",
				"input_schema": {
					"type": "object",
					"properties": {
						"city": {"type": "string", "description": "City name."},
						"days": {"type": "integer", "description": "Forecast days."}
					},
					"required": ["city"]
				}
			},
			{
				"type": "web_search_20250305",
				"name": "web_search"
			}
		]
	}`)

	ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
	extractAnthropicMessagesRequest(ctx, body)

	if got, want := ctx.MaxTokens, 128; got != want {
		t.Fatalf("MaxTokens=%d want=%d", got, want)
	}
	if got, want := ctx.MaxThinkingTokens, 256; got != want {
		t.Fatalf("MaxThinkingTokens=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages), 4; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Role, "system"; got != want {
		t.Fatalf("system role=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Text, "You are a concise assistant."; got != want {
		t.Fatalf("system text=%q want=%q", got, want)
	}

	assistant := ctx.Messages[2]
	if got, want := assistant.Role, "assistant"; got != want {
		t.Fatalf("assistant role=%q want=%q", got, want)
	}
	if got, want := assistant.Content[1].Type, "tool_use"; got != want {
		t.Fatalf("tool_use type=%q want=%q", got, want)
	}
	if got, want := assistant.Content[1].ID, "toolu_1"; got != want {
		t.Fatalf("tool_use id=%q want=%q", got, want)
	}
	if got, want := assistant.Content[1].Name, "get_weather"; got != want {
		t.Fatalf("tool_use name=%q want=%q", got, want)
	}
	if got, want := assistant.Content[1].Arguments["city"], "Shanghai"; got != want {
		t.Fatalf("tool_use city=%#v want=%q", got, want)
	}

	toolResult := ctx.Messages[3].Content[0]
	if got, want := toolResult.Type, "tool_result"; got != want {
		t.Fatalf("tool_result type=%q want=%q", got, want)
	}
	if got, want := toolResult.ID, "toolu_1"; got != want {
		t.Fatalf("tool_result id=%q want=%q", got, want)
	}
	if got, want := toolResult.Content[0].Text, "Shanghai is cloudy."; got != want {
		t.Fatalf("tool_result nested text=%q want=%q", got, want)
	}

	if got, want := len(ctx.Tools), 2; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	tool := ctx.Tools[0]
	if got, want := tool.Name, "get_weather"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	if got, want := tool.Parameters.Type, "object"; got != want {
		t.Fatalf("tool schema type=%#v want=%q", got, want)
	}
	if got, want := tool.Parameters.Properties["city"].Description, "City name."; got != want {
		t.Fatalf("city description=%q want=%q", got, want)
	}
	if got, want := ctx.Tools[1].Name, "web_search"; got != want {
		t.Fatalf("builtin tool name=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemThinkingBlock, 1)
	assertOverHead(t, ctx, ItemToolChoiceAny, 1)
	assertOverHead(t, ctx, ItemSystemMessage, 1)
	assertOverHead(t, ctx, ItemRoleUser, 2)
	assertOverHead(t, ctx, ItemRoleAssistant, 1)
	assertOverHead(t, ctx, ItemFunctionCall, 1)
	assertOverHead(t, ctx, ItemFunctionCallResult, 1)
	assertOverHead(t, ctx, ItemHiddenReasoningBlock, 1)
	assertOverHead(t, ctx, ItemToolSection, 1)
	assertOverHead(t, ctx, ItemToolDefinition, 1)
}

func TestExtractAnthropicMessagesRequest_WebSearchOnlyDoesNotAddToolDefinitionOverheads(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
	extractAnthropicMessagesRequest(ctx, mustJSONMap(t, `{
		"tools": [
			{"type": "web_search_20250305", "name": "web_search"}
		]
	}`))

	if got, want := len(ctx.Tools), 1; got != want {
		t.Fatalf("tools len=%d want=%d", got, want)
	}
	if got, want := ctx.Tools[0].Name, "web_search"; got != want {
		t.Fatalf("tool name=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemToolSection, 0)
	assertOverHead(t, ctx, ItemToolDefinition, 0)
}

func TestExtractAnthropicMessagesRequest_ToolUseInputOverheads(t *testing.T) {
	body := mustJSONMap(t, `{
		"messages": [
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "toolu_1",
						"name": "mixed_args",
						"input": {
							"text": "x",
							"count": 2,
							"enabled": true,
							"items": ["a", "b"]
						}
					}
				]
			}
		]
	}`)

	ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
	extractAnthropicMessagesRequest(ctx, body)

	assertOverHead(t, ctx, ItemToolUseBlockInputString, 1)
	assertOverHead(t, ctx, ItemToolUseBlockInputInt, 1)
	assertOverHead(t, ctx, ItemToolUseBlockInputBool, 1)
	assertOverHead(t, ctx, ItemToolUseBlockInputList, 1)
}

func TestExtractAnthropicMessagesRequest_ToolChoiceOverheads(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want OverHeadItemKind
	}{
		{
			name: "any",
			raw:  `{"tool_choice":{"type":"any"}}`,
			want: ItemToolChoiceAny,
		},
		{
			name: "auto",
			raw:  `{"tool_choice":{"type":"auto"}}`,
			want: ItemToolChoiceAuto,
		},
		{
			name: "none",
			raw:  `{"tool_choice":{"type":"none"}}`,
			want: ItemToolChoiceNone,
		},
		{
			name: "named tool",
			raw:  `{"tool_choice":{"type":"tool","name":"get_weather"}}`,
			want: ItemToolChoiceToolName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
			extractAnthropicMessagesRequest(ctx, mustJSONMap(t, tt.raw))

			assertOverHead(t, ctx, tt.want, 1)
			if got := ctx.ToolChoice["type"]; got == "" {
				t.Fatalf("tool_choice type was not retained: %#v", ctx.ToolChoice)
			}
		})
	}
}

func TestExtractAnthropicMessagesRequest_ContentBlockOverheads(t *testing.T) {
	body := mustJSONMap(t, `{
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "AA=="}},
					{"type": "document", "source": {"type": "text", "media_type": "text/plain", "data": "doc"}},
					{"type": "unknown_block", "value": true}
				]
			}
		]
	}`)

	ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
	extractAnthropicMessagesRequest(ctx, body)

	if got, want := len(ctx.Messages), 1; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages[0].Content), 2; got != want {
		t.Fatalf("content len=%d want=%d", got, want)
	}
	if got, want := ctx.Messages[0].Content[0].Type, "image"; got != want {
		t.Fatalf("content[0].type=%q want=%q", got, want)
	}
	if got, want := ctx.Messages[0].Content[1].Type, "document"; got != want {
		t.Fatalf("content[1].type=%q want=%q", got, want)
	}
	assertOverHead(t, ctx, ItemImageBlock, 1)
	assertOverHead(t, ctx, ItemDocumentBlock, 1)
	assertOverHead(t, ctx, ItemUnknownBlcok, 1)
}

func TestExtractAnthropicMessagesRequest_ToolResultStringContent(t *testing.T) {
	body := mustJSONMap(t, `{
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "toolu_1",
						"content": "plain result"
					}
				]
			}
		]
	}`)

	ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
	extractAnthropicMessagesRequest(ctx, body)

	got := ctx.Messages[0].Content[0]
	if got.Type != "tool_result" || got.ID != "toolu_1" || got.Text != "plain result" {
		t.Fatalf("tool result=%#v", got)
	}
	assertOverHead(t, ctx, ItemFunctionCallResult, 1)
}

func TestExtractAnthropicMessagesRequest_IgnoresMalformedMessages(t *testing.T) {
	body := mustJSONMap(t, `{
		"messages": [
			"bad message",
			{"role": "user", "content": [{"text": "missing type"}]},
			{"role": "assistant", "content": 123}
		]
	}`)

	ctx := NewEstimateContext("claude-opus-4-7", apiMessages, EstimateInput)
	extractAnthropicMessagesRequest(ctx, body)

	if got, want := len(ctx.Messages), 2; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages[0].Content), 0; got != want {
		t.Fatalf("first content len=%d want=%d", got, want)
	}
	if got, want := len(ctx.Messages[1].Content), 0; got != want {
		t.Fatalf("second content len=%d want=%d", got, want)
	}
	assertOverHead(t, ctx, ItemRoleUser, 1)
	assertOverHead(t, ctx, ItemRoleAssistant, 1)
}

func TestExtractAnthropicMessagesResponse_FullResponse(t *testing.T) {
	body := mustJSONMap(t, `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4-8",
		"content": [
			{"type": "thinking", "thinking": "Need weather lookup.", "signature": "sig"},
			{"type": "redacted_thinking", "data": "opaque"},
			{"type": "text", "text": "I will check the weather."},
			{
				"type": "tool_use",
				"id": "toolu_1",
				"name": "get_weather",
				"input": {"city": "Shanghai", "days": 2, "alerts": true, "tags": ["forecast"]}
			},
			{
				"type": "server_tool_use",
				"id": "srvtoolu_1",
				"name": "web_search",
				"input": {"query": "Shanghai weather"}
			}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 10, "output_tokens": 20}
	}`)

	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateOutput)
	extractAnthropicMessagesResponse(ctx, body)

	if got, want := len(ctx.Messages), 1; got != want {
		t.Fatalf("messages len=%d want=%d", got, want)
	}
	msg := ctx.Messages[0]
	if got, want := msg.Role, "assistant"; got != want {
		t.Fatalf("role=%q want=%q", got, want)
	}
	if got, want := len(msg.Content), 5; got != want {
		t.Fatalf("content len=%d want=%d", got, want)
	}
	if got, want := msg.Content[0].Type, "thinking"; got != want {
		t.Fatalf("thinking type=%q want=%q", got, want)
	}
	if got, want := msg.Content[0].Text, "Need weather lookup."; got != want {
		t.Fatalf("thinking text=%q want=%q", got, want)
	}
	if got, want := msg.Content[0].Signature, "sig"; got != want {
		t.Fatalf("thinking signature=%q want=%q", got, want)
	}
	if got, want := msg.Content[1].Type, "redacted_thinking"; got != want {
		t.Fatalf("redacted type=%q want=%q", got, want)
	}
	if msg.Content[1].Raw == nil {
		t.Fatalf("redacted raw missing")
	}
	if got, want := msg.Content[2].Text, "I will check the weather."; got != want {
		t.Fatalf("text=%q want=%q", got, want)
	}

	toolUse := msg.Content[3]
	if got, want := toolUse.Type, "tool_use"; got != want {
		t.Fatalf("tool_use type=%q want=%q", got, want)
	}
	if got, want := toolUse.ID, "toolu_1"; got != want {
		t.Fatalf("tool_use id=%q want=%q", got, want)
	}
	if got, want := toolUse.Name, "get_weather"; got != want {
		t.Fatalf("tool_use name=%q want=%q", got, want)
	}
	if got, want := toolUse.Arguments["city"], "Shanghai"; got != want {
		t.Fatalf("tool_use city=%#v want=%q", got, want)
	}

	serverToolUse := msg.Content[4]
	if got, want := serverToolUse.Type, "server_tool_use"; got != want {
		t.Fatalf("server_tool_use type=%q want=%q", got, want)
	}
	if got, want := serverToolUse.Name, "web_search"; got != want {
		t.Fatalf("server_tool_use name=%q want=%q", got, want)
	}
	if got, want := serverToolUse.Arguments["query"], "Shanghai weather"; got != want {
		t.Fatalf("server_tool_use query=%#v want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemRoleAssistant, 1)
	assertOverHead(t, ctx, ItemHiddenReasoningBlock, 1)
	assertOverHead(t, ctx, ItemFunctionCall, 2)
	assertOverHead(t, ctx, ItemToolUseBlockInput, 2)
	assertOverHead(t, ctx, ItemToolUseBlockInputItem, 5)
	assertOverHead(t, ctx, ItemToolUseBlockInputString, 2)
	assertOverHead(t, ctx, ItemToolUseBlockInputInt, 1)
	assertOverHead(t, ctx, ItemToolUseBlockInputBool, 1)
	assertOverHead(t, ctx, ItemToolUseBlockInputList, 1)
}

func TestAnthropicMessagesOutputTemplateAggregatesResponseText(t *testing.T) {
	body := mustJSONMap(t, `{
		"role": "assistant",
		"content": [
			{"type": "thinking", "thinking": "Need lookup.", "signature": "signature_should_not_be_aggregated_as_text"},
			{"type": "text", "text": "Calling tool."},
			{
				"type": "tool_use",
				"id": "tool_use_id_should_not_be_aggregated",
				"name": "get_weather",
				"input": {"city": "Shanghai"}
			}
		]
	}`)

	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateOutput)
	extractAnthropicMessagesResponse(ctx, body)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	got := tokenizer.ApplyChatTemplate()
	assertContainsAll(t, got, "Need lookup.", "Calling tool.", "get_weather", "city", "Shanghai")
	assertNotContains(t, got, "signature_should_not_be_aggregated_as_text")
	assertNotContains(t, got, "tool_use_id_should_not_be_aggregated")
	assertOverHead(t, ctx, ItemThinkingSignature, countTextLenTokens("signature_should_not_be_aggregated_as_text"))
}

func mustJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	return out
}

func assertOverHead(t *testing.T, ctx *EstimateContext, kind OverHeadItemKind, want int) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("context is nil")
	}
	got := ctx.OverHeadItems[kind]
	if got != want {
		t.Fatalf("overhead[%s]=%d want=%d", kind, got, want)
	}
}
