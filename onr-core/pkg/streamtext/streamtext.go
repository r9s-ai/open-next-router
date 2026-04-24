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
		if c.Text != "" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

func extractOpenAIResponsesDeltaText(payload []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return ""
	}
	if s, ok := obj["delta"].(string); ok && s != "" {
		return s
	}
	return ""
}

func extractAnthropicDeltaText(payload []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return ""
	}
	delta, _ := obj["delta"].(map[string]any)
	if delta == nil {
		return ""
	}
	if s, ok := delta["text"].(string); ok && s != "" {
		return s
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
