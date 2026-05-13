package streamtext

import (
	"bytes"
	"encoding/json"
	"strings"
)

type deltaTextEnvelope struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments any    `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			FunctionCall struct {
				Name      string `json:"name"`
				Arguments any    `json:"arguments"`
			} `json:"function_call"`
		} `json:"delta"`
		Text string `json:"text"`
	} `json:"choices"`
}

func NormalizeAPI(api string) string {
	return strings.ToLower(strings.TrimSpace(api))
}

func ExtractDeltaText(api string, payload []byte) string {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return ""
	}

	switch NormalizeAPI(api) {
	case "chat.completions":
		return extractOpenAIDeltaText(payload)
	case "responses":
		return extractOpenAIResponsesDeltaText(payload)
	case "claude.messages":
		return extractAnthropicDeltaText(payload)
	default:
		return ""
	}
}

func ExtractFromSSE(api string, sse []byte, limit int) string {
	sse = clampBytes(sse, limit)
	if len(bytes.TrimSpace(sse)) == 0 {
		return ""
	}
	events := bytes.Split(sse, []byte("\n\n"))
	var out strings.Builder
	for _, ev := range events {
		lines := bytes.Split(ev, []byte("\n"))
		var dataLines [][]byte
		for _, raw := range lines {
			line := bytes.TrimRight(raw, "\r")
			if bytes.HasPrefix(line, []byte("data:")) {
				dataLines = append(dataLines, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
			}
		}
		if len(dataLines) == 0 {
			continue
		}
		payload := bytes.TrimSpace(bytes.Join(dataLines, []byte("\n")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if delta := ExtractDeltaText(api, payload); delta != "" {
			out.WriteString(delta)
			continue
		}

		var obj any
		if err := json.Unmarshal(payload, &obj); err != nil {
			continue
		}
		collectTextFields(&out, obj, 0, 6)
	}
	return out.String()
}

func extractOpenAIDeltaText(payload []byte) string {
	var envelope deltaTextEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil || len(envelope.Choices) == 0 {
		return ""
	}

	var b strings.Builder
	for _, c := range envelope.Choices {
		if c.Delta.Content != "" {
			b.WriteString(c.Delta.Content)
		}
		if c.Delta.ReasoningContent != "" {
			b.WriteString(c.Delta.ReasoningContent)
		}
		for _, toolCall := range c.Delta.ToolCalls {
			if arguments := jsonValueString(toolCall.Function.Arguments); arguments != "" {
				b.WriteString(arguments)
				continue
			}
			if toolCall.Function.Name != "" {
				b.WriteString(toolCall.Function.Name)
				continue
			}
			if toolCall.ID != "" {
				b.WriteString(toolCall.ID)
			}
		}
		if arguments := jsonValueString(c.Delta.FunctionCall.Arguments); arguments != "" {
			b.WriteString(arguments)
		} else if c.Delta.FunctionCall.Name != "" {
			b.WriteString(c.Delta.FunctionCall.Name)
		}
		if c.Text != "" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

func jsonValueString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func extractOpenAIResponsesDeltaText(payload []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return ""
	}
	if s, ok := obj["delta"].(string); ok && s != "" {
		return s
	}
	// get tool name
	if typeInfo, ok := obj["type"].(string); ok && !strings.HasSuffix(typeInfo, "done") {
		if item, ok := obj["item"].(map[string]any); ok {
			if toolName, ok := item["name"].(string); ok {
				return toolName
			}
		}
	}

	return ""
}

func extractAnthropicDeltaText(payload []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return ""
	}
	if delta, ok := obj["delta"].(map[string]any); ok {
		for _, key := range []string{"text", "partial_json", "thinking"} {
			if s, ok := delta[key].(string); ok && s != "" {
				return s
			}
		}
	}
	if block, ok := obj["content_block"].(map[string]any); ok {
		switch block["type"] {
		case "tool_use", "server_tool_use":
			if name, ok := block["name"].(string); ok && name != "" {
				return name
			}
			if id, ok := block["id"].(string); ok && id != "" {
				return id
			}
		}
	}
	return ""
}

func collectTextFields(out *strings.Builder, v any, depth, maxDepth int) {
	if depth > maxDepth || v == nil {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			if strings.EqualFold(k, "text") {
				if s, ok := vv.(string); ok && strings.TrimSpace(s) != "" {
					out.WriteString(s)
					out.WriteByte('\n')
					continue
				}
			}
			collectTextFields(out, vv, depth+1, maxDepth)
		}
	case []any:
		for _, it := range t {
			collectTextFields(out, it, depth+1, maxDepth)
		}
	}
}

func clampBytes(b []byte, limit int) []byte {
	if limit <= 0 || len(b) <= limit {
		return b
	}
	return b[len(b)-limit:]
}
