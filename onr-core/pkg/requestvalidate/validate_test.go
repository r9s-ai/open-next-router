package requestvalidate

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

func bodyRule(op, path string) dslconfig.RequestValidationRule {
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	return dslconfig.RequestValidationRule{
		Op:        op,
		Source:    dslconfig.ReqValidationSourceBody,
		Path:      path,
		PathParts: parts,
	}
}

func mustRoot(t *testing.T, body string) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		t.Fatalf("unmarshal test body: %v", err)
	}
	return root
}

func TestRequired(t *testing.T) {
	root := mustRoot(t, `{"model":"gpt-4o","meta":null,"nested":{"a":1}}`)

	if err := Validate([]dslconfig.RequestValidationRule{bodyRule("required", "$.model")}, root, nil, ""); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := Validate([]dslconfig.RequestValidationRule{bodyRule("required", "$.nested.a")}, root, nil, ""); err != nil {
		t.Fatalf("expected nested pass, got %v", err)
	}

	err := Validate([]dslconfig.RequestValidationRule{bodyRule("required", "$.missing")}, root, nil, "")
	if err == nil || err.Rule != dslconfig.ReqRuleRequired || err.PathOrName != "$.missing" {
		t.Fatalf("expected required failure, got %#v", err)
	}

	err = Validate([]dslconfig.RequestValidationRule{bodyRule("required", "$.meta")}, root, nil, "")
	if err == nil || !strings.Contains(err.Message, "null") {
		t.Fatalf("expected null rejection, got %#v", err)
	}

	allowNull := bodyRule("required", "$.meta")
	allowNull.AllowNull = true
	if err := Validate([]dslconfig.RequestValidationRule{allowNull}, root, nil, ""); err != nil {
		t.Fatalf("expected allow_null pass, got %v", err)
	}
}

func TestForbid(t *testing.T) {
	root := mustRoot(t, `{"legacy":1}`)
	if err := Validate([]dslconfig.RequestValidationRule{bodyRule("forbid", "$.other")}, root, nil, ""); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	err := Validate([]dslconfig.RequestValidationRule{bodyRule("forbid", "$.legacy")}, root, nil, "")
	if err == nil || err.Rule != dslconfig.ReqRuleForbid {
		t.Fatalf("expected forbid failure, got %#v", err)
	}
}

func TestType(t *testing.T) {
	root := mustRoot(t, `{"s":"x","n":1.5,"i":3,"b":true,"nul":null,"arr":[1],"obj":{}}`)
	cases := []struct {
		path string
		typ  string
		pass bool
	}{
		{"$.s", "string", true},
		{"$.s", "number", false},
		{"$.n", "number", true},
		{"$.n", "integer", false},
		{"$.i", "integer", true},
		{"$.i", "number", true},
		{"$.b", "bool", true},
		{"$.nul", "null", true},
		{"$.arr", "array", true},
		{"$.obj", "object", true},
		{"$.obj", "array", false},
		{"$.missing", "string", true}, // missing target is a no-op
	}
	for _, tc := range cases {
		rule := bodyRule("type", tc.path)
		rule.Type = tc.typ
		err := Validate([]dslconfig.RequestValidationRule{rule}, root, nil, "")
		if tc.pass && err != nil {
			t.Fatalf("%s as %s: expected pass, got %v", tc.path, tc.typ, err)
		}
		if !tc.pass && err == nil {
			t.Fatalf("%s as %s: expected failure", tc.path, tc.typ)
		}
	}
}

func TestRange(t *testing.T) {
	root := mustRoot(t, `{"t":1.5,"neg":-3,"i":7,"s":"x"}`)
	cases := []struct {
		path     string
		min, max *float64
		pass     bool
	}{
		{"$.t", f64(0), f64(2), true},
		{"$.t", f64(1.6), nil, false},
		{"$.t", nil, f64(1.4), false},
		{"$.neg", f64(-5), f64(0), true},
		{"$.i", f64(0), f64(7), true},
		{"$.s", f64(0), nil, false}, // non-number fails
		{"$.missing", f64(0), nil, true},
	}
	for i, tc := range cases {
		rule := bodyRule("range", tc.path)
		rule.Min, rule.Max = tc.min, tc.max
		err := Validate([]dslconfig.RequestValidationRule{rule}, root, nil, "")
		if tc.pass != (err == nil) {
			t.Fatalf("case %d (%s): pass=%v, got %v", i, tc.path, tc.pass, err)
		}
	}
}

