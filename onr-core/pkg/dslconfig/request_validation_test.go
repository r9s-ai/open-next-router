package dslconfig

import (
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func parseValidationConf(t *testing.T, requestBody string) ProviderRequestTransform {
	t.Helper()
	conf := `
syntax "next-router/0.1";

provider "demo" {
  defaults {
    request {
` + requestBody + `
    }
  }
}
`
	_, _, req, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	return req
}

func validateValidationConf(t *testing.T, requestBody string) (ProviderRequestTransform, error) {
	t.Helper()
	req := parseValidationConf(t, requestBody)
	return validateProviderRequestTransform("demo.conf", "demo", req)
}

func mustValidateRules(t *testing.T, requestBody string) []RequestValidationRule {
	t.Helper()
	resolved, err := validateValidationConf(t, requestBody)
	if err != nil {
		t.Fatalf("validateProviderRequestTransform: %v", err)
	}
	return resolved.Defaults.ValidationRules
}

func TestParseRequestValidationDirectives(t *testing.T) {
	rules := mustValidateRules(t, `
      req_required body "$.model";
      req_required body "$.metadata" allow_null=true;
      req_forbid body "$.legacy_field";
      req_type body "$.messages" array;
      req_range body "$.temperature" min=0 max=2;
      req_range body "$.penalty" min=-2.5;
      req_len body "$.messages" min=1 max=256;
      req_enum body "$.reasoning_effort" "low" "medium" "high";
      req_enum body "$.stream" true false;
      req_enum body "$.n" 1 2;
      req_required header "anthropic-beta";
      req_enum query "api-version" "2024-06-01" "2024-10-21";
`)
	if len(rules) != 12 {
		t.Fatalf("expected 12 rules, got %d: %#v", len(rules), rules)
	}
	assertRequestValidationPresenceRules(t, rules)
	assertRequestValidationBoundsRules(t, rules)
	assertRequestValidationEnumRules(t, rules)
	assertRequestValidationHeaderQueryRules(t, rules)
}

func assertRequestValidationPresenceRules(t *testing.T, rules []RequestValidationRule) {
	t.Helper()
	r := rules[0]
	if r.Op != ReqRuleRequired || r.Source != ReqValidationSourceBody || r.Path != "$.model" || r.AllowNull {
		t.Fatalf("unexpected rule[0]: %#v", r)
	}
	if len(r.PathParts) != 1 || r.PathParts[0] != "model" {
		t.Fatalf("unexpected rule[0].PathParts: %#v", r.PathParts)
	}

	if !rules[1].AllowNull {
		t.Fatalf("expected rule[1] allow_null=true: %#v", rules[1])
	}
	if rules[2].Op != ReqRuleForbid {
		t.Fatalf("unexpected rule[2]: %#v", rules[2])
	}
	if rules[3].Op != ReqRuleType || rules[3].Type != ReqTypeArray {
		t.Fatalf("unexpected rule[3]: %#v", rules[3])
	}
}

func assertRequestValidationBoundsRules(t *testing.T, rules []RequestValidationRule) {
	t.Helper()
	r := rules[4]
	if r.Op != ReqRuleRange || r.Min == nil || *r.Min != 0 || r.Max == nil || *r.Max != 2 {
		t.Fatalf("unexpected rule[4]: %#v", r)
	}
	r = rules[5]
	if r.Min == nil || *r.Min != -2.5 || r.Max != nil {
		t.Fatalf("unexpected rule[5]: %#v", r)
	}
	r = rules[6]
	if r.Op != ReqRuleLen || r.MinLen == nil || *r.MinLen != 1 || r.MaxLen == nil || *r.MaxLen != 256 {
		t.Fatalf("unexpected rule[6]: %#v", r)
	}
}

func assertRequestValidationEnumRules(t *testing.T, rules []RequestValidationRule) {
	t.Helper()
	r := rules[7]
	if r.Op != ReqRuleEnum || len(r.LiteralValues) != 3 || r.LiteralValues[0] != "low" || r.LiteralValues[2] != "high" {
		t.Fatalf("unexpected rule[7]: %#v", r)
	}
	r = rules[8]
	if len(r.LiteralValues) != 2 || r.LiteralValues[0] != true || r.LiteralValues[1] != false {
		t.Fatalf("unexpected rule[8]: %#v", r)
	}
	r = rules[9]
	if len(r.LiteralValues) != 2 || r.LiteralValues[0] != float64(1) || r.LiteralValues[1] != float64(2) {
		t.Fatalf("unexpected rule[9]: %#v", r)
	}
}

func assertRequestValidationHeaderQueryRules(t *testing.T, rules []RequestValidationRule) {
	t.Helper()
	r := rules[10]
	if r.Source != ReqValidationSourceHeader || r.Name != "anthropic-beta" || r.CanonicalName != "Anthropic-Beta" {
		t.Fatalf("unexpected rule[10]: %#v", r)
	}
	r = rules[11]
	if r.Source != ReqValidationSourceQuery || r.Name != "api-version" {
		t.Fatalf("unexpected rule[11]: %#v", r)
	}
	if _, ok := r.StringValueSet["2024-06-01"]; !ok || len(r.StringValueSet) != 2 {
		t.Fatalf("unexpected rule[11].StringValueSet: %#v", r.StringValueSet)
	}
}

func TestRequestValidationConfigErrors(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"bad source", `req_required cookie "$.model";`, "unsupported source"},
		{"bad body path", `req_required body "model";`, "invalid body path"},
		{"array path unsupported", `req_required body "$.messages[0]";`, "invalid body path"},
		{"bad type", `req_type body "$.a" float;`, "unsupported type"},
		{"type on header", `req_type header "x-a" string;`, "only supports body source"},
		{"range without bounds", `req_range body "$.a";`, "at least one of min or max"},
		{"range min gt max", `req_range body "$.a" min=2 max=1;`, "min must be <= max"},
		{"len negative", `req_len body "$.a" min=-1;`, "non-negative"},
		{"len min gt max", `req_len body "$.a" min=3 max=2;`, "min must be <= max"},
		{"enum empty", `req_enum body "$.a";`, "at least one value"},
		{"enum bad literal", `req_enum body "$.a" bogus;`, "invalid body enum literal"},
		{"enum non finite literal", `req_enum body "$.a" NaN;`, "invalid body enum literal"},
		{"allow_null on header", `req_required header "x-a" allow_null=true;`, "only supported by req_required with body source"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateValidationConf(t, tc.body)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestRequestValidationSyntaxErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"range bad number", `req_range body "$.a" min=abc;`},
		{"range non finite number", `req_range body "$.a" min=NaN;`},
		{"range scientific notation", `req_range body "$.a" min=1e3;`},
		{"len float", `req_len body "$.a" min=1.5;`},
		{"required extra arg", `req_required body "$.a" bogus;`},
		{"allow_null on forbid", `req_forbid body "$.a" allow_null=true;`},
		{"missing target", `req_required body;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := `
syntax "next-router/0.1";

provider "demo" {
  defaults {
    request {
` + tc.body + `
    }
  }
}
`
			_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
			if err == nil {
				t.Fatalf("expected parse error, got nil")
			}
		})
	}
}

func TestRequestValidationMergeOrderAndSelect(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "demo" {
  defaults {
    request {
      req_required body "$.model";
    }
  }

  match api = "chat.completions" {
    request {
      req_required body "$.messages";
      req_type body "$.messages" array;
    }
  }
}
`
	_, _, req, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	resolved, err := validateProviderRequestTransform("demo.conf", "demo", req)
	if err != nil {
		t.Fatalf("validateProviderRequestTransform: %v", err)
	}

	m := &dslmeta.Meta{API: "chat.completions"}
	tcfg, ok := resolved.Select(m)
	if !ok {
		t.Fatalf("expected validation-only transform to be selected")
	}
	rules := tcfg.ValidationRules
	if len(rules) != 3 {
		t.Fatalf("expected 3 merged rules, got %d", len(rules))
	}
	if rules[0].Path != "$.model" || rules[1].Path != "$.messages" || rules[2].Op != ReqRuleType {
		t.Fatalf("unexpected merged rule order: %#v", rules)
	}
}

func TestRequestValidationMetadataExport(t *testing.T) {
	rules := mustValidateRules(t, `
      req_range body "$.temperature" min=0 max=2;
      req_enum body "$.effort" "low" "high";
`)
	exported := exportValidationRules(rules)
	if len(exported) != 2 {
		t.Fatalf("expected 2 exported rules, got %d", len(exported))
	}
	if exported[0].Op != ReqRuleRange || exported[0].Min == nil || *exported[0].Min != 0 {
		t.Fatalf("unexpected exported rule[0]: %#v", exported[0])
	}
	if len(exported[1].Values) != 2 || exported[1].Values[0] != `"low"` {
		t.Fatalf("unexpected exported rule[1] values: %#v", exported[1].Values)
	}
}
