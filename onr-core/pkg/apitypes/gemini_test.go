package apitypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiChatRequestGetPrompt(t *testing.T) {
	t.Parallel()

	req := &ChatRequest{
		Contents: []ChatContent{
			{
				Role: "user",
				Parts: []Part{
					{Text: "hello"},
					{Text: " world"},
				},
			},
			{
				Role: "assistant",
				Parts: []Part{
					{Text: "ignored"},
				},
			},
		},
		SystemInstruction: &ChatContent{
			Parts: []Part{
				{Text: "system"},
			},
		},
	}

	require.Equal(t, "hello worldsystem", req.GetPrompt())
}

func TestGeminiChatRequestFromMap(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{"text": "hello"},
					map[string]any{
						"inlineData": map[string]any{
							"mimeType": "image/png",
							"data":     "abc",
						},
						"mediaResolution": MediaResolutionHigh,
					},
				},
			},
		},
		"safety_settings": []any{
			map[string]any{
				"category":  "HARM_CATEGORY_HATE_SPEECH",
				"threshold": "BLOCK_NONE",
			},
		},
		"generationConfig": map[string]any{
			"responseMimeType":   "application/json",
			"responseModalities": []any{"TEXT", "IMAGE"},
			"thinkingConfig": map[string]any{
				"thinkingLevel":   "high",
				"thinkingBudget":  float64(128),
				"includeThoughts": true,
			},
			"temperature":     0.5,
			"topP":            0.9,
			"topK":            40.0,
			"maxOutputTokens": float64(512),
			"candidateCount":  float64(2),
			"stopSequences":   []any{"END"},
		},
		"tools": []any{
			map[string]any{
				"function_declarations": []any{
					map[string]any{"name": "lookup"},
				},
			},
		},
		"system_instruction": map[string]any{
			"parts": []any{
				map[string]any{"text": "follow policy"},
			},
		},
	}

	var req ChatRequest
	require.NoError(t, req.FromMap(input))
	require.Len(t, req.Contents, 1)
	require.Equal(t, "user", req.Contents[0].Role)
	require.Len(t, req.Contents[0].Parts, 2)
	require.Equal(t, "hello", req.Contents[0].Parts[0].Text)
	require.NotNil(t, req.Contents[0].Parts[1].InlineData)
	require.Equal(t, "image/png", req.Contents[0].Parts[1].InlineData.MimeType)
	require.Equal(t, MediaResolutionHigh, req.Contents[0].Parts[1].MediaResolution)
	require.Len(t, req.SafetySettings, 1)
	require.Equal(t, "BLOCK_NONE", req.SafetySettings[0].Threshold)
	require.Equal(t, "application/json", req.GenerationConfig.ResponseMimeType)
	require.NotNil(t, req.GenerationConfig.ThinkingConfig)
	require.Equal(t, "high", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
	require.NotNil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, 128, *req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.NotNil(t, req.GenerationConfig.ThinkingConfig.IncludeThoughts)
	require.True(t, *req.GenerationConfig.ThinkingConfig.IncludeThoughts)
	require.Len(t, req.Tools, 1)
	require.NotNil(t, req.SystemInstruction)
	require.Equal(t, "follow policy", req.SystemInstruction.Parts[0].Text)
}

func TestGeminiChatRequestToMap(t *testing.T) {
	t.Parallel()

	thinkingBudget := 64
	includeThoughts := true
	temperature := 0.2
	topP := 0.8
	req := &ChatRequest{
		Contents: []ChatContent{
			{
				Role: "user",
				Parts: []Part{
					{Text: "hello"},
					{
						FunctionCall: &FunctionCall{
							FunctionName: "lookup",
							Arguments: map[string]any{
								"query": "weather",
							},
						},
					},
				},
			},
		},
		SafetySettings: []ChatSafetySettings{
			{
				Category:  "HARM_CATEGORY_HARASSMENT",
				Threshold: "BLOCK_NONE",
			},
		},
		GenerationConfig: ChatGenerationConfig{
			ResponseMimeType: "application/json",
			ThinkingConfig: &GeminiThinkingConfig{
				ThinkingLevel:   "medium",
				ThinkingBudget:  &thinkingBudget,
				IncludeThoughts: &includeThoughts,
			},
			Temperature: &temperature,
			TopP:        &topP,
		},
		SystemInstruction: &ChatContent{
			Parts: []Part{{Text: "follow policy"}},
		},
	}

	got, err := req.ToMap()
	require.NoError(t, err)

	contents, ok := got["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 1)
	content, ok := contents[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user", content["role"])

	parts, ok := content["parts"].([]any)
	require.True(t, ok)
	require.Len(t, parts, 2)
	secondPart, ok := parts[1].(map[string]any)
	require.True(t, ok)
	functionCall, ok := secondPart["functionCall"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "lookup", functionCall["name"])

	generationConfig, ok := got["generationConfig"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "application/json", generationConfig["responseMimeType"])
	thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "medium", thinkingConfig["thinkingLevel"])
	require.Equal(t, 64, thinkingConfig["thinkingBudget"])
	require.Equal(t, true, thinkingConfig["includeThoughts"])
}

func TestGeminiEmbeddingResponseMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"embeddings": []any{
			map[string]any{
				"values": []any{1.5, 2.5, 3.5},
			},
		},
		"error": map[string]any{
			"code":    float64(400),
			"message": "bad request",
			"status":  "INVALID_ARGUMENT",
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(10),
			"candidatesTokenCount": float64(20),
			"totalTokenCount":      float64(30),
			"promptTokensDetails": []any{
				map[string]any{
					"modality":   "TEXT",
					"tokenCount": float64(10),
				},
			},
		},
	}

	var resp EmbeddingResponse
	require.NoError(t, resp.FromMap(input))
	require.Len(t, resp.Embeddings, 1)
	require.Equal(t, []float64{1.5, 2.5, 3.5}, resp.Embeddings[0].Values)
	require.NotNil(t, resp.Error)
	require.Equal(t, 400, resp.Error.Code)
	require.NotNil(t, resp.UsageMetadata)
	require.Equal(t, 30, resp.UsageMetadata.TotalTokenCount)
	require.Len(t, resp.UsageMetadata.PromptTokensDetails, 1)
	require.Equal(t, "TEXT", resp.UsageMetadata.PromptTokensDetails[0].Modality)

	got, err := resp.ToMap()
	require.NoError(t, err)
	embeddings, ok := got["embeddings"].([]any)
	require.True(t, ok)
	require.Len(t, embeddings, 1)
	errMap, ok := got["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 400, errMap["code"])
	usageMetadata, ok := got["usageMetadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 30, usageMetadata["totalTokenCount"])
}
