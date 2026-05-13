package usageestimate

import (
	"encoding/json"
	"strings"
)

func stringifyAnthropicRequest(reqBody map[string]any) *tokenEstimateContext {
	var b strings.Builder
	ctx := &tokenEstimateContext{}
	for k, v := range reqBody {
		switch k {
		case "system":
			b.WriteString("system\n")
			// data, _ := json.Marshal(v)
			// b.WriteString(string(data) + "\n")
			stringifyAnthropicMessages(v, &b, 0, 10)
			b.WriteString("\n")
		case "tools":
			b.WriteString("tools\n")
			toolList, ok := v.([]any)
			ctx.numTools = len(toolList)
			if !ok {
				data, _ := json.Marshal(v)
				b.WriteString(string(data) + "\n")
				break
			}
			for _, item := range toolList {
				tool, ok := item.(map[string]any)
				if ok {
					data, _ := json.Marshal(tool)
					b.WriteString(jsonNoiseReplacer.Replace(string(data)) + "\n")
				}
			}
		case "messages":
			b.WriteString("messages\n")
			stringifyAnthropicMessages(v, &b, 0, 10)
			b.WriteString("\n")
		}

	}
	ctx.text = b.String()
	return ctx
}
func stringifyAnthropicMessages(v any, b *strings.Builder, deep int, maxDeep int) {
	if deep > maxDeep {
		return
	}
	switch t := v.(type) {
	case []any: // Handle content block list.
		for _, it := range t {
			stringifyAnthropicMessages(it, b, deep+1, maxDeep)
		}
		return
	case string: // Handle content string.
		b.WriteString(t)
		return
	case map[string]any: // Handle content block.
		stringifyAnthropicMessageObject(t, b, deep, maxDeep)
		return

	default:
		return

	}

}
func stringifyAnthropicMessageObject(t map[string]any, b *strings.Builder, deep int, maxDeep int) {
	if role, ok := t["role"].(string); ok { // Role info.
		b.WriteString("role:" + role + "\n")
	}

	if typeInfo, ok := t["type"].(string); ok {
		stringifyAnthropicContentObject(typeInfo, t, b, deep, maxDeep)
		return
	}

	if content, ok := t["content"]; ok {
		stringifyAnthropicMessages(content, b, deep+1, maxDeep)
		b.WriteString("\n")
	}
}
func stringifyAnthropicContentObject(typeInfo string, t map[string]any, b *strings.Builder, deep int, maxDeep int) {
	switch typeInfo {
	case "text": // Text content.
		if text, ok := t["text"].(string); ok {
			b.WriteString(text)
		}
	case "image":
		// Image content is not extracted yet.
	case "document":
		// Document content is not extracted yet.

	case "tool_use", "server_tool_use": // Extract tool call details.
		b.WriteString(typeInfo + " ")
		if toolName, ok := t["name"].(string); ok {
			b.WriteString(toolName + " ")
		}
		if input, ok := t["input"].(map[string]any); ok {
			data, _ := json.Marshal(input)
			b.WriteString(jsonNoiseReplacer.Replace(string(data)))
		}
	case "tool_result": // Extract tool result details.
		b.WriteString(typeInfo + " ")
		if content, ok := t["content"]; ok {
			stringifyAnthropicToolResultContent(content, b, deep, maxDeep)
		}
	case "web_search_tool_result":
		// Web search result is not estimated yet.
	case "thinking": // Thinking content.
		if thinking, ok := t["thinking"].(string); ok {
			b.WriteString("thinking " + thinking)
		}
	default:
		// Unsupported Anthropic content block types are not extracted yet.
	}
}
func stringifyAnthropicToolResultContent(content any, b *strings.Builder, deep int, maxDeep int) {
	switch t := content.(type) {
	case string, []any:
		stringifyAnthropicMessages(t, b, deep+1, maxDeep)
	case map[string]any:
		if _, ok := t["type"].(string); ok {
			stringifyAnthropicMessages(t, b, deep+1, maxDeep)
			return
		}
		data, _ := json.Marshal(t)
		b.WriteString(jsonNoiseReplacer.Replace(string(data)))
	default:
		data, _ := json.Marshal(t)
		b.WriteString(jsonNoiseReplacer.Replace(string(data)))
	}
}