func TestLen(t *testing.T) {
	root := mustRoot(t, `{"s":"héllo","arr":[1,2,3],"n":5}`)
	cases := []struct {
		path     string
		min, max *int
		pass     bool
	}{
		{"$.s", iptr(5), iptr(5), true}, // rune count, not byte count
		{"$.s", iptr(6), nil, false},
		{"$.arr", iptr(1), iptr(3), true},
		{"$.arr", nil, iptr(2), false},
		{"$.n", iptr(0), nil, false}, // non-string/array fails
		{"$.missing", iptr(1), nil, true},
	}
	for i, tc := range cases {
		rule := bodyRule("len", tc.path)
		rule.MinLen, rule.MaxLen = tc.min, tc.max
		err := Validate([]dslconfig.RequestValidationRule{rule}, root, nil, "")
		if tc.pass != (err == nil) {
			t.Fatalf("case %d (%s): pass=%v, got %v", i, tc.path, tc.pass, err)
		}
	}
}

func TestEnumBody(t *testing.T) {
	root := mustRoot(t, `{"effort":"low","stream":true,"n":2,"nul":null}`)
	cases := []struct {
		path     string
		literals []any
		pass     bool
	}{
		{"$.effort", []any{"low", "high"}, true},
		{"$.effort", []any{"medium", "high"}, false},
		{"$.stream", []any{true, false}, true},
		{"$.stream", []any{false}, false},
		{"$.n", []any{float64(1), float64(2)}, true},
		{"$.n", []any{float64(3)}, false},
		{"$.nul", []any{nil}, true},
		{"$.effort", []any{float64(1)}, false}, // type mismatch never matches
		{"$.missing", []any{"x"}, true},
	}
	for i, tc := range cases {
		rule := bodyRule("enum", tc.path)
		rule.LiteralValues = tc.literals
		err := Validate([]dslconfig.RequestValidationRule{rule}, root, nil, "")
		if tc.pass != (err == nil) {
			t.Fatalf("case %d (%s): pass=%v, got %v", i, tc.path, tc.pass, err)
		}
	}
}

func TestHeaderRules(t *testing.T) {
	headers := http.Header{
		"Anthropic-Beta": []string{"tools-2024"},
		"X-Budget":       []string{"42"},
	}

	required := dslconfig.RequestValidationRule{
		Op: "required", Source: "header", Name: "anthropic-beta", CanonicalName: "Anthropic-Beta",
	}
	if err := Validate([]dslconfig.RequestValidationRule{required}, nil, headers, ""); err != nil {
		t.Fatalf("expected header pass, got %v", err)
	}

	missing := required
	missing.Name, missing.CanonicalName = "x-missing", "X-Missing"
	err := Validate([]dslconfig.RequestValidationRule{missing}, nil, headers, "")
	if err == nil || err.Source != "header" || err.PathOrName != "x-missing" {
		t.Fatalf("expected header failure, got %#v", err)
	}

	enum := dslconfig.RequestValidationRule{
		Op: "enum", Source: "header", Name: "anthropic-beta", CanonicalName: "Anthropic-Beta",
		StringValueSet: map[string]struct{}{"tools-2024": {}},
	}
	if err := Validate([]dslconfig.RequestValidationRule{enum}, nil, headers, ""); err != nil {
		t.Fatalf("expected header enum pass, got %v", err)
	}
	enum.StringValueSet = map[string]struct{}{"other": {}}
	if err := Validate([]dslconfig.RequestValidationRule{enum}, nil, headers, ""); err == nil {
		t.Fatalf("expected header enum failure")
	}

	lenRule := dslconfig.RequestValidationRule{
		Op: "len", Source: "header", Name: "anthropic-beta", CanonicalName: "Anthropic-Beta",
		MinLen: iptr(1), MaxLen: iptr(20),
	}
	if err := Validate([]dslconfig.RequestValidationRule{lenRule}, nil, headers, ""); err != nil {
		t.Fatalf("expected header len pass, got %v", err)
	}

	rangeRule := dslconfig.RequestValidationRule{
		Op: dslconfig.ReqRuleRange, Source: dslconfig.ReqValidationSourceHeader, Name: "x-budget", CanonicalName: "X-Budget",
		Min: f64(1), Max: f64(100),
	}
	if err := Validate([]dslconfig.RequestValidationRule{rangeRule}, nil, headers, ""); err != nil {
		t.Fatalf("expected header range pass, got %v", err)
	}
	rangeRule.Max = f64(10)
	if err := Validate([]dslconfig.RequestValidationRule{rangeRule}, nil, headers, ""); err == nil {
		t.Fatalf("expected header range failure")
	}
}

