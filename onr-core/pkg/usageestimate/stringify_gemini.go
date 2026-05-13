package usageestimate

import "strings"

func stringifyGeminiContents(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return stringifyAny(v)
	}
	var b strings.Builder
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		if parts, ok := m["parts"]; ok {
			s := stringifyAny(parts)
			if strings.TrimSpace(s) != "" {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}
func stringifyGeminiCandidates(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return stringifyAny(v)
	}
	var b strings.Builder
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		if content, ok := m["content"].(map[string]any); ok {
			if parts, ok := content["parts"]; ok {
				s := stringifyAny(parts)
				if strings.TrimSpace(s) != "" {
					b.WriteString(s)
					b.WriteByte('\n')
				}
			}
		}
	}
	return b.String()
}
