package apitransform

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

const (
	chatRoleFunction    = "function"
	chatRoleUser        = "user"
	chatContentTypeText = "text"
)

// MapOpenAIChatCompletionsToResponsesRequest converts an OpenAI-style chat.completions request JSON
// into an OpenAI Responses request JSON (best-effort), following new-api's compatibility semantics.
//
// Notes:
// - This function only transforms the JSON payload; it does not alter headers or URL.
// - It is intentionally permissive and keeps unknown fields out (to avoid surprising upstream behavior).
func MapOpenAIChatCompletionsToResponsesRequest(reqBody []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(reqBody, "chat request")
	if err != nil {
		return nil, err
	}

	// If it already looks like a Responses request, do not re-map.
	if _, ok := root["input"]; ok && root["messages"] == nil {
		return reqBody, nil
	}

	out, err := MapOpenAIChatCompletionsToResponsesObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapOpenAIChatCompletionsToResponsesObject converts chat object payload to responses object payload.
func MapOpenAIChatCompletionsToResponsesObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))
	if model == "" {
		return nil, errors.New("model is required")
	}
	if n := jsonutil.CoerceInt(root["n"]); n > 1 {
		return nil, fmt.Errorf("n>1 is not supported in responses compatibility mode")
	}

	inputItems, instructions := mapChatMessagesToResponsesInput(root["messages"])

	out := apitypes.JSONObject{
		"model": model,
		"input": inputItems,
	}
	if strings.TrimSpace(instructions) != "" {
		out["instructions"] = instructions
	}

	copyChatRequestTopLevelToResponses(root, out)
	return out, nil
}

func mapChatMessagesToResponsesInput(messagesAny any) (inputItems []map[string]any, instructions string) {
	messages, _ := messagesAny.([]any)
	instructionsParts := make([]string, 0, 2)
	inputItems = make([]map[string]any, 0, len(messages))

	for _, raw := range messages {
		msg, _ := raw.(map[string]any)
		if msg == nil {
			continue
		}
		role := strings.TrimSpace(jsonutil.CoerceString(msg["role"]))
		if role == "" {
			continue
		}

		switch role {
		case "tool", chatRoleFunction:
			appendToolOutputAsResponsesItem(&inputItems, msg)
		case "system", "developer":
			if s := strings.TrimSpace(extractChatTextFromContent(msg["content"])); s != "" {
				instructionsParts = append(instructionsParts, s)
			}
		default:
			appendChatMessageAsResponsesItem(&inputItems, role, msg)
		}
	}

	instructions = strings.Join(instructionsParts, "\n\n")
	return inputItems, instructions
}

func appendToolOutputAsResponsesItem(dst *[]map[string]any, msg map[string]any) {
	if dst == nil || msg == nil {
		return
	}
	callID := strings.TrimSpace(jsonutil.CoerceString(msg["tool_call_id"]))
	if callID == "" {
		callID = strings.TrimSpace(jsonutil.CoerceString(msg["tool_callid"]))
	}
	output := coerceChatContentToOutputValue(msg["content"])
	if callID == "" {
		*dst = append(*dst, map[string]any{
			"role":    "user",
			"content": fmt.Sprintf("[tool_output_missing_call_id] %v", output),
		})
		return
	}
	*dst = append(*dst, map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  output,
	})
}

func appendChatMessageAsResponsesItem(dst *[]map[string]any, role string, msg map[string]any) {
	if dst == nil || msg == nil {
		return
	}

	item := map[string]any{"role": role}
	item["content"] = mapChatContentToResponsesContent(msg["content"])
	*dst = append(*dst, item)

	if role == "assistant" {
		appendAssistantToolCallsAsInputItems(dst, msg)
	}
}

func mapChatContentToResponsesContent(content any) any {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	if parts, ok := content.([]any); ok {
		return mapChatContentPartsToResponses(parts)
	}
	// Fallback: stringify unknown content shapes.
	return fmt.Sprintf("%v", content)
}

func mapChatContentPartsToResponses(parts []any) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		pm, _ := p.(map[string]any)
		if pm == nil {
			continue
		}
		pt := strings.TrimSpace(jsonutil.CoerceString(pm["type"]))
		switch pt {
		case chatContentTypeText:
			out = append(out, map[string]any{
				"type": "input_text",
				"text": jsonutil.CoerceString(pm["text"]),
			})
		case "image_url":
			out = append(out, map[string]any{
				"type":      "input_image",
				"image_url": normalizeChatImageURLToString(pm["image_url"]),
			})
		case "input_audio":
			out = append(out, map[string]any{
				"type":        "input_audio",
				"input_audio": pm["input_audio"],
			})
		case "file":
			out = append(out, map[string]any{
				"type": "input_file",
				"file": pm["file"],
			})
		case "video_url":
			out = append(out, map[string]any{
				"type":      "input_video",
				"video_url": pm["video_url"],
			})
		default:
			out = append(out, map[string]any{
				"type": pt,
			})
		}
	}
	return out
}

