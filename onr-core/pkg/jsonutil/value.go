package jsonutil

import (
	"encoding/json"
	"strconv"
	"strings"
)

// CoerceString converts a value to string when it is already a string.
func CoerceString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

// CoerceInt converts common numeric-like values to int.
func CoerceInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i
		}
	case []any:
		sum := 0
		for _, item := range t {
			sum += CoerceInt(item)
		}
		return sum
	}
	return 0
}

// FirstInt returns the first non-zero integer.
func FirstInt(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

// CoerceFloat converts common numeric-like values to float64.
func CoerceFloat(v any) float64 {
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
		if f, err := t.Float64(); err == nil {
			return f
		}
		if i, err := t.Int64(); err == nil {
			return float64(i)
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f
		}
	}
	return 0
}

// GetIntByPath reads integer values from a restricted JSONPath subset and sums matched values.
// Supported syntax:
// - $.a.b.c
// - $.items[0].x
// - $.items[*].x (sum over matched items)
func GetIntByPath(root map[string]any, path string) int {
	vals, ok := GetValuesByPath(root, path)
	if !ok {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += CoerceInt(v)
	}
	return sum
}

// GetFloatByPath reads float values from a restricted JSONPath subset and sums matched values.
func GetFloatByPath(root map[string]any, path string) float64 {
	sum, _ := GetFloatByPathWithMatch(root, path)
	return sum
}

// GetFloatByPathWithMatch reads float values from a restricted JSONPath subset,
// sums matched values, and reports whether the path matched.
func GetFloatByPathWithMatch(root map[string]any, path string) (float64, bool) {
	vals, ok := GetValuesByPath(root, path)
	if !ok {
		return 0, false
	}
	sum := 0.0
	for _, v := range vals {
		sum += CoerceFloat(v)
	}
	return sum, true
}

// GetStringByPath reads a string from a restricted JSONPath subset.
// When wildcard is used, returns the first non-empty string.
func GetStringByPath(root map[string]any, path string) string {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	return getStringByParts(root, parts)
}

// GetValuesByPath reads all matched terminal values from a restricted JSONPath subset.
func GetValuesByPath(root any, path string) ([]any, bool) {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
		return nil, false
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	return collectPathValues(root, parts)
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
