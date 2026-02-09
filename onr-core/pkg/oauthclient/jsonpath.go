package oauthclient

import (
	"encoding/json"
	"strconv"
	"strings"
)

func getStringByPath(root map[string]any, path string) string {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
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
	v, _ := cur.(string)
	return strings.TrimSpace(v)
}

func getFloatByPath(root map[string]any, path string) float64 {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
		return 0
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	vals, ok := collectPathValues(root, parts)
	if !ok {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += coerceFloat(v)
	}
	return sum
}

func collectPathValues(cur any, parts []string) ([]any, bool) {
	if len(parts) == 0 {
		return []any{cur}, true
	}
	part := strings.TrimSpace(parts[0])
	if part == "" {
		return nil, false
	}
	name, idx, hasIdx, isStar := splitIndex(part)
	if name != "" {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[name]
		if !ok {
			return nil, false
		}
		cur = next
	}
	rest := parts[1:]
	if !hasIdx {
		return collectPathValues(cur, rest)
	}
	arr, ok := cur.([]any)
	if !ok {
		return nil, false
	}
	if isStar {
		out := make([]any, 0, len(arr))
		if len(rest) == 0 {
			out = append(out, arr...)
			return out, true
		}
		for _, item := range arr {
			vals, ok := collectPathValues(item, rest)
			if !ok {
				continue
			}
			out = append(out, vals...)
		}
		return out, true
	}
	if idx < 0 || idx >= len(arr) {
		return nil, false
	}
	return collectPathValues(arr[idx], rest)
}

func splitIndex(s string) (name string, idx int, hasIdx bool, isStar bool) {
	open := strings.IndexByte(s, '[')
	if open < 0 {
		return s, 0, false, false
	}
	close := strings.IndexByte(s, ']')
	if close < 0 || close < open {
		return s, 0, false, false
	}
	name = s[:open]
	inner := strings.TrimSpace(s[open+1 : close])
	if inner == "*" {
		return name, 0, true, true
	}
	n, err := strconv.Atoi(inner)
	if err != nil {
		return name, 0, false, false
	}
	return name, n, true, false
}

func coerceFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return f
		}
		i, err := t.Int64()
		if err == nil {
			return float64(i)
		}
		return 0
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
