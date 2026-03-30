package jsonutil

import (
	"strings"
	"testing"
)

func BenchmarkGetFloatByPathWithMatch_New(b *testing.B) {
	root := benchmarkUsageRoot()
	path := "$.usage.items[*].tokens"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sum, matched := GetFloatByPathWithMatch(root, path)
		if !matched || sum == 0 {
			b.Fatalf("unexpected result matched=%v sum=%v", matched, sum)
		}
	}
}

func BenchmarkGetFloatByPathWithMatch_OldTwoPass(b *testing.B) {
	root := benchmarkUsageRoot()
	path := "$.usage.items[*].tokens"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sum, matched := benchmarkLegacyGetFloatByPathWithMatchTwoPass(root, path)
		if !matched || sum == 0 {
			b.Fatalf("unexpected result matched=%v sum=%v", matched, sum)
		}
	}
}

func BenchmarkGetFirstIntByPaths_New(b *testing.B) {
	root := benchmarkUsageRoot()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := GetFirstIntByPaths(root, "$.usage.prompt_tokens", "$.usage.input_tokens"); got != 123 {
			b.Fatalf("unexpected result %d", got)
		}
	}
}

func BenchmarkGetFirstIntByPaths_OldEager(b *testing.B) {
	root := benchmarkUsageRoot()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := FirstInt(GetIntByPath(root, "$.usage.prompt_tokens"), GetIntByPath(root, "$.usage.input_tokens")); got != 123 {
			b.Fatalf("unexpected result %d", got)
		}
	}
}

func benchmarkUsageRoot() map[string]any {
	items := make([]any, 0, 64)
	for i := 0; i < 64; i++ {
		items = append(items, map[string]any{
			"tokens": float64(i%5 + 1),
			"type":   "token",
		})
	}
	return map[string]any{
		"usage": map[string]any{
			"prompt_tokens": float64(123),
			"input_tokens":  float64(99),
			"items":         items,
		},
	}
}

func benchmarkLegacyGetFloatByPathWithMatchTwoPass(root map[string]any, path string) (float64, bool) {
	vals, matched := benchmarkLegacyGetValuesByPath(root, path)
	if !matched {
		return 0, false
	}
	sum := 0.0
	for _, v := range benchmarkLegacyGetFloatValuesByPath(root, path) {
		sum += CoerceFloat(v)
	}
	_ = vals
	return sum, true
}

func benchmarkLegacyGetFloatValuesByPath(root map[string]any, path string) []any {
	vals, _ := benchmarkLegacyGetValuesByPath(root, path)
	return vals
}

func benchmarkLegacyGetValuesByPath(root any, path string) ([]any, bool) {
	parts, ok := benchmarkLegacyParsePath(path)
	if !ok {
		return nil, false
	}
	out := make([]any, 0)
	if !benchmarkLegacyVisitPathValues(root, parts, func(v any) {
		out = append(out, v)
	}) {
		return nil, false
	}
	return out, true
}

func benchmarkLegacyParsePath(path string) ([]compiledPathPart, bool) {
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
	return parts, true
}

func benchmarkLegacyVisitPathValues(cur any, parts []compiledPathPart, visit func(any)) bool {
	if len(parts) == 0 {
		visit(cur)
		return true
	}
	part := parts[0]
	if part.name != "" {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		next, ok := m[part.name]
		if !ok {
			return false
		}
		cur = next
	}
	rest := parts[1:]
	if !part.hasIdx {
		return benchmarkLegacyVisitPathValues(cur, rest, visit)
	}
	arr, ok := cur.([]any)
	if !ok {
		return false
	}
	if part.isStar {
		if len(rest) == 0 {
			for _, item := range arr {
				visit(item)
			}
			return true
		}
		matched := true
		for _, item := range arr {
			if !benchmarkLegacyVisitPathValues(item, rest, visit) {
				matched = false
			}
		}
		return matched || len(arr) == 0
	}
	if part.idx < 0 || part.idx >= len(arr) {
		return false
	}
	return benchmarkLegacyVisitPathValues(arr[part.idx], rest, visit)
}
