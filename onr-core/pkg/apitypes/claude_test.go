package apitypes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeMessageUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		payload    string
		wantString string
		wantTexts  []string
		wantNil    bool
		wantRole   string
	}{
		{
			name:       "string content",
			payload:    `{"role":"user","content":"hello world"}`,
			wantString: "hello world",
			wantRole:   "user",
		},
		{
			name: "array content",
			payload: `{
				"role":"assistant",
				"content":[{"type":"text","text":"chunk one"},{"type":"text","text":"chunk two"}]
			}`,
			wantTexts: []string{"chunk one", "chunk two"},
			wantRole:  "assistant",
		},
		{
			name:     "null content",
			payload:  `{"role":"assistant","content":null}`,
			wantNil:  true,
			wantRole: "assistant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var msg ClaudeMessage
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &msg))
			require.Equal(t, tt.wantRole, msg.Role)

			switch {
			case tt.wantNil:
				require.Nil(t, msg.Content)
			case tt.wantString != "":
				require.Equal(t, tt.wantString, msg.Content)
			case len(tt.wantTexts) > 0:
				blocks, ok := msg.Content.([]ClaudeContent)
				require.True(t, ok)
				require.Len(t, blocks, len(tt.wantTexts))
				for i, want := range tt.wantTexts {
					text, ok := blocks[i].(*ClaudeTextContent)
					require.True(t, ok)
					require.Equal(t, want, text.Text)
				}
			default:
				t.Fatalf("missing expectation for test %q", tt.name)
			}
		})
	}
}

func TestClaudeToolResultContentUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		payload   string
		wantText  string
		wantTexts []string
		wantNil   bool
	}{
		{
			name:     "string content",
			payload:  `{"type":"tool_result","tool_use_id":"tool_1","content":"done"}`,
			wantText: "done",
		},
		{
			name: "structured content",
			payload: `{
				"type":"tool_result",
				"tool_use_id":"tool_2",
				"content":[{"type":"text","text":"pass"}]
			}`,
			wantTexts: []string{"pass"},
		},
		{
			name:    "null content",
			payload: `{"type":"tool_result","tool_use_id":"tool_3","content":null}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var content ClaudeToolResultContent
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &content))

			switch {
			case tt.wantNil:
				require.Nil(t, content.Content)
			case tt.wantText != "":
				require.Equal(t, tt.wantText, content.Content)
			case len(tt.wantTexts) > 0:
				blocks, ok := content.Content.([]ClaudeContent)
				require.True(t, ok)
				require.Len(t, blocks, len(tt.wantTexts))
				for i, want := range tt.wantTexts {
					text, ok := blocks[i].(*ClaudeTextContent)
					require.True(t, ok)
					require.Equal(t, want, text.Text)
				}
			default:
				t.Fatalf("missing expectation for test %q", tt.name)
			}
		})
	}
}

func TestClaudeRequestUnmarshalJSONSystemAndThinking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		payload          string
		wantSystemText   string
		wantSystemBlocks []string
		wantThinkingType string
		wantBudgetTokens int
	}{
		{
			name:           "string system prompt",
			payload:        `{"model":"claude","messages":[],"system":"Stay helpful"}`,
			wantSystemText: "Stay helpful",
		},
		{
			name: "structured system prompt and thinking config",
			payload: `{
				"model":"claude",
				"messages":[],
				"system":[{"type":"text","text":"Act as a tool"}],
				"thinking":{"type":"enabled","budget_tokens":256,"display":"minimal"}
			}`,
			wantSystemBlocks: []string{"Act as a tool"},
			wantThinkingType: "enabled",
			wantBudgetTokens: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var req ClaudeRequest
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &req))

			switch {
			case tt.wantSystemText != "":
				require.Equal(t, tt.wantSystemText, req.System)
			case len(tt.wantSystemBlocks) > 0:
				blocks, ok := req.System.([]ClaudeTextContent)
				require.True(t, ok)
				require.Len(t, blocks, len(tt.wantSystemBlocks))
				for i, want := range tt.wantSystemBlocks {
					require.Equal(t, want, blocks[i].Text)
				}
			default:
				require.Nil(t, req.System)
			}

			switch tt.wantThinkingType {
			case "":
				require.Nil(t, req.Thinking)
			case "enabled":
				require.NotNil(t, req.Thinking)
				cfg, ok := req.Thinking.Data.(*ThinkingConfigEnabled)
				require.True(t, ok)
				require.Equal(t, tt.wantBudgetTokens, cfg.BudgetTokens)
			default:
				t.Fatalf("unexpected thinking type expectation %q", tt.wantThinkingType)
			}
		})
	}
}

func TestThinkingConfigMarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      ThinkingConfig
		wantJSON string
	}{
		{
			name:     "nil data marshals to null",
			cfg:      ThinkingConfig{},
			wantJSON: "null",
		},
		{
			name: "enabled thinking marshals underlying struct",
			cfg: ThinkingConfig{
				Data: &ThinkingConfigEnabled{
					BaseThinkingConfig: BaseThinkingConfig{Type: "enabled"},
					BudgetTokens:       512,
					Display:            "minimal",
				},
			},
			wantJSON: `{"type":"enabled","budget_tokens":512,"display":"minimal"}`,
		},
		{
			name: "adaptive thinking marshals underlying struct",
			cfg: ThinkingConfig{
				Data: &ThinkingConfigAdaptive{
					BaseThinkingConfig: BaseThinkingConfig{Type: "adaptive"},
					Display:            "low",
				},
			},
			wantJSON: `{"type":"adaptive","display":"low"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw, err := json.Marshal(tt.cfg)
			require.NoError(t, err)
			require.JSONEq(t, tt.wantJSON, string(raw))
		})
	}
}

func TestThinkingConfigUnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		payload       string
		wantType      string
		wantBudget    int
		wantDisplay   string
		expectNilData bool
		expectErr     string
	}{
		{
			name:          "null payload clears data",
			payload:       "null",
			expectNilData: true,
		},
		{
			name:        "enabled config unmarshals correctly",
			payload:     `{"type":"enabled","budget_tokens":1024,"display":"minimal"}`,
			wantType:    "enabled",
			wantBudget:  1024,
			wantDisplay: "minimal",
		},
		{
			name:        "adaptive config unmarshals correctly",
			payload:     `{"type":"adaptive","display":"low"}`,
			wantType:    "adaptive",
			wantDisplay: "low",
		},
		{
			name:     "disabled config unmarshals correctly",
			payload:  `{"type":"disabled"}`,
			wantType: "disabled",
		},
		{
			name:      "unknown config returns error",
			payload:   `{"type":"future"}`,
			expectErr: "unsupported thinking config type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg ThinkingConfig
			err := json.Unmarshal([]byte(tt.payload), &cfg)
			if tt.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectErr)
				return
			}

			require.NoError(t, err)
			if tt.expectNilData {
				require.Nil(t, cfg.Data)
				return
			}

			require.NotNil(t, cfg.Data)
			switch typed := cfg.Data.(type) {
			case *ThinkingConfigEnabled:
				require.Equal(t, tt.wantType, typed.GetType())
				require.Equal(t, tt.wantBudget, typed.BudgetTokens)
				require.Equal(t, tt.wantDisplay, typed.Display)
			case *ThinkingConfigAdaptive:
				require.Equal(t, tt.wantType, typed.GetType())
				require.Equal(t, tt.wantDisplay, typed.Display)
			case *ThinkingConfigDisabled:
				require.Equal(t, tt.wantType, typed.GetType())
			default:
				t.Fatalf("unexpected config type %T", typed)
			}
		})
	}
}

func TestClaudeRequestFromMap(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"model": "claude-3-7-sonnet",
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "hello",
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "hi"},
				},
			},
		},
		"system": []any{
			map[string]any{"type": "text", "text": "act carefully"},
		},
		"max_tokens": float64(128),
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": float64(64),
			"display":       "minimal",
		},
		"metadata": map[string]any{
			"user_id": "user-1",
		},
		"tools": []any{
			map[string]any{
				"name": "lookup",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []any{"query"},
				},
			},
		},
	}

	var req ClaudeRequest
	require.NoError(t, req.FromMap(input))
	require.Equal(t, "claude-3-7-sonnet", req.Model)
	require.NotNil(t, req.MaxTokens)
	require.Equal(t, 128, *req.MaxTokens)
	require.NotNil(t, req.Metadata)
	require.Equal(t, "user-1", req.Metadata.UserId)
	require.Len(t, req.Messages, 2)
	require.Equal(t, "hello", req.Messages[0].Content)

	assistantBlocks, ok := req.Messages[1].Content.([]ClaudeContent)
	require.True(t, ok)
	require.Len(t, assistantBlocks, 1)
	assistantText, ok := assistantBlocks[0].(*ClaudeTextContent)
	require.True(t, ok)
	require.Equal(t, "hi", assistantText.Text)

	systemBlocks, ok := req.System.([]ClaudeTextContent)
	require.True(t, ok)
	require.Len(t, systemBlocks, 1)
	require.Equal(t, "act carefully", systemBlocks[0].Text)

	require.NotNil(t, req.Thinking)
	thinking, ok := req.Thinking.Data.(*ThinkingConfigEnabled)
	require.True(t, ok)
	require.Equal(t, 64, thinking.BudgetTokens)
	require.Equal(t, "minimal", thinking.Display)

	require.Len(t, req.Tools, 1)
	require.NotNil(t, req.Tools[0].InputSchema)
	require.Equal(t, "object", req.Tools[0].InputSchema.Type)
}

