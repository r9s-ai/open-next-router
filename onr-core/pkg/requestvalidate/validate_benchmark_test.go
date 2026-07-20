package requestvalidate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

// benchmarkRules builds n body rules that all pass against benchmarkRoot.
func benchmarkRules(n int) []dslconfig.RequestValidationRule {
	min := float64(0)
	max := float64(10)
	rules := make([]dslconfig.RequestValidationRule, 0, n)
	for i := 0; len(rules) < n; i++ {
		field := fmt.Sprintf("f%d", i%8)
		switch i % 4 {
		case 0:
			rules = append(rules, dslconfig.RequestValidationRule{
				Op: "required", Source: "body", Path: "$." + field, PathParts: []string{field},
			})
		case 1:
			rules = append(rules, dslconfig.RequestValidationRule{
				Op: "type", Source: "body", Path: "$." + field, PathParts: []string{field}, Type: "number",
			})
		case 2:
			rules = append(rules, dslconfig.RequestValidationRule{
				Op: "range", Source: "body", Path: "$." + field, PathParts: []string{field}, Min: &min, Max: &max,
			})
		case 3:
			rules = append(rules, dslconfig.RequestValidationRule{
				Op: "enum", Source: "body", Path: "$." + field, PathParts: []string{field},
				LiteralValues: []any{float64(1), float64(2), float64(3)},
			})
		}
	}
	return rules
}

func benchmarkRoot(b *testing.B) map[string]any {
	b.Helper()
	body := `{"f0":1,"f1":2,"f2":3,"f3":1,"f4":2,"f5":3,"f6":1,"f7":2,"deep":{"l2":{"l3":{"l4":{"l5":"v"}}}}}`
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		b.Fatalf("unmarshal: %v", err)
	}
	return root
}

func BenchmarkValidate_BodyRules(b *testing.B) {
	for _, n := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("rules_%d", n), func(b *testing.B) {
			rules := benchmarkRules(n)
			root := benchmarkRoot(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := Validate(rules, root, nil, ""); err != nil {
					b.Fatalf("unexpected failure: %v", err)
				}
			}
		})
	}
}

func BenchmarkValidate_PathDepth(b *testing.B) {
	paths := map[int]string{1: "$.f0", 3: "$.deep.l2.l3", 5: "$.deep.l2.l3.l4.l5"}
	for depth, path := range paths {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			rule := dslconfig.RequestValidationRule{
				Op: "required", Source: "body", Path: path,
				PathParts: strings.Split(strings.TrimPrefix(path, "$."), "."),
			}
			rules := []dslconfig.RequestValidationRule{rule}
			root := benchmarkRoot(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := Validate(rules, root, nil, ""); err != nil {
					b.Fatalf("unexpected failure: %v", err)
				}
			}
		})
	}
}

func BenchmarkValidate_HeaderQuery(b *testing.B) {
	rules := []dslconfig.RequestValidationRule{
		{Op: "required", Source: "header", Name: "anthropic-beta", CanonicalName: "Anthropic-Beta"},
		{Op: "enum", Source: "header", Name: "anthropic-beta", CanonicalName: "Anthropic-Beta",
			StringValueSet: map[string]struct{}{"tools-2024": {}}},
		{Op: "required", Source: "query", Name: "api-version"},
		{Op: "enum", Source: "query", Name: "api-version",
			StringValueSet: map[string]struct{}{"2024-06-01": {}}},
	}
	headers := http.Header{"Anthropic-Beta": []string{"tools-2024"}}
	rawQuery := "api-version=2024-06-01"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Validate(rules, nil, headers, rawQuery); err != nil {
			b.Fatalf("unexpected failure: %v", err)
		}
	}
}

func BenchmarkValidate_FirstRuleFails(b *testing.B) {
	rules := benchmarkRules(100)
	// Remove the first rule's target so it fails immediately.
	root := benchmarkRoot(b)
	delete(root, "f0")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Validate(rules, root, nil, ""); err == nil {
			b.Fatalf("expected failure")
		}
	}
}
