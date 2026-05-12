package apitransform

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

const (
	claudeContentTypeToolUse = "tool_use"
	finishReasonToolCalls    = "tool_calls"
	claudeStopReasonMax      = "max_tokens"
)

// MapOpenAIChatCompletionsToClaudeMessagesRequestObject maps OpenAI chat request object to Claude messages request object.
func MapOpenAIChatCompletionsToClaudeMessagesRequestObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	out := apitypes.JSONObject{
		"model": model,
	}
	maxTokens := jsonutil.CoerceInt(root["max_tokens"])
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	out["max_tokens"] = maxTokens
	if stream, ok := root["stream"].(bool); ok {
		out["stream"] = stream
	}
	if v, ok := root["temperature"].(float64); ok {
		out["temperature"] = v
	}
	if v, ok := root["top_p"].(float64); ok {
		out["top_p"] = v
	}

	messages, _ := root["messages"].([]any)
	claudeMessages := make([]any, 0, len(messages))
	systemParts := make([]string, 0, 2)
	for _, raw := range messages {
		msg, _ := raw.(map[string]any)
		if msg == nil {
			continue
		}
		role := strings.TrimSpace(jsonutil.CoerceString(msg["role"]))
		if role == openAIRoleSystem {
			if c := strings.TrimSpace(jsonutil.CoerceString(msg["content"])); c != "" {
				systemParts = append(systemParts, c)
			}
			continue
		}
		if role == "tool" {
			claudeMessages = append(claudeMessages, openAIToolMessageToClaudeUser(msg))
			continue
		}
		claudeMessages = append(claudeMessages, openAIMessageToClaudeMessage(msg))
	}
	if len(systemParts) > 0 {
		out["system"] = strings.Join(systemParts, "\n")
	}
	out["messages"] = claudeMessages

	if tools, ok := mapOpenAIToolsToClaude(root["tools"]); ok {
		out["tools"] = tools
	}
	return out, nil
}

func openAIToolMessageToClaudeUser(msg map[string]any) apitypes.JSONObject {
	callID := strings.TrimSpace(jsonutil.CoerceString(msg["tool_call_id"]))
	return apitypes.JSONObject{
		"role": "user",
		"content": []any{
			apitypes.JSONObject{
				"type":        "tool_result",
				"tool_use_id": callID,
				"content":     jsonutil.CoerceString(msg["content"]),
			},
		},
	}
}

func openAIMessageToClaudeMessage(msg map[string]any) apitypes.JSONObject {
	role := strings.TrimSpace(jsonutil.CoerceString(msg["role"]))
	if role == "" {
		role = "user"
	}
	content := make([]any, 0, 2)
	if text := strings.TrimSpace(jsonutil.CoerceString(msg["content"])); text != "" {
		content = append(content, apitypes.JSONObject{"type": chatContentTypeText, "text": text})
	}
	if toolCalls, _ := msg["tool_calls"].([]any); len(toolCalls) > 0 {
		for _, raw := range toolCalls {
			tc, _ := raw.(map[string]any)
			if tc == nil {
				continue
			}
			fn, _ := tc["function"].(map[string]any)
			name := strings.TrimSpace(jsonutil.CoerceString(fn["name"]))
			if name == "" {
				continue
			}
			input := apitypes.JSONObject{}
			if args := strings.TrimSpace(jsonutil.CoerceString(fn["arguments"])); args != "" {
				var v any
				if err := json.Unmarshal([]byte(args), &v); err == nil {
					if m, ok := v.(map[string]any); ok && m != nil {
						input = m
					}
				}
			}
			content = append(content, apitypes.JSONObject{
				"type":  claudeContentTypeToolUse,
				"id":    strings.TrimSpace(jsonutil.CoerceString(tc["id"])),
				"name":  name,
				"input": input,
			})
		}
	}
	if len(content) == 0 {
		content = append(content, apitypes.JSONObject{"type": chatContentTypeText, "text": ""})
	}
	return apitypes.JSONObject{
		"role":    role,
		"content": content,
	}
}