func copyChatRequestTopLevelToResponses(in map[string]any, out map[string]any) {
	if in == nil || out == nil {
		return
	}

	if v, ok := in["stream"].(bool); ok {
		out["stream"] = v
	}
	if v, ok := in["temperature"].(float64); ok {
		out["temperature"] = v
	}
	if v, ok := in["top_p"].(float64); ok && v != 0 {
		out["top_p"] = v
	}
	if s := strings.TrimSpace(jsonutil.CoerceString(in["user"])); s != "" {
		out["user"] = s
	}

	maxOutput := jsonutil.CoerceInt(in["max_tokens"])
	if v := jsonutil.CoerceInt(in["max_completion_tokens"]); v > maxOutput {
		maxOutput = v
	}
	if maxOutput > 0 {
		out["max_output_tokens"] = maxOutput
	}

	if tools, ok := in["tools"].([]any); ok && len(tools) > 0 {
		out["tools"] = mapChatToolsToResponsesTools(tools)
	}
	if tc := in["tool_choice"]; tc != nil {
		if mapped := mapChatToolChoiceToResponses(tc); mapped != nil {
			out["tool_choice"] = mapped
		}
	}
	if v := in["parallel_tool_calls"]; v != nil {
		out["parallel_tool_calls"] = v
	}

	if rf := in["response_format"]; rf != nil {
		out["text"] = map[string]any{
			"format": rf,
		}
	}

	if v, ok := in["store"].(bool); ok {
		out["store"] = v
	}
	if md, ok := in["metadata"].(map[string]any); ok && md != nil {
		out["metadata"] = md
	}

	if s := strings.TrimSpace(jsonutil.CoerceString(in["reasoning_effort"])); s != "" && s != "none" {
		out["reasoning"] = map[string]any{"effort": s}
	}
}

func normalizeChatImageURLToString(v any) any {
	switch vv := v.(type) {
	case string:
		return vv
	case map[string]any:
		if url := strings.TrimSpace(jsonutil.CoerceString(vv["url"])); url != "" {
			return url
		}
		return v
	default:
		return v
	}
}

func extractChatTextFromContent(content any) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	parts, ok := content.([]any)
	if !ok || len(parts) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		pm, _ := p.(map[string]any)
		if pm == nil {
			continue
		}
		if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != "text" {
			continue
		}
		t := jsonutil.CoerceString(pm["text"])
		if strings.TrimSpace(t) == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(t)
	}
	return sb.String()
}

func coerceChatContentToOutputValue(content any) any {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	// Keep structured content as JSON string when possible.
	if b, err := json.Marshal(content); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", content)
}

func appendAssistantToolCallsAsInputItems(dst *[]map[string]any, msg map[string]any) {
	if dst == nil || msg == nil {
		return
	}
	raw, ok := msg["tool_calls"].([]any)
	if !ok || len(raw) == 0 {
		return
	}
	for _, it := range raw {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		id := strings.TrimSpace(jsonutil.CoerceString(m["id"]))
		if id == "" {
			continue
		}
		typ := strings.TrimSpace(jsonutil.CoerceString(m["type"]))
		if typ != "" && typ != chatRoleFunction {
			continue
		}
		fn, _ := m["function"].(map[string]any)
		if fn == nil {
			continue
		}
		name := strings.TrimSpace(jsonutil.CoerceString(fn["name"]))
		if name == "" {
			continue
		}
		args := jsonutil.CoerceString(fn["arguments"])
		*dst = append(*dst, map[string]any{
			"type":      "function_call",
			"call_id":   id,
			"name":      name,
			"arguments": args,
		})
	}
}

func mapChatToolsToResponsesTools(tools []any) []any {
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		m, _ := t.(map[string]any)
		if m == nil {
			continue
		}
		typ := strings.TrimSpace(jsonutil.CoerceString(m["type"]))
		switch typ {
		case chatRoleFunction:
			fn, _ := m["function"].(map[string]any)
			if fn == nil {
				continue
			}
			out = append(out, map[string]any{
				"type":        "function",
				"name":        jsonutil.CoerceString(fn["name"]),
				"description": fn["description"],
				"parameters":  fn["parameters"],
			})
		default:
			// Best-effort: keep unknown tool shape.
			out = append(out, m)
		}
	}
	return out
}

func mapChatToolChoiceToResponses(v any) any {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		typ := strings.TrimSpace(jsonutil.CoerceString(t["type"]))
		if typ == chatRoleFunction {
			// Chat: {"type":"function","function":{"name":"..."}}
			// Responses: {"type":"function","name":"..."}
			if name := strings.TrimSpace(jsonutil.CoerceString(t["name"])); name != "" {
				return map[string]any{"type": chatRoleFunction, "name": name}
			}
			if fn, ok := t["function"].(map[string]any); ok && fn != nil {
				if name := strings.TrimSpace(jsonutil.CoerceString(fn["name"])); name != "" {
					return map[string]any{"type": chatRoleFunction, "name": name}
				}
			}
		}
		return t
	default:
		return v
	}
}