func TestClaudeRequestToMap(t *testing.T) {
	t.Parallel()

	maxTokens := 256
	stream := true
	req := ClaudeRequest{
		Model: "claude-3-7-sonnet",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
			{
				Role: "assistant",
				Content: []ClaudeContent{
					&ClaudeTextContent{
						ClaudeBaseContent: ClaudeBaseContent{Type: "text"},
						Text:              "hi",
					},
				},
			},
		},
		System: []ClaudeTextContent{
			{
				ClaudeBaseContent: ClaudeBaseContent{Type: "text"},
				Text:              "system prompt",
			},
		},
		MaxTokens: &maxTokens,
		Stream:    &stream,
		Thinking: &ThinkingConfig{
			Data: &ThinkingConfigEnabled{
				BaseThinkingConfig: BaseThinkingConfig{Type: "enabled"},
				BudgetTokens:       32,
				Display:            "minimal",
			},
		},
	}

	got, err := req.ToMap()
	require.NoError(t, err)
	require.Equal(t, "claude-3-7-sonnet", got["model"])
	require.Equal(t, 256, got["max_tokens"])
	require.Equal(t, true, got["stream"])

	messages, ok := got["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	firstMessage, ok := messages[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user", firstMessage["role"])
	require.Equal(t, "hello", firstMessage["content"])

	secondMessage, ok := messages[1].(map[string]any)
	require.True(t, ok)
	contentBlocks, ok := secondMessage["content"].([]any)
	require.True(t, ok)
	require.Len(t, contentBlocks, 1)
	firstBlock, ok := contentBlocks[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "text", firstBlock["type"])
	require.Equal(t, "hi", firstBlock["text"])

	systemBlocks, ok := got["system"].([]any)
	require.True(t, ok)
	require.Len(t, systemBlocks, 1)
	systemBlock, ok := systemBlocks[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "system prompt", systemBlock["text"])

	thinking, ok := got["thinking"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "enabled", thinking["type"])
	require.Equal(t, 32, thinking["budget_tokens"])
}

func TestClaudeRequestGetPrompt(t *testing.T) {
	t.Parallel()

	req := &ClaudeRequest{
		Messages: []ClaudeMessage{
			{Role: "assistant", Content: "ignore"},
			{Role: "user", Content: "hello"},
			{
				Role: "user",
				Content: []ClaudeContent{
					&ClaudeTextContent{
						ClaudeBaseContent: ClaudeBaseContent{Type: "text"},
						Text:              "world",
					},
					&ClaudeToolUseContent{
						ClaudeBaseContent: ClaudeBaseContent{Type: "tool_use"},
						Name:              "lookup",
					},
				},
			},
		},
	}

	require.Equal(t, "hello\nworld", req.GetPrompt())
}

