package apitypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIChatCompletionsRequestMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"model":                 "gpt-5",
		"messages":              []any{map[string]any{"role": "user", "content": "hello"}, map[string]any{"role": "assistant", "audio": map[string]any{"id": "audio_1"}, "tool_calls": []any{map[string]any{"id": "call_1", "type": "function", "function": map[string]any{"name": "lookup", "arguments": "{\"city\":\"SF\"}"}}}}},
		"stream":                true,
		"temperature":           0.7,
		"max_completion_tokens": float64(256),
		"tools":                 []any{map[string]any{"type": "function", "function": map[string]any{"name": "lookup"}}},
		"tool_choice":           map[string]any{"type": "function", "function": map[string]any{"name": "lookup"}},
		"parallel_tool_calls":   true,
		"response_format":       map[string]any{"type": "json_schema"},
		"metadata":              map[string]any{"trace_id": "abc"},
		"reasoning_effort":      "medium",
		"service_tier":          "default",
	}

	var req OpenAIChatCompletionsRequest
	require.NoError(t, req.FromMap(input))
	require.Equal(t, "gpt-5", req.Model)
	require.Len(t, req.Messages, 2)
	require.NotNil(t, req.Messages[0].Content)
	require.NotNil(t, req.Messages[0].Content.Text)
	require.Equal(t, "hello", *req.Messages[0].Content.Text)
	require.NotNil(t, req.Messages[1].Audio)
	require.Equal(t, "audio_1", req.Messages[1].Audio.ID)
	require.Len(t, req.Messages[1].ToolCalls, 1)
	require.Equal(t, "lookup", req.Messages[1].ToolCalls[0].Function.Name)
	require.NotNil(t, req.Temperature)
	require.Equal(t, 0.7, *req.Temperature)
	require.Equal(t, 256, req.MaxCompletionTokens)
	require.Len(t, req.Tools, 1)
	require.Equal(t, "medium", req.ReasoningEffort)

	got, err := req.ToMap()
	require.NoError(t, err)
	require.Equal(t, "gpt-5", got["model"])
	require.Equal(t, true, got["stream"])
	require.Equal(t, 0.7, got["temperature"])
	require.Equal(t, 256, got["max_completion_tokens"])
	messages, ok := got["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
	secondMessage, ok := messages[1].(map[string]any)
	require.True(t, ok)
	audio, ok := secondMessage["audio"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "audio_1", audio["id"])
	toolCalls, ok := secondMessage["tool_calls"].([]any)
	require.True(t, ok)
	require.Len(t, toolCalls, 1)
}

func TestOpenAIChatCompletionsResponseFromMap(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"id":      "chatcmpl-1",
		"object":  "chat.completion",
		"created": int64(1710000000),
		"model":   "gpt-5",
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     float64(10),
			"completion_tokens": float64(5),
			"total_tokens":      float64(15),
			"completion_tokens_details": map[string]any{
				"reasoning_tokens": float64(2),
			},
		},
	}

	var resp OpenAIChatCompletionsResponse
	require.NoError(t, resp.FromMap(input))
	require.Equal(t, int64(1710000000), resp.Created)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message.Content)
	require.NotNil(t, resp.Choices[0].Message.Content.Text)
	require.Equal(t, "done", *resp.Choices[0].Message.Content.Text)
	require.NotNil(t, resp.Usage)
	require.Equal(t, 15, resp.Usage.TotalTokens)
	require.NotNil(t, resp.Usage.CompletionTokensDetails)
	require.Equal(t, 2, resp.Usage.CompletionTokensDetails.ReasoningTokens)
}

func TestOpenAIChatCompletionsStreamResponseMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"id":      "chatcmpl-1",
		"object":  "chat.completion.chunk",
		"created": float64(1710000001),
		"model":   "gpt-5",
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": "hel",
					"tool_calls": []any{
						map[string]any{
							"index": 0,
							"id":    "call_1",
							"type":  "function",
							"function": map[string]any{
								"name":      "lookup",
								"arguments": "{\"city\":\"SF\"}",
							},
						},
					},
				},
			},
		},
	}

	var resp OpenAIChatCompletionsStreamResponse
	require.NoError(t, resp.FromMap(input))
	require.Equal(t, int64(1710000001), resp.Created)
	require.Len(t, resp.Choices, 1)
	require.Equal(t, "hel", resp.Choices[0].Delta.Content)
	require.Len(t, resp.Choices[0].Delta.ToolCalls, 1)

	got, err := resp.ToMap()
	require.NoError(t, err)
	require.Equal(t, int64(1710000001), got["created"])
	choices, ok := got["choices"].([]any)
	require.True(t, ok)
	require.Len(t, choices, 1)
}

func TestOpenAIResponsesRequestMapRoundTrip(t *testing.T) {
	t.Parallel()

	store := true
	input := map[string]any{
		"model":                "gpt-5",
		"input":                []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "hello"}}}},
		"instructions":         "be concise",
		"stream":               true,
		"max_output_tokens":    float64(512),
		"temperature":          0.3,
		"tools":                []any{map[string]any{"type": "web_search_preview", "search_context_size": "medium"}},
		"tool_choice":          "auto",
		"parallel_tool_calls":  true,
		"text":                 map[string]any{"format": map[string]any{"type": "text"}},
		"store":                store,
		"metadata":             map[string]any{"trace_id": "abc"},
		"reasoning":            map[string]any{"effort": "medium"},
		"previous_response_id": "resp_prev",
		"truncation":           "auto",
	}

	var req OpenAIResponsesRequest
	require.NoError(t, req.FromMap(input))
	require.Equal(t, "gpt-5", req.Model)
	require.Equal(t, "be concise", req.Instructions)
	require.Equal(t, 512, req.MaxOutputTokens)
	require.Len(t, req.Tools, 1)
	require.Equal(t, "web_search_preview", req.Tools[0].Type)
	require.Equal(t, "medium", req.Tools[0].SearchContextSize)
	require.NotNil(t, req.Store)
	require.True(t, *req.Store)
	require.NotNil(t, req.Text)
	require.NotNil(t, req.Text.Format)
	require.Equal(t, "text", req.Text.Format.Type)
	require.NotNil(t, req.Reasoning)
	require.Equal(t, "medium", req.Reasoning.Effort)

	got, err := req.ToMap()
	require.NoError(t, err)
	require.Equal(t, "gpt-5", got["model"])
	require.Equal(t, 512, got["max_output_tokens"])
	tools, ok := got["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "medium", tool["search_context_size"])
}

func TestOpenAIResponsesResponseMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"id":          "resp_1",
		"object":      "response",
		"created_at":  float64(1710000010),
		"model":       "gpt-5",
		"status":      "completed",
		"output_text": "hello world",
		"output": []any{
			map[string]any{
				"id":     "msg_1",
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": "hello world",
						"annotations": []any{
							map[string]any{"type": "url_citation"},
						},
					},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  float64(20),
			"output_tokens": float64(10),
			"total_tokens":  float64(30),
			"output_token_details": map[string]any{
				"reasoning_tokens": float64(4),
			},
		},
		"metadata": map[string]any{"trace_id": "abc"},
		"user":     "user_1",
	}

	var resp OpenAIResponsesResponse
	require.NoError(t, resp.FromMap(input))
	require.Equal(t, int64(1710000010), resp.CreatedAt)
	require.Equal(t, "hello world", resp.OutputText)
	require.Len(t, resp.Output, 1)
	require.Len(t, resp.Output[0].Content, 1)
	require.Equal(t, "output_text", resp.Output[0].Content[0].Type)
	require.Len(t, resp.Output[0].Content[0].Annotations, 1)
	require.Equal(t, "url_citation", resp.Output[0].Content[0].Annotations[0].Type)
	require.NotNil(t, resp.Usage)
	require.Equal(t, 30, resp.Usage.TotalTokens)
	require.Equal(t, "abc", resp.Metadata["trace_id"])

	got, err := resp.ToMap()
	require.NoError(t, err)
	require.Equal(t, int64(1710000010), got["created_at"])
	output, ok := got["output"].([]any)
	require.True(t, ok)
	require.Len(t, output, 1)
}

func TestOpenAIResponsesStreamResponseMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"type":          "response.output_text.delta",
		"output_index":  float64(0),
		"content_index": float64(0),
		"delta":         "hel",
		"item": map[string]any{
			"id":     "msg_1",
			"type":   "message",
			"role":   "assistant",
			"status": "in_progress",
		},
		"response": map[string]any{
			"id":         "resp_1",
			"object":     "response",
			"created_at": float64(1710000010),
		},
		"usage": map[string]any{
			"input_tokens": float64(20),
			"total_tokens": float64(20),
		},
		"sequence_number": float64(3),
	}

	var resp OpenAIResponsesStreamResponse
	require.NoError(t, resp.FromMap(input))
	require.Equal(t, "response.output_text.delta", resp.Type)
	require.Equal(t, "hel", resp.Delta)
	require.NotNil(t, resp.Item)
	require.Equal(t, "message", resp.Item.Type)
	require.NotNil(t, resp.Response)
	require.Equal(t, "resp_1", resp.Response.ID)
	require.Equal(t, int64(3), resp.SequenceNumber)

	got, err := resp.ToMap()
	require.NoError(t, err)
	require.Equal(t, int64(3), got["sequence_number"])
	require.Equal(t, "hel", got["delta"])
}

func TestOpenAIChatCompletionsRequestGetPrompt(t *testing.T) {
	t.Parallel()

	userText := "hello"
	req := &OpenAIChatCompletionsRequest{
		Messages: []OpenAIChatMessage{
			{Role: "system", Content: &OpenAIChatMessageContent{Text: &userText}},
			{Role: "user", Content: &OpenAIChatMessageContent{Text: &userText}},
			{
				Role: "user",
				Content: &OpenAIChatMessageContent{
					Parts: []OpenAIChatContentPart{
						{Type: "text", Text: "part 1"},
						{Type: "input_image", Text: ""},
						{Type: "text", Text: "part 2"},
					},
				},
			},
			{Role: "assistant", Content: &OpenAIChatMessageContent{Text: &userText}},
		},
	}

	require.Equal(t, "hello\npart 1\npart 2", req.GetPrompt())
}

func TestOpenAIResponsesRequestGetPrompt(t *testing.T) {
	t.Parallel()

	text := "plain prompt"
	req := &OpenAIResponsesRequest{
		Input: &OpenAIResponseInput{Text: &text},
	}
	require.Equal(t, "plain prompt", req.GetPrompt())

	req = &OpenAIResponsesRequest{
		Input: &OpenAIResponseInput{
			Items: []OpenAIResponseInputItem{
				{
					Role: "developer",
					Content: &OpenAIResponseInputContent{
						Text: &text,
					},
				},
				{
					Role: "user",
					Content: &OpenAIResponseInputContent{
						Parts: []OpenAIResponseInputPart{
							{Type: "input_text", Text: "first"},
							{Type: "input_image", ImageURL: "https://example.com/image.png"},
							{Type: "input_text", Text: "second"},
						},
					},
				},
				{
					Role: "assistant",
					Content: &OpenAIResponseInputContent{
						Text: &text,
					},
				},
			},
		},
	}

	require.Equal(t, "first\nsecond", req.GetPrompt())
}
