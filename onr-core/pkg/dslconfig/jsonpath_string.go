package dslconfig

import (
	"strings"
)

// getStringByPath implements a restricted JSONPath subset:
// - $.a.b.c
// - $.items[0].x
// - $.items[*].x (returns the first non-empty string)
func getStringByPath(root map[string]any, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "$.") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	return getStringByParts(root, parts)
}

func getStringByParts(cur any, parts []string) string {
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return ""
		}
		name, idx, hasIdx, isStar := splitIndex(part)
		if name != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return ""
			}
			next, ok := m[name]
			if !ok {
				return ""
			}
			cur = next
		}
		if hasIdx {
			arr, ok := cur.([]any)
			if !ok {
				return ""
			}
			if isStar {
				rest := parts[i+1:]
				if len(rest) == 0 {
					for _, item := range arr {
						if v, ok := item.(string); ok && strings.TrimSpace(v) != "" {
							return v
						}
					}
					return ""
				}
				for _, item := range arr {
					if v := getStringByParts(item, rest); strings.TrimSpace(v) != "" {
						return v
					}
				}
				return ""
			}
			if idx < 0 || idx >= len(arr) {
				return ""
			}
			cur = arr[idx]
		}
	}
	v, ok := cur.(string)
	if !ok {
		return ""
	}
	return v
}
