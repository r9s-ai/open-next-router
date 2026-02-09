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

// GetIntByPath reads integer values from a restricted JSONPath subset and sums matched values.
// Supported syntax:
// - $.a.b.c
// - $.items[0].x
// - $.items[*].x (sum over matched items)
func GetIntByPath(root map[string]any, path string) int {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
		return 0
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	vals, ok := collectPathValues(root, parts)
	if !ok {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += CoerceInt(v)
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