func TestClaudeResponseFromMap(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"id":          "msg_1",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-3-7-sonnet",
		"stop_reason": "end_turn",
		"content": []any{
			map[string]any{"type": "text", "text": "done"},
			map[string]any{
				"type": "tool_use",
				"id":   "toolu_1",
				"name": "lookup",
				"input": map[string]any{
					"query": "weather",
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  float64(10),
			"output_tokens": float64(20),
		},
		"error": map[string]any{
			"type":    "api_error",
			"message": "example",
		},
	}

	var resp ClaudeResponse
	require.NoError(t, resp.FromMap(input))
	require.Equal(t, "msg_1", resp.Id)
	require.Equal(t, "assistant", resp.Role)
	require.Len(t, resp.Content, 2)

	textBlock, ok := resp.Content[0].(*ClaudeTextContent)
	require.True(t, ok)
	require.Equal(t, "done", textBlock.Text)

	toolUse, ok := resp.Content[1].(*ClaudeToolUseContent)
	require.True(t, ok)
	require.Equal(t, "toolu_1", toolUse.Id)
	require.Equal(t, "lookup", toolUse.Name)
	require.Equal(t, "weather", toolUse.Input["query"])

	require.NotNil(t, resp.Usage)
	require.Equal(t, 10, resp.Usage.InputTokens)
	require.Equal(t, 20, resp.Usage.OutputTokens)
	require.NotNil(t, resp.Error)
	require.Equal(t, "api_error", resp.Error.Type)
}

func TestClaudeResponseToMap(t *testing.T) {
	t.Parallel()

	resp := ClaudeResponse{
		Id:         "msg_1",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-7-sonnet",
		StopReason: "end_turn",
		Content: []ClaudeContent{
			&ClaudeTextContent{
				ClaudeBaseContent: ClaudeBaseContent{Type: "text"},
				Text:              "done",
			},
			&ClaudeToolUseContent{
				ClaudeBaseContent: ClaudeBaseContent{Type: "tool_use"},
				Id:                "toolu_1",
				Name:              "lookup",
				Input: map[string]any{
					"query": "weather",
				},
			},
		},
		Usage: &ClaudeUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
		Error: &ClaudeError{
			Type:    "api_error",
			Message: "example",
		},
	}

	got, err := resp.ToMap()
	require.NoError(t, err)
	require.Equal(t, "msg_1", got["id"])
	require.Equal(t, "assistant", got["role"])
	require.Equal(t, "end_turn", got["stop_reason"])

	content, ok := got["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)

	textBlock, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "text", textBlock["type"])
	require.Equal(t, "done", textBlock["text"])

	toolUse, ok := content[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_use", toolUse["type"])
	require.Equal(t, "toolu_1", toolUse["id"])
	require.Equal(t, "lookup", toolUse["name"])
	input, ok := toolUse["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "weather", input["query"])

	usage, ok := got["usage"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 10, usage["input_tokens"])
	require.Equal(t, 20, usage["output_tokens"])

	errMap, ok := got["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "api_error", errMap["type"])
	require.Equal(t, "example", errMap["message"])
}

func TestClaudeUsageIterationsMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"input_tokens":  float64(10),
		"output_tokens": float64(2),
		"cache_creation": map[string]any{
			"ephemeral_5m_input_tokens": float64(3),
			"ephemeral_1h_input_tokens": float64(4),
		},
		"iterations": []any{
			map[string]any{"type": "message", "model": "primary", "input_tokens": float64(10), "output_tokens": float64(0)},
			map[string]any{"type": "fallback_message", "model": "fallback", "input_tokens": float64(10), "output_tokens": float64(2), "cache_creation": map[string]any{"ephemeral_5m_input_tokens": float64(1)}},
		},
	}

	var usage ClaudeUsage
	require.NoError(t, usage.FromMap(input))
	require.Len(t, usage.Iterations, 2)
	require.Equal(t, "primary", usage.Iterations[0].Model)
	require.Equal(t, "fallback", usage.Iterations[1].Model)
	require.Equal(t, 2, usage.Iterations[1].OutputTokens)
	require.NotNil(t, usage.CacheCreation)
	require.Equal(t, 3, usage.CacheCreation.Ephemeral5mInputTokens)
	require.Equal(t, 4, usage.CacheCreation.Ephemeral1hInputTokens)
	require.NotNil(t, usage.Iterations[1].CacheCreation)
	require.Equal(t, 1, usage.Iterations[1].CacheCreation.Ephemeral5mInputTokens)

	got, err := usage.ToMap()
	require.NoError(t, err)
	cacheCreation, ok := got["cache_creation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 3, cacheCreation["ephemeral_5m_input_tokens"])
	require.Equal(t, 4, cacheCreation["ephemeral_1h_input_tokens"])
	items, ok := got["iterations"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	fallback, ok := items[1].(map[string]any)
	require.True(t, ok)
	fallbackCacheCreation, ok := fallback["cache_creation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1, fallbackCacheCreation["ephemeral_5m_input_tokens"])
}

func TestClaudeUsageByModelGetClaudeUsage(t *testing.T) {
	t.Parallel()
	cacheDetail := &CacheCreationUsageDetail{Ephemeral5mInputTokens: 2, Ephemeral1hInputTokens: 4}
	u := ClaudeUsageByModel{
		Type:                     "fallback_message",
		Model:                    "claude-fallback",
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 10,
		CacheReadInputTokens:     5,
		CacheCreation:            cacheDetail,
	}
	got := u.GetClaudeUsage()
	require.Equal(t, u.InputTokens, got.InputTokens)
	require.Equal(t, u.OutputTokens, got.OutputTokens)
	require.Equal(t, u.CacheCreationInputTokens, got.CacheCreationInputTokens)
	require.Equal(t, u.CacheReadInputTokens, got.CacheReadInputTokens)
	require.Equal(t, cacheDetail, got.CacheCreation)
	require.Nil(t, got.Iterations)
}

func TestClaudeRequestFallbacksMapRoundTrip(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"model":    "claude-primary",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"fallbacks": []any{
			map[string]any{"model": "claude-fallback", "max_tokens": float64(128)},
		},
	}

	var req ClaudeRequest
	require.NoError(t, req.FromMap(input))
	require.Len(t, req.Fallbacks, 1)
	require.Equal(t, "claude-fallback", req.Fallbacks[0].Model)
	require.NotNil(t, req.Fallbacks[0].MaxTokens)
	require.Equal(t, 128, *req.Fallbacks[0].MaxTokens)

	got, err := req.ToMap()
	require.NoError(t, err)
	items, ok := got["fallbacks"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
}