func mapOpenAIToolsToClaude(rawTools any) ([]any, bool) {
	tools, _ := rawTools.([]any)
	if len(tools) == 0 {
		return nil, false
	}
	out := make([]any, 0, len(tools))
	for _, raw := range tools {
		tm, _ := raw.(map[string]any)
		if tm == nil {
			continue
		}
		if strings.TrimSpace(jsonutil.CoerceString(tm["type"])) != chatRoleFunction {
			continue
		}
		fn, _ := tm["function"].(map[string]any)
		name := strings.TrimSpace(jsonutil.CoerceString(fn["name"]))
		if name == "" {
			continue
		}
		inputSchema, _ := fn["parameters"].(map[string]any)
		if inputSchema == nil {
			inputSchema = apitypes.JSONObject{"type": "object", "properties": apitypes.JSONObject{}}
		}
		out = append(out, apitypes.JSONObject{
			"name":         name,
			"description":  strings.TrimSpace(jsonutil.CoerceString(fn["description"])),
			"input_schema": inputSchema,
		})
	}
	return out, len(out) > 0
}

// MapClaudeMessagesResponseToOpenAIChatCompletions maps Claude messages response JSON
// to OpenAI chat.completions response JSON.
func MapClaudeMessagesResponseToOpenAIChatCompletions(body []byte) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("parse json object: %w", err)
	}
	out, err := MapClaudeMessagesResponseToOpenAIChatCompletionsObject(obj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// MapClaudeMessagesResponseToOpenAIChatCompletionsObject maps Claude messages response object
// to OpenAI chat.completions response object.
// The mapping intentionally emits a single OpenAI choice and aggregates supported Claude
// content blocks (text/thinking + tool_use metadata) into one assistant message.
func MapClaudeMessagesResponseToOpenAIChatCompletionsObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	var src apitypes.ClaudeResponse
	if err := src.FromMap(root); err != nil {
		return nil, err
	}
	toolCalls := make([]any, 0)
	contentParts := make([]string, 0)
	for _, content := range src.Content {
		if content == nil {
			continue
		}
		switch v := content.(type) {
		case *apitypes.ClaudeToolUseContent:
			args, _ := json.Marshal(v.Input)
			toolCalls = append(toolCalls, apitypes.JSONObject{
				"id":   v.Id,
				"type": chatRoleFunction,
				"function": apitypes.JSONObject{
					"name":      v.Name,
					"arguments": string(args),
				},
			})
		case *apitypes.ClaudeTextContent:
			if v.Text == "" {
				continue
			}
			contentParts = append(contentParts, v.Text)
		case *apitypes.ClaudeThinkingContent:
			if v.Thinking == "" {
				continue
			}
			contentParts = append(contentParts, v.Thinking)
		}
	}
	if len(contentParts) == 0 && len(toolCalls) == 0 {
		return nil, fmt.Errorf("content is required")
	}
	message := apitypes.JSONObject{
		"role": openAIRoleAssistant,
	}
	if len(contentParts) > 0 {
		message["content"] = strings.Join(contentParts, "")
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	choices := []any{apitypes.JSONObject{
		"index":         0,
		"message":       message,
		"finish_reason": stopReasonClaude2OpenAI(src.StopReason),
	}}

	out := apitypes.JSONObject{
		"id":      normalizeChatCompletionID(src.Id),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   src.Model,
		"choices": choices,
	}
	if src.Usage != nil {
		usage, err := mapClaudeUsageToOpenAIChatUsage(src.Usage)
		if err != nil {
			return nil, err
		}
		out["usage"] = usage
	}
	return out, nil
}

func mapClaudeUsageToOpenAIChatUsage(raw *apitypes.ClaudeUsage) (apitypes.JSONObject, error) {
	promptTokens := raw.InputTokens + raw.CacheCreationInputTokens + raw.CacheReadInputTokens
	u := &apitypes.OpenAIChatCompletionsUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: raw.OutputTokens,
		TotalTokens:      promptTokens + raw.OutputTokens,
	}
	if raw.CacheReadInputTokens > 0 || raw.CacheCreationInputTokens > 0 {
		u.PromptTokensDetails = &apitypes.OpenAITokenDetails{
			CachedTokens:     raw.CacheReadInputTokens,
			CacheWriteTokens: raw.CacheCreationInputTokens,
		}
	}
	out, err := u.ToMap()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func stopReasonClaude2OpenAI(reason string) string {
	switch strings.TrimSpace(reason) {
	case "end_turn", "stop_sequence":
		return finishReasonStop
	case claudeStopReasonMax:
		return finishReasonLength
	case claudeContentTypeToolUse:
		return finishReasonToolCalls
	default:
		return strings.TrimSpace(reason)
	}
}

func normalizeChatCompletionID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "chatcmpl_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if strings.HasPrefix(id, "chatcmpl_") {
		return id
	}
	return "chatcmpl_" + id
}

