package apitransform

import (
	"fmt"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

const finishReasonLength = "length"

// MapOpenAIResponsesToChatCompletions maps an OpenAI Responses API JSON response into an
// OpenAI Chat Completions JSON response (best-effort).
//
// This is primarily meant for "legacy client -> /v1/chat/completions" compatibility when the upstream only
// supports "/v1/responses" (or Azure "/openai/responses").
func MapOpenAIResponsesToChatCompletions(respBody []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(respBody, "responses")
	if err != nil {
		return nil, err
	}
	out, err := MapOpenAIResponsesToChatCompletionsObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapOpenAIResponsesToChatCompletionsObject maps responses object payload to chat.completions object payload.
func MapOpenAIResponsesToChatCompletionsObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	outObj := mapOpenAIResponsesObjectToChat(root)
	if outObj == nil {
		return nil, fmt.Errorf("responses json is not an object")
	}
	return apitypes.JSONObject(outObj), nil
}

func mapOpenAIResponsesObjectToChat(root map[string]any) map[string]any {
	if root == nil {
		return nil
	}
	// Some SSE events wrap the final response in {"response":{...}}.
	if inner, ok := root["response"].(map[string]any); ok && inner != nil {
		root = inner
	}

	id := strings.TrimSpace(jsonutil.CoerceString(root["id"]))
	if id == "" {
		id = "chatcmpl_" + fmt.Sprintf("%d", time.Now().UnixNano())
	} else if !strings.HasPrefix(id, "chatcmpl_") {
		id = "chatcmpl_" + id
	}

	created := coerceInt64(root["created_at"])
	if created <= 0 {
		created = time.Now().Unix()
	}
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))

	contentText, toolCalls := extractResponsesOutput(root)
	finishReason := mapResponsesFinishReason(root)
	if len(toolCalls) > 0 && strings.TrimSpace(contentText) == "" {
		finishReason = "tool_calls"
	}
	usage := mapResponsesUsageToChat(root)

	msg := map[string]any{
		"role": "assistant",
	}
	// Keep it a string for compatibility. When only tool_calls exist, prefer empty content (new-api aligned).
	msg["content"] = contentText
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
		if strings.TrimSpace(contentText) == "" {
			msg["content"] = ""
		}
	}

	out := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
	}
	if model != "" {
		out["model"] = model
	}
	out["choices"] = []any{
		map[string]any{
			"index":   0,
			"message": msg,
			// Keep "finish_reason" aligned with OpenAI-style chat responses.
			"finish_reason": finishReason,
		},
	}
	if usage != nil {
		out["usage"] = usage
	}
	return out
}

func extractResponsesOutput(root map[string]any) (text string, toolCalls []any) {
	// Fast path if server provides a convenience field.
	if s := strings.TrimSpace(jsonutil.CoerceString(root["output_text"])); s != "" {
		text = s
	}

	output, _ := root["output"].([]any)
	for _, item := range output {
		m, ok := item.(map[string]any)
		if !ok || m == nil {
			continue
		}
		typ := strings.TrimSpace(jsonutil.CoerceString(m["type"]))
		switch typ {
		case "message":
			role := strings.TrimSpace(jsonutil.CoerceString(m["role"]))
			if role != "" && role != "assistant" {
				continue
			}
			if s := extractResponsesMessageText(m); s != "" {
				// Prefer message content over output_text convenience.
				text = s
			}
		case "function_call":
			toolCall := mapResponsesFunctionCallToToolCall(m)
			if toolCall != nil {
				toolCalls = append(toolCalls, toolCall)
			}
		}
	}
	return text, toolCalls
}

func extractResponsesMessageText(msg map[string]any) string {
	contents, _ := msg["content"].([]any)
	if len(contents) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range contents {
		cm, ok := c.(map[string]any)
		if !ok || cm == nil {
			continue
		}
		typ := strings.TrimSpace(jsonutil.CoerceString(cm["type"]))
		switch typ {
		case "output_text", "text":
			if s := jsonutil.CoerceString(cm["text"]); s != "" {
				b.WriteString(s)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func mapResponsesFunctionCallToToolCall(fc map[string]any) map[string]any {
	name := strings.TrimSpace(jsonutil.CoerceString(fc["name"]))
	args := strings.TrimSpace(jsonutil.CoerceString(fc["arguments"]))
	if name == "" {
		return nil
	}
	id := strings.TrimSpace(jsonutil.CoerceString(fc["call_id"]))
	if id == "" {
		id = strings.TrimSpace(jsonutil.CoerceString(fc["id"]))
	}
	if id == "" {
		id = "call_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return map[string]any{
		"id":   id,
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
}

func mapResponsesFinishReason(root map[string]any) string {
	status := strings.ToLower(strings.TrimSpace(jsonutil.CoerceString(root["status"])))
	// Best-effort: map incomplete reasons (if present) to chat finish_reason.
	if inc, ok := root["incomplete_details"].(map[string]any); ok && inc != nil {
		reason := strings.ToLower(strings.TrimSpace(jsonutil.CoerceString(inc["reason"])))
		switch reason {
		case "max_output_tokens", finishReasonLength:
			return finishReasonLength
		case finishContentFilter:
			return finishContentFilter
		}
	}
	switch status {
	case "completed", "":
		return finishReasonStop
	case "incomplete":
		return finishReasonLength
	default:
		// Keep it conservative.
		return finishReasonStop
	}
}

func mapResponsesUsageToChat(root map[string]any) map[string]any {
	u, _ := root["usage"].(map[string]any)
	if u == nil {
		return nil
	}
	inputTokens := jsonutil.FirstInt(
		jsonutil.GetIntByPath(u, "$.prompt_tokens"),
		jsonutil.GetIntByPath(u, "$.input_tokens"),
	)
	outputTokens := jsonutil.FirstInt(
		jsonutil.GetIntByPath(u, "$.completion_tokens"),
		jsonutil.GetIntByPath(u, "$.output_tokens"),
	)
	totalTokens := jsonutil.FirstInt(
		jsonutil.GetIntByPath(u, "$.total_tokens"),
		inputTokens+outputTokens,
	)

	// Provide both the legacy (prompt/completion) and the newer (input/output) fields for compatibility.
	out := map[string]any{
		"prompt_tokens":     inputTokens,
		"completion_tokens": outputTokens,
		"total_tokens":      totalTokens,
		"input_tokens":      inputTokens,
		"output_tokens":     outputTokens,
	}
	// Pass through cached tokens when present.
	if v := jsonutil.FirstInt(
		jsonutil.GetIntByPath(u, "$.prompt_tokens_details.cached_tokens"),
		jsonutil.GetIntByPath(u, "$.input_tokens_details.cached_tokens"),
		jsonutil.GetIntByPath(u, "$.cached_tokens"),
	); v > 0 {
		out["prompt_tokens_details"] = map[string]any{"cached_tokens": v}
		out["input_tokens_details"] = map[string]any{"cached_tokens": v}
	}
	return out
}

func coerceInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	default:
		return 0
	}
}
