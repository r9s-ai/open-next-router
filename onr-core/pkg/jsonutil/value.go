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
	name        string
	idx         int
	hasIdx      bool
	isStar      bool
	hasFilter   bool
	filterField string
	filterValue string
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
// - $.items[?(@.field=="VALUE")].x
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
			if part.hasFilter {
				rest := parts[i+1:]
				for _, item := range arr {
					if !matchesFilter(item, part.filterField, part.filterValue) {
						continue
					}
					if len(rest) == 0 {
						if v, ok := item.(string); ok && strings.TrimSpace(v) != "" {
							return v
						}
						continue
					}
					if v := getStringByParts(item, rest); strings.TrimSpace(v) != "" {
						return v
					}
				}
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
	if part.hasFilter {
		matched := true
		for _, item := range arr {
			if !matchesFilter(item, part.filterField, part.filterValue) {
				continue
			}
			visitPathValues(item, rest, visit)
		}
		return matched
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
	rawParts, ok := splitPathParts(strings.TrimPrefix(p, "$."))
	if !ok {
		return nil, false
	}
	parts := make([]compiledPathPart, 0, len(rawParts))
	for _, raw := range rawParts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return nil, false
		}
		name, idx, hasIdx, isStar, hasFilter, filterField, filterValue, ok := splitIndex(part)
		if !ok {
			return nil, false
		}
		parts = append(parts, compiledPathPart{
			name:        name,
			idx:         idx,
			hasIdx:      hasIdx,
			isStar:      isStar,
			hasFilter:   hasFilter,
			filterField: filterField,
			filterValue: filterValue,
		})
	}
	return &compiledPath{parts: parts}, true
}

func splitPathParts(path string) ([]string, bool) {
	parts := make([]string, 0, 4)
	start := 0
	bracketDepth := 0
	var quote byte
	for i := 0; i < len(path); i++ {
		ch := path[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return nil, false
			}
		case '.':
			if bracketDepth == 0 {
				parts = append(parts, path[start:i])
				start = i + 1
			}
		}
	}
	if quote != 0 || bracketDepth != 0 {
		return nil, false
	}
	parts = append(parts, path[start:])
	return parts, true
}

func splitIndex(s string) (name string, idx int, hasIdx bool, isStar bool, hasFilter bool, filterField string, filterValue string, ok bool) {
	open := strings.IndexByte(s, '[')
	if open < 0 {
		return s, 0, false, false, false, "", "", true
	}
	close := strings.LastIndexByte(s, ']')
	if close < 0 || close < open || close != len(s)-1 {
		return s, 0, false, false, false, "", "", false
	}
	name = s[:open]
	inner := strings.TrimSpace(s[open+1 : close])
	if inner == "*" {
		return name, 0, true, true, false, "", "", true
	}
	if field, value, ok := parseFilter(inner); ok {
		return name, 0, true, false, true, field, value, true
	}
	n, err := strconv.Atoi(inner)
	if err != nil {
		return name, 0, false, false, false, "", "", false
	}
	return name, n, true, false, false, "", "", true
}

func parseFilter(inner string) (field string, value string, ok bool) {
	trimmed := strings.TrimSpace(inner)
	if !strings.HasPrefix(trimmed, "?(") || !strings.HasSuffix(trimmed, ")") {
		return "", "", false
	}
	body := strings.TrimSpace(trimmed[2 : len(trimmed)-1])
	if !strings.HasPrefix(body, "@.") {
		return "", "", false
	}
	body = strings.TrimPrefix(body, "@.")
	parts := strings.SplitN(body, "==", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	field = strings.TrimSpace(parts[0])
	rawValue := strings.TrimSpace(parts[1])
	if field == "" || rawValue == "" {
		return "", "", false
	}
	if len(rawValue) < 2 {
		return "", "", false
	}
	quote := rawValue[0]
	if (quote != '"' && quote != '\'') || rawValue[len(rawValue)-1] != quote {
		return "", "", false
	}
	value = rawValue[1 : len(rawValue)-1]
	return field, value, true
}

func matchesFilter(v any, field, want string) bool {
	got, ok := getNestedStringField(v, field)
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want))
}

func getNestedStringField(v any, field string) (string, bool) {
	current := v
	for _, part := range strings.Split(strings.TrimSpace(field), ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", false
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		next, ok := obj[part]
		if !ok {
			return "", false
		}
		current = next
	}
	s, ok := current.(string)
	return s, ok
}