// MapClaudeMessagesToOpenAIChatCompletionsObject maps Claude messages object to OpenAI chat request object.
func MapClaudeMessagesToOpenAIChatCompletionsObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	out := apitypes.JSONObject{
		"model": model,
	}
	if s, ok := root["stream"].(bool); ok {
		out["stream"] = s
	}
	if v := jsonutil.CoerceInt(root["max_tokens"]); v > 0 {
		out["max_tokens"] = v
	}

	openAIMessages := mapClaudeMessages(root["messages"])
	openAIMessages = prependClaudeSystemMessages(root["system"], openAIMessages)

	out["messages"] = openAIMessages
	return out, nil
}

func mapClaudeMessages(rawMessages any) []any {
	messages, _ := rawMessages.([]any)
	out := make([]any, 0, len(messages)+1)
	for _, raw := range messages {
		msg, _ := raw.(map[string]any)
		if msg == nil {
			continue
		}
		items := mapOneClaudeMessage(msg)
		out = append(out, items...)
	}
	return out
}

func mapOneClaudeMessage(msg map[string]any) []any {
	role := strings.TrimSpace(jsonutil.CoerceString(msg["role"]))
	if role == "" {
		return nil
	}
	content := msg["content"]
	parts, isArray := content.([]any)
	if !isArray {
		return []any{apitypes.JSONObject{"role": role, "content": content}}
	}

	textParts := make([]string, 0, len(parts))
	toolCalls := make([]any, 0, 2)
	toolMessages := make([]any, 0, 2)
	for _, p := range parts {
		pm, _ := p.(map[string]any)
		if pm == nil {
			continue
		}
		if text, ok := claudeTextPart(pm); ok {
			textParts = append(textParts, text)
			continue
		}
		if toolCall, ok := claudeToolUsePart(pm); ok {
			toolCalls = append(toolCalls, toolCall)
			continue
		}
		if toolMsg, ok := claudeToolResultPart(pm); ok {
			toolMessages = append(toolMessages, toolMsg)
		}
	}

	item := apitypes.JSONObject{"role": role, "content": strings.Join(textParts, "\n")}
	if len(toolCalls) > 0 {
		item["tool_calls"] = toolCalls
	}
	return append([]any{item}, toolMessages...)
}

func claudeTextPart(pm map[string]any) (string, bool) {
	if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != chatContentTypeText {
		return "", false
	}
	t := strings.TrimSpace(jsonutil.CoerceString(pm["text"]))
	return t, t != ""
}

func claudeToolUsePart(pm map[string]any) (apitypes.JSONObject, bool) {
	if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != claudeContentTypeToolUse {
		return nil, false
	}
	name := strings.TrimSpace(jsonutil.CoerceString(pm["name"]))
	if name == "" {
		return nil, false
	}
	id := strings.TrimSpace(jsonutil.CoerceString(pm["id"]))
	args := "{}"
	if pm["input"] != nil {
		if b, err := json.Marshal(pm["input"]); err == nil {
			args = string(b)
		}
	}
	return apitypes.JSONObject{
		"id":   id,
		"type": chatRoleFunction,
		"function": apitypes.JSONObject{
			"name":      name,
			"arguments": args,
		},
	}, true
}

