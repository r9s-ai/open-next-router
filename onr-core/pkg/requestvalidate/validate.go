package requestvalidate

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

// RuleJSONBody is the error Rule reported when body rules are configured but the
// request has no parsed JSON object root.
const RuleJSONBody = "json_body"

// Validate runs rules in order and returns the first failure, or nil when all
// rules pass. Rules must be config-validated dslconfig rules with compiled
// execution plan fields; root is the parsed request JSON object root (nil for
// non-JSON bodies); headers may be nil; rawQuery is the client's original URL
// query string before routing rewrites, parsed lazily only when query rules exist.
//
// The success path performs no allocations and no error string formatting.
func Validate(rules []dslconfig.RequestValidationRule, root map[string]any, headers http.Header, rawQuery string) *RequestValidationError {
	var query url.Values
	queryParsed := false
	for i := range rules {
		rule := &rules[i]
		var (
			value  any
			exists bool
		)
		switch rule.Source {
		case dslconfig.ReqValidationSourceBody:
			if root == nil {
				return &RequestValidationError{
					Source:     rule.Source,
					PathOrName: rule.Path,
					Rule:       RuleJSONBody,
					Message:    fmt.Sprintf("%s validation requires a JSON object body", rule.Path),
				}
			}
			value, exists = lookupBody(root, rule.PathParts)
		case dslconfig.ReqValidationSourceHeader:
			value, exists = lookupHeader(headers, rule.CanonicalName)
		case dslconfig.ReqValidationSourceQuery:
			if !queryParsed {
				// Ignore malformed query strings: unparseable parameters behave as missing.
				query, _ = url.ParseQuery(rawQuery)
				queryParsed = true
			}
			value, exists = lookupQuery(query, rule.Name)
		}
		if err := checkRule(rule, value, exists); err != nil {
			return err
		}
	}
	return nil
}

func checkRule(rule *dslconfig.RequestValidationRule, value any, exists bool) *RequestValidationError {
	switch rule.Op {
	case dslconfig.ReqRuleRequired:
		return checkRequired(rule, value, exists)
	case dslconfig.ReqRuleForbid:
		if exists {
			return failRule(rule, "must not be present")
		}
		return nil
	}
	// Remaining rules are no-ops for missing targets.
	if !exists {
		return nil
	}
	switch rule.Op {
	case dslconfig.ReqRuleType:
		return checkType(rule, value)
	case dslconfig.ReqRuleRange:
		return checkRange(rule, value)
	case dslconfig.ReqRuleLen:
		return checkLen(rule, value)
	case dslconfig.ReqRuleEnum:
		return checkEnum(rule, value)
	}
	// Config validation only emits known ops; reaching here is a programming error.
	panic("requestvalidate: unknown rule op " + rule.Op)
}

func checkRequired(rule *dslconfig.RequestValidationRule, value any, exists bool) *RequestValidationError {
	if !exists {
		return failRule(rule, "is required")
	}
	if value == nil && rule.Source == dslconfig.ReqValidationSourceBody && !rule.AllowNull {
		return failRule(rule, "must not be null")
	}
	return nil
}

func checkType(rule *dslconfig.RequestValidationRule, value any) *RequestValidationError {
	if jsonTypeMatches(value, rule.Type) {
		return nil
	}
	return failRule(rule, "must be of type "+rule.Type)
}

func jsonTypeMatches(value any, typ string) bool {
	switch typ {
	case dslconfig.ReqTypeNull:
		return value == nil
	case dslconfig.ReqTypeBool:
		_, ok := value.(bool)
		return ok
	case dslconfig.ReqTypeNumber:
		_, ok := value.(float64)
		return ok
	case dslconfig.ReqTypeInteger:
		f, ok := value.(float64)
		return ok && f == math.Trunc(f)
	case dslconfig.ReqTypeString:
		_, ok := value.(string)
		return ok
	case dslconfig.ReqTypeArray:
		_, ok := value.([]any)
		return ok
	case dslconfig.ReqTypeObject:
		_, ok := value.(map[string]any)
		return ok
	}
	return false
}

func checkRange(rule *dslconfig.RequestValidationRule, value any) *RequestValidationError {
	f, ok := rangeNumber(value)
	if !ok {
		return failRule(rule, "must be a number")
	}
	if rule.Min != nil && f < *rule.Min {
		return failRangeBound(rule)
	}
	if rule.Max != nil && f > *rule.Max {
		return failRangeBound(rule)
	}
	return nil
}

func rangeNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func failRangeBound(rule *dslconfig.RequestValidationRule) *RequestValidationError {
	switch {
	case rule.Min != nil && rule.Max != nil:
		return failRule(rule, fmt.Sprintf("must be between %v and %v", *rule.Min, *rule.Max))
	case rule.Min != nil:
		return failRule(rule, fmt.Sprintf("must be >= %v", *rule.Min))
	default:
		return failRule(rule, fmt.Sprintf("must be <= %v", *rule.Max))
	}
}

func checkLen(rule *dslconfig.RequestValidationRule, value any) *RequestValidationError {
	var length int
	switch v := value.(type) {
	case string:
		// Length counts Unicode code points, not bytes.
		length = utf8.RuneCountInString(v)
	case []any:
		length = len(v)
	default:
		return failRule(rule, "must be a string or array")
	}
	if rule.MinLen != nil && length < *rule.MinLen {
		return failLenBound(rule)
	}
	if rule.MaxLen != nil && length > *rule.MaxLen {
		return failLenBound(rule)
	}
	return nil
}

func failLenBound(rule *dslconfig.RequestValidationRule) *RequestValidationError {
	switch {
	case rule.MinLen != nil && rule.MaxLen != nil:
		return failRule(rule, fmt.Sprintf("length must be between %d and %d", *rule.MinLen, *rule.MaxLen))
	case rule.MinLen != nil:
		return failRule(rule, fmt.Sprintf("length must be >= %d", *rule.MinLen))
	default:
		return failRule(rule, fmt.Sprintf("length must be <= %d", *rule.MaxLen))
	}
}

func checkEnum(rule *dslconfig.RequestValidationRule, value any) *RequestValidationError {
	if rule.Source == dslconfig.ReqValidationSourceBody {
		if bodyEnumMatches(value, rule.LiteralValues) {
			return nil
		}
	} else {
		s, ok := value.(string)
		if ok {
			if _, found := rule.StringValueSet[s]; found {
				return nil
			}
		}
	}
	return failRule(rule, "must be one of the allowed values")
}

// bodyEnumMatches compares the request value against pre-parsed typed literals.
// Enum candidate lists are small, so a linear typed scan avoids the per-request
// formatting a map keyed by canonical strings would require.
func bodyEnumMatches(value any, literals []any) bool {
	for _, lit := range literals {
		switch expected := lit.(type) {
		case nil:
			if value == nil {
				return true
			}
		case string:
			if v, ok := value.(string); ok && v == expected {
				return true
			}
		case bool:
			if v, ok := value.(bool); ok && v == expected {
				return true
			}
		case float64:
			if v, ok := value.(float64); ok && v == expected {
				return true
			}
		}
	}
	return false
}

func failRule(rule *dslconfig.RequestValidationRule, condition string) *RequestValidationError {
	pathOrName := rule.Path
	if rule.Source != dslconfig.ReqValidationSourceBody {
		pathOrName = rule.Name
	}
	return &RequestValidationError{
		Source:     rule.Source,
		PathOrName: pathOrName,
		Rule:       rule.Op,
		Message:    fmt.Sprintf("%s %s", pathOrName, condition),
	}
}
