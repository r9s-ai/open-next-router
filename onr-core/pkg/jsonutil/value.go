package jsonutil

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
)

type compiledPath struct {
	parts []compiledPathPart
}

type compiledPathPart struct {
	name   string
	idx    int
	hasIdx bool
	isStar bool
}

type compiledPathCacheEntry struct {
	path *compiledPath
	ok   bool
}

var compiledPathCache sync.Map

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

// GetFirstIntByPaths returns the first non-zero integer resolved from the given paths.
func GetFirstIntByPaths(root map[string]any, paths ...string) int {
	for _, path := range paths {
		if v := GetIntByPath(root, path); v != 0 {
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
	sum := 0
	if !VisitValuesByPath(root, path, func(v any) {
		sum += CoerceInt(v)
	}) {
		return 0
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
	sum := 0.0
	if !VisitValuesByPath(root, path, func(v any) {
		sum += CoerceFloat(v)
	}) {
		return 0, false
	}
	return sum, true
}

// GetStringByPath reads a string from a restricted JSONPath subset.
// When wildcard is used, returns the first non-empty string.
func GetStringByPath(root map[string]any, path string) string {
	compiled, ok := lookupCompiledPath(path)
	if !ok {
		return ""
	}
	return getStringByParts(root, compiled.parts)
}

// GetFirstStringByPaths returns the first non-empty string resolved from the given paths.
func GetFirstStringByPaths(root map[string]any, paths ...string) string {
	for _, path := range paths {
		if v := strings.TrimSpace(GetStringByPath(root, path)); v != "" {
			return v
		}
	}
	return ""
}

// GetValuesByPath reads all matched terminal values from a restricted JSONPath subset.
func GetValuesByPath(root any, path string) ([]any, bool) {
	vals := make([]any, 0)
	if !VisitValuesByPath(root, path, func(v any) {
		vals = append(vals, v)
	}) {
		return nil, false
	}
	return vals, true
}

// VisitValuesByPath walks all matched terminal values from a restricted JSONPath subset.
// It returns whether the path matched, even when the final wildcard expansion is empty.
func VisitValuesByPath(root any, path string, visit func(any)) bool {
	compiled, ok := lookupCompiledPath(path)
	if !ok {
		return false
	}
	return visitPathValues(root, compiled.parts, visit)
}

func getStringByParts(cur any, parts []compiledPathPart) string {
	for i, part := range parts {
		name, idx, hasIdx, isStar := part.name, part.idx, part.hasIdx, part.isStar
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

func visitPathValues(cur any, parts []compiledPathPart, visit func(any)) bool {
	if len(parts) == 0 {
		visit(cur)
		return true
	}
	part := parts[0]
	name, idx, hasIdx, isStar := part.name, part.idx, part.hasIdx, part.isStar
	if name != "" {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		next, ok := m[name]
		if !ok {
			return false
		}
		cur = next
	}
	rest := parts[1:]
	if !hasIdx {
		return visitPathValues(cur, rest, visit)
	}
	arr, ok := cur.([]any)
	if !ok {
		return false
	}
	if isStar {
		if len(rest) == 0 {
			for _, item := range arr {
				visit(item)
			}
			return true
		}
		for _, item := range arr {
			visitPathValues(item, rest, visit)
		}
		return true
	}
	if idx < 0 || idx >= len(arr) {
		return false
	}
	return visitPathValues(arr[idx], rest, visit)
}

func lookupCompiledPath(path string) (*compiledPath, bool) {
	if cached, ok := compiledPathCache.Load(path); ok {
		entry := cached.(compiledPathCacheEntry)
		return entry.path, entry.ok
	}
	compiled, ok := compilePath(path)
	entry := compiledPathCacheEntry{path: compiled, ok: ok}
	actual, _ := compiledPathCache.LoadOrStore(path, entry)
	resolved := actual.(compiledPathCacheEntry)
	return resolved.path, resolved.ok
}

func compilePath(path string) (*compiledPath, bool) {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
		return nil, false
	}
	rawParts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	parts := make([]compiledPathPart, 0, len(rawParts))
	for _, raw := range rawParts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return nil, false
		}
		name, idx, hasIdx, isStar := splitIndex(part)
		parts = append(parts, compiledPathPart{
			name:   name,
			idx:    idx,
			hasIdx: hasIdx,
			isStar: isStar,
		})
	}
	return &compiledPath{parts: parts}, true
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
