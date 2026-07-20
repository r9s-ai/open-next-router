package dslconfig

import (
	"fmt"
	"math"
	"net/textproto"
	"strconv"
	"strings"
)

// validateRequestValidationRules validates parsed req_* rules and compiles their
// execution plan fields (PathParts, CanonicalName, LiteralValues, StringValueSet).
// It returns the resolved rules; runtime code relies on the compiled fields and
// must not re-parse Path or re-process Values per request.
func validateRequestValidationRules(path, providerName, scope string, rules []RequestValidationRule) ([]RequestValidationRule, error) {
	if len(rules) == 0 {
		return rules, nil
	}
	resolved := make([]RequestValidationRule, 0, len(rules))
	for i, rule := range rules {
		ruleScope := fmt.Sprintf("%s.req_rule[%d]", scope, i)
		fail := func(format string, args ...any) error {
			return fmt.Errorf("provider %q in %q: %s req_%s %s", providerName, path, ruleScope, rule.Op, fmt.Sprintf(format, args...))
		}

		if err := compileReqRuleSource(&rule, fail); err != nil {
			return nil, err
		}
		if err := validateReqRuleOp(&rule, fail); err != nil {
			return nil, err
		}
		resolved = append(resolved, rule)
	}
	return resolved, nil
}

func compileReqRuleSource(rule *RequestValidationRule, fail func(format string, args ...any) error) error {
	switch rule.Source {
	case ReqValidationSourceBody:
		parts, err := parseObjectPath(rule.Path)
		if err != nil {
			return fail("invalid body path: %v", err)
		}
		rule.PathParts = parts
	case ReqValidationSourceHeader, ReqValidationSourceQuery:
		if rule.Name == "" {
			return fail("requires a non-empty %s name", rule.Source)
		}
		if rule.Source == ReqValidationSourceHeader {
			rule.CanonicalName = textproto.CanonicalMIMEHeaderKey(rule.Name)
		}
	default:
		return fail("unsupported source %q; expected body, header or query", rule.Source)
	}
	return nil
}

func validateReqRuleOp(rule *RequestValidationRule, fail func(format string, args ...any) error) error {
	if rule.AllowNull && (rule.Op != ReqRuleRequired || rule.Source != ReqValidationSourceBody) {
		return fail("allow_null is only supported by req_required with body source")
	}
	switch rule.Op {
	case ReqRuleRequired, ReqRuleForbid:
		return nil
	case ReqRuleType:
		return validateReqRuleType(rule, fail)
	case ReqRuleRange:
		return validateReqRuleRange(rule, fail)
	case ReqRuleLen:
		return validateReqRuleLen(rule, fail)
	case ReqRuleEnum:
		return compileReqRuleEnum(rule, fail)
	default:
		// The parser only emits known ops; an unknown op here is a programming error.
		return fail("unsupported rule op %q", rule.Op)
	}
}

func validateReqRuleType(rule *RequestValidationRule, fail func(format string, args ...any) error) error {
	if rule.Source != ReqValidationSourceBody {
		return fail("only supports body source; header and query values are always strings")
	}
	switch rule.Type {
	case ReqTypeNull, ReqTypeBool, ReqTypeNumber, ReqTypeInteger, ReqTypeString, ReqTypeArray, ReqTypeObject:
		return nil
	default:
		return fail("unsupported type %q", rule.Type)
	}
}

func validateReqRuleRange(rule *RequestValidationRule, fail func(format string, args ...any) error) error {
	if rule.Min == nil && rule.Max == nil {
		return fail("requires at least one of min or max")
	}
	if (rule.Min != nil && !isFiniteReqNumber(*rule.Min)) || (rule.Max != nil && !isFiniteReqNumber(*rule.Max)) {
		return fail("min and max must be finite numbers")
	}
	if rule.Min != nil && rule.Max != nil && *rule.Min > *rule.Max {
		return fail("min must be <= max")
	}
	return nil
}

func validateReqRuleLen(rule *RequestValidationRule, fail func(format string, args ...any) error) error {
	if rule.MinLen == nil && rule.MaxLen == nil {
		return fail("requires at least one of min or max")
	}
	if (rule.MinLen != nil && *rule.MinLen < 0) || (rule.MaxLen != nil && *rule.MaxLen < 0) {
		return fail("min and max must be non-negative")
	}
	if rule.MinLen != nil && rule.MaxLen != nil && *rule.MinLen > *rule.MaxLen {
		return fail("min must be <= max")
	}
	return nil
}

// compileReqRuleEnum pre-processes enum candidates: body candidates become typed
// JSON literals in LiteralValues for allocation-free comparison; header/query
// candidates become a string set.
func compileReqRuleEnum(rule *RequestValidationRule, fail func(format string, args ...any) error) error {
	if len(rule.Values) == 0 {
		return fail("requires at least one value")
	}
	if rule.Source != ReqValidationSourceBody {
		set := make(map[string]struct{}, len(rule.Values))
		for _, v := range rule.Values {
			text := v
			if unquoted, err := strconv.Unquote(v); err == nil && strings.HasPrefix(v, `"`) {
				text = unquoted
			}
			set[text] = struct{}{}
		}
		rule.StringValueSet = set
		return nil
	}
	literals := make([]any, 0, len(rule.Values))
	for _, v := range rule.Values {
		if strings.HasPrefix(v, `"`) {
			text, err := strconv.Unquote(v)
			if err != nil {
				return fail("invalid string literal %s", v)
			}
			literals = append(literals, text)
			continue
		}
		switch v {
		case "true":
			literals = append(literals, true)
		case "false":
			literals = append(literals, false)
		case "null":
			literals = append(literals, nil)
		default:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || !isFiniteReqNumber(f) {
				return fail("invalid body enum literal %q; expected string, number, true, false or null", v)
			}
			literals = append(literals, f)
		}
	}
	rule.LiteralValues = literals
	return nil
}

func isFiniteReqNumber(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
