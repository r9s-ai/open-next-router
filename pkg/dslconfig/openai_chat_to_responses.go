package dslconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// MapOpenAIChatCompletionsToResponsesRequest converts an OpenAI-style chat.completions request JSON
// into an OpenAI Responses request JSON (best-effort), following new-api's compatibility semantics.
//
// Notes:
// - This function only transforms the JSON payload; it does not alter headers or URL.
// - It is intentionally permissive and keeps unknown fields out (to avoid surprising upstream behavior).
func MapOpenAIChatCompletionsToResponsesRequest(reqBody []byte) ([]byte, error) {
	var obj any
	if err := json.Unmarshal(reqBody, &obj); err != nil {
		return nil, fmt.Errorf("parse chat request json: %w", err)
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return nil, errors.New("chat request json is not an object")
	}

	// If it already looks like a Responses request, do not re-map.
	if _, ok := root["input"]; ok && root["messages"] == nil {
		return reqBody, nil
	}

	model := strings.TrimSpace(coerceString(root["model"]))
	if model == "" {
		return nil, errors.New("model is required")
	}

	if n := coerceInt(root["n"]); n > 1 {
		return nil, fmt.Errorf("n>1 is not supported in responses compatibility mode")
	}

	messages, _ := root["messages"].([]any)
	instructionsParts := make([]string, 0, 2)
	inputItems := make([]map[string]any, 0, len(messages))

	for _, raw := range messages {
		msg, _ := raw.(map[string]any)
		if msg == nil {
			continue
		}
		role := strings.TrimSpace(coerceString(msg["role"]))
		if role == "" {
			continue
		}

		// Tool/function message => function_call_output item (needs call_id).
		if role == "tool" || role == "function" {
			callID := strings.TrimSpace(coerceString(msg["tool_call_id"]))
			if callID == "" {
				callID = strings.TrimSpace(coerceString(msg["tool_callid"]))
			}
			output := coerceChatContentToOutputValue(msg["content"])
			if callID == "" {
				inputItems = append(inputItems, map[string]any{
					"role":    "user",
					"content": fmt.Sprintf("[tool_output_missing_call_id] %v", output),
				})
				continue
			}
			inputItems = append(inputItems, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  output,
			})
			continue
		}

		// Prefer mapping system/developer messages into instructions.
		if role == "system" || role == "developer" {
			if s := extractChatTextFromContent(msg["content"]); strings.TrimSpace(s) != "" {
				instructionsParts = append(instructionsParts, strings.TrimSpace(s))
			}
			continue
		}

		item := map[string]any{
			"role": role,
		}

		if msg["content"] == nil {
			item["content"] = ""
			inputItems = append(inputItems, item)
			if role == "assistant" {
				appendAssistantToolCallsAsInputItems(&inputItems, msg)
			}
			continue
		}

		if s, ok := msg["content"].(string); ok {
			item["content"] = s
			inputItems = append(inputItems, item)
			if role == "assistant" {
				appendAssistantToolCallsAsInputItems(&inputItems, msg)
			}
			continue
		}

		// Content parts (multi-modal).
		if parts, ok := msg["content"].([]any); ok {
			contentParts := make([]map[string]any, 0, len(parts))
			for _, p := range parts {
				pm, _ := p.(map[string]any)
				if pm == nil {
					continue
				}
				pt := strings.TrimSpace(coerceString(pm["type"]))
				switch pt {
				case "text":
					contentParts = append(contentParts, map[string]any{
						"type": "input_text",
						"text": coerceString(pm["text"]),
					})
				case "image_url":
					contentParts = append(contentParts, map[string]any{
						"type":      "input_image",
						"image_url": normalizeChatImageURLToString(pm["image_url"]),
					})
				case "input_audio":
					contentParts = append(contentParts, map[string]any{
						"type":        "input_audio",
						"input_audio": pm["input_audio"],
					})
				case "file":
					contentParts = append(contentParts, map[string]any{
						"type": "input_file",
						"file": pm["file"],
					})
				case "video_url":
					contentParts = append(contentParts, map[string]any{
						"type":      "input_video",
						"video_url": pm["video_url"],
					})
				default:
					contentParts = append(contentParts, map[string]any{
						"type": pt,
					})
				}
			}
			item["content"] = contentParts
			inputItems = append(inputItems, item)
			if role == "assistant" {
				appendAssistantToolCallsAsInputItems(&inputItems, msg)
			}
			continue
		}

		// Fallback: stringify unknown content shapes.
		item["content"] = fmt.Sprintf("%v", msg["content"])
		inputItems = append(inputItems, item)
		if role == "assistant" {
			appendAssistantToolCallsAsInputItems(&inputItems, msg)
		}
	}

	out := map[string]any{
		"model": model,
		"input": inputItems,
	}

	// instructions
	if len(instructionsParts) > 0 {
		out["instructions"] = strings.Join(instructionsParts, "\n\n")
	}

	// stream passthrough
	if v, ok := root["stream"].(bool); ok {
		out["stream"] = v
	}

	// temperature/top_p/user
	if v, ok := root["temperature"].(float64); ok {
		out["temperature"] = v
	}
	if v, ok := root["top_p"].(float64); ok && v != 0 {
		out["top_p"] = v
	}
	if s := strings.TrimSpace(coerceString(root["user"])); s != "" {
		out["user"] = s
	}

	// max_output_tokens: max(max_tokens, max_completion_tokens)
	maxOutput := coerceInt(root["max_tokens"])
	if v := coerceInt(root["max_completion_tokens"]); v > maxOutput {
		maxOutput = v
	}
	if maxOutput > 0 {
		out["max_output_tokens"] = maxOutput
	}

	// tools/tool_choice/parallel_tool_calls
	if tools, ok := root["tools"].([]any); ok && len(tools) > 0 {
		out["tools"] = mapChatToolsToResponsesTools(tools)
	}
	if tc := root["tool_choice"]; tc != nil {
		if mapped := mapChatToolChoiceToResponses(tc); mapped != nil {
			out["tool_choice"] = mapped
		}
	}
	if v := root["parallel_tool_calls"]; v != nil {
		out["parallel_tool_calls"] = v
	}

	// response_format -> text.format
	if rf := root["response_format"]; rf != nil {
		out["text"] = map[string]any{
			"format": rf,
		}
	}

	// store/metadata
	if v, ok := root["store"].(bool); ok {
		out["store"] = v
	}
	if md, ok := root["metadata"].(map[string]any); ok && md != nil {
		out["metadata"] = md
	}

	// reasoning.effort (best-effort; keep only when explicitly set and not "none")
	if s := strings.TrimSpace(coerceString(root["reasoning_effort"])); s != "" && s != "none" {
		out["reasoning"] = map[string]any{"effort": s}
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func normalizeChatImageURLToString(v any) any {
	switch vv := v.(type) {
	case string:
		return vv
	case map[string]any:
		if url := strings.TrimSpace(coerceString(vv["url"])); url != "" {
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
		if strings.TrimSpace(coerceString(pm["type"])) != "text" {
			continue
		}
		t := coerceString(pm["text"])
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
		id := strings.TrimSpace(coerceString(m["id"]))
		if id == "" {
			continue
		}
		typ := strings.TrimSpace(coerceString(m["type"]))
		if typ != "" && typ != "function" {
			continue
		}
		fn, _ := m["function"].(map[string]any)
		if fn == nil {
			continue
		}
		name := strings.TrimSpace(coerceString(fn["name"]))
		if name == "" {
			continue
		}
		args := coerceString(fn["arguments"])
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
		typ := strings.TrimSpace(coerceString(m["type"]))
		switch typ {
		case "function":
			fn, _ := m["function"].(map[string]any)
			if fn == nil {
				continue
			}
			out = append(out, map[string]any{
				"type":        "function",
				"name":        coerceString(fn["name"]),
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
		typ := strings.TrimSpace(coerceString(t["type"]))
		if typ == "function" {
			// Chat: {"type":"function","function":{"name":"..."}}
			// Responses: {"type":"function","name":"..."}
			if name := strings.TrimSpace(coerceString(t["name"])); name != "" {
				return map[string]any{"type": "function", "name": name}
			}
			if fn, ok := t["function"].(map[string]any); ok && fn != nil {
				if name := strings.TrimSpace(coerceString(fn["name"])); name != "" {
					return map[string]any{"type": "function", "name": name}
				}
			}
		}
		return t
	default:
		return v
	}
}