func TestQueryRules(t *testing.T) {
	rawQuery := "api-version=2024-06-01&x=1&budget=42"

	required := dslconfig.RequestValidationRule{Op: "required", Source: "query", Name: "api-version"}
	if err := Validate([]dslconfig.RequestValidationRule{required}, nil, nil, rawQuery); err != nil {
		t.Fatalf("expected query pass, got %v", err)
	}

	enum := dslconfig.RequestValidationRule{
		Op: "enum", Source: "query", Name: "api-version",
		StringValueSet: map[string]struct{}{"2024-06-01": {}, "2024-10-21": {}},
	}
	if err := Validate([]dslconfig.RequestValidationRule{enum}, nil, nil, rawQuery); err != nil {
		t.Fatalf("expected query enum pass, got %v", err)
	}
	enum.StringValueSet = map[string]struct{}{"2023-01-01": {}}
	err := Validate([]dslconfig.RequestValidationRule{enum}, nil, nil, rawQuery)
	if err == nil || err.Source != "query" {
		t.Fatalf("expected query enum failure, got %#v", err)
	}

	missing := dslconfig.RequestValidationRule{Op: "required", Source: "query", Name: "absent"}
	if err := Validate([]dslconfig.RequestValidationRule{missing}, nil, nil, rawQuery); err == nil {
		t.Fatalf("expected query required failure")
	}

	// Malformed query behaves as missing parameters.
	if err := Validate([]dslconfig.RequestValidationRule{missing}, nil, nil, "%zz=1"); err == nil {
		t.Fatalf("expected required failure on malformed query")
	}

	rangeRule := dslconfig.RequestValidationRule{Op: "range", Source: "query", Name: "budget", Min: f64(1), Max: f64(100)}
	if err := Validate([]dslconfig.RequestValidationRule{rangeRule}, nil, nil, rawQuery); err != nil {
		t.Fatalf("expected query range pass, got %v", err)
	}
	rangeRule.Max = f64(10)
	if err := Validate([]dslconfig.RequestValidationRule{rangeRule}, nil, nil, rawQuery); err == nil {
		t.Fatalf("expected query range failure")
	}
	if err := Validate([]dslconfig.RequestValidationRule{rangeRule}, nil, nil, "budget=NaN"); err == nil {
		t.Fatalf("expected query range failure for non-finite value")
	}
}

func TestBodyRulesRequireJSONRoot(t *testing.T) {
	err := Validate([]dslconfig.RequestValidationRule{bodyRule("type", "$.a")}, nil, nil, "")
	if err == nil || err.Rule != RuleJSONBody {
		t.Fatalf("expected json_body failure for nil root, got %#v", err)
	}

	// Header-only rules must not require a JSON body.
	headers := http.Header{"X-A": []string{"1"}}
	rule := dslconfig.RequestValidationRule{Op: "required", Source: "header", Name: "x-a", CanonicalName: "X-A"}
	if err := Validate([]dslconfig.RequestValidationRule{rule}, nil, headers, ""); err != nil {
		t.Fatalf("expected header-only pass with nil root, got %v", err)
	}
}

func TestFirstErrorShortCircuit(t *testing.T) {
	root := mustRoot(t, `{"b":"x"}`)
	rules := []dslconfig.RequestValidationRule{
		bodyRule("required", "$.a"),
		bodyRule("required", "$.also_missing"),
	}
	err := Validate(rules, root, nil, "")
	if err == nil || err.PathOrName != "$.a" {
		t.Fatalf("expected first rule failure, got %#v", err)
	}
}

func TestErrorOmitsFieldValue(t *testing.T) {
	const secret = "super-secret-value"
	root := map[string]any{"token": secret}
	rule := bodyRule("enum", "$.token")
	rule.LiteralValues = []any{"expected"}
	err := Validate([]dslconfig.RequestValidationRule{rule}, root, nil, "")
	if err == nil {
		t.Fatalf("expected enum failure")
	}
	if strings.Contains(err.Message, secret) || strings.Contains(err.Error(), secret) {
		t.Fatalf("error message leaks field value: %q", err.Message)
	}
}
