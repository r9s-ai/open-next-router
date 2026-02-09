package dslconfig

import (
	"strconv"
	"strings"
)

// collectPathValues walks a restricted JSONPath parts list and returns terminal values.
// It supports object navigation, index access and star expansion.
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