func claudeToolResultPart(pm map[string]any) (apitypes.JSONObject, bool) {
	if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != "tool_result" {
		return nil, false
	}
	callID := strings.TrimSpace(jsonutil.CoerceString(pm["tool_use_id"]))
	if callID == "" {
		return nil, false
	}
	output := jsonutil.CoerceString(pm["content"])
	if output == "" && pm["content"] != nil {
		if b, err := json.Marshal(pm["content"]); err == nil {
			output = string(b)
		}
	}
	return apitypes.JSONObject{
		"role":         "tool",
		"tool_call_id": callID,
		"content":      output,
	}, true
}

func prependClaudeSystemMessages(rawSystem any, openAIMessages []any) []any {
	switch v := rawSystem.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return append([]any{apitypes.JSONObject{"role": "system", "content": v}}, openAIMessages...)
		}
	case []any:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			pm, _ := p.(map[string]any)
			if pm == nil {
				continue
			}
			if t, ok := claudeTextPart(pm); ok {
				parts = append(parts, t)
			}
		}
		if len(parts) > 0 {
			return append([]any{apitypes.JSONObject{"role": "system", "content": strings.Join(parts, "\n")}}, openAIMessages...)
		}
	}
	return openAIMessages
}

// MapOpenAIChatCompletionsToClaudeMessagesResponse maps OpenAI chat response JSON to Claude response JSON.
func MapOpenAIChatCompletionsToClaudeMessagesResponse(body []byte) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("parse json object: %w", err)
	}
	out, err := MapOpenAIChatCompletionsToClaudeMessagesResponseObject(obj)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// MapOpenAIChatCompletionsToClaudeMessagesResponseObject maps OpenAI chat response object to Claude response object.
func MapOpenAIChatCompletionsToClaudeMessagesResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	choices, _ := root["choices"].([]any)
	if len(choices) == 0 {
		return nil, fmt.Errorf("choices is required")
	}
	choice0, _ := choices[0].(map[string]any)
	if choice0 == nil {
		return nil, fmt.Errorf("invalid choices[0]")
	}
	msg, _ := choice0["message"].(map[string]any)
	if msg == nil {
		return nil, fmt.Errorf("invalid choices[0].message")
	}

	content := make([]any, 0, 2)
	toolCalls, _ := msg["tool_calls"].([]any)
	if len(toolCalls) > 0 {
		for _, raw := range toolCalls {
			tc, _ := raw.(map[string]any)
			if tc == nil {
				continue
			}
			fn, _ := tc["function"].(map[string]any)
			name := strings.TrimSpace(jsonutil.CoerceString(fn["name"]))
			if name == "" {
				continue
			}
			input := apitypes.JSONObject{}
			if args := strings.TrimSpace(jsonutil.CoerceString(fn["arguments"])); args != "" {
				var v any
				if err := json.Unmarshal([]byte(args), &v); err == nil {
					if m, ok := v.(map[string]any); ok && m != nil {
						input = m
					} else {
						input["arguments"] = args
					}
				} else {
					input["arguments"] = args
				}
			}
			content = append(content, apitypes.JSONObject{
				"type":  claudeContentTypeToolUse,
				"id":    jsonutil.CoerceString(tc["id"]),
				"name":  name,
				"input": input,
			})
		}
	} else {
		text := jsonutil.CoerceString(msg["content"])
		content = append(content, apitypes.JSONObject{"type": chatContentTypeText, "text": text})
	}

	stopReason := "end_turn"
	switch strings.TrimSpace(jsonutil.CoerceString(choice0["finish_reason"])) {
	case finishReasonLength:
		stopReason = "max_tokens"
	case finishReasonToolCalls:
		stopReason = claudeContentTypeToolUse
	}

	usage := apitypes.JSONObject{}
	if um, _ := root["usage"].(map[string]any); um != nil {
		usage["input_tokens"] = jsonutil.GetFirstIntByPaths(um, "$.prompt_tokens", "$.input_tokens")
		usage["output_tokens"] = jsonutil.GetFirstIntByPaths(um, "$.completion_tokens", "$.output_tokens")
	}

	out := apitypes.JSONObject{
		"id":          jsonutil.CoerceString(root["id"]),
		"type":        "message",
		"role":        "assistant",
		"model":       jsonutil.CoerceString(root["model"]),
		"content":     content,
		"stop_reason": stopReason,
	}
	if len(usage) > 0 {
		out["usage"] = usage
	}
	return out, nil
}
