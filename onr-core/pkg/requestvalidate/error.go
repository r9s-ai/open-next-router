// Package requestvalidate executes DSL request validation rules (req_required,
// req_forbid, req_type, req_range, req_len, req_enum) against the client
// request before request transforms run. It is HTTP-framework-free and relies
// on the execution plan fields compiled during config validation.
package requestvalidate

import "fmt"

// RequestValidationError reports one failed validation rule.
// Message never contains the actual field value, only the path, rule and
// expected condition, so it is safe for logs and client responses.
type RequestValidationError struct {
	// Source is body, header or query.
	Source string
	// PathOrName is the body object-path or header/query parameter name.
	PathOrName string
	// Rule is the failed rule kind (required/forbid/type/range/len/enum/json_body).
	Rule string
	// Message is a human-readable description without the actual field value.
	Message string
}

func (e *RequestValidationError) Error() string {
	return fmt.Sprintf("request validation failed: %s", e.Message)
}
