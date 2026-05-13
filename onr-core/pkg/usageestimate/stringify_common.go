package usageestimate

import (
	"strings"
)

var jsonNoiseReplacer = strings.NewReplacer(
	`"`, "",
	"{", "",
	"}", "",
	"[", "",
	"]", "",
	":", " ",
	",", " ",
)

func stringifyAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, it := range t {
			s := stringifyAny(it)
			if strings.TrimSpace(s) == "" {
				continue
			}
			b.WriteString(s)
			b.WriteByte('\n')
		}
		return b.String()
	case map[string]any:
		var b strings.Builder
		if role, ok := t["role"].(string); ok {
			b.WriteString(role + " ")
		}

		if text, ok := t["text"].(string); ok {
			b.WriteString(text + " ")
		}
		if content, ok := t["content"].(string); ok {
			// Anthropic example: {"role": "user", "content": "Hello"}
			b.WriteString(content + " ")
		}

		collectTextFields(&b, t, 0, 4)
		return b.String()
	default:
		return ""
	}
}

// extract text content and tool count from Anthropic request body for token estimation
