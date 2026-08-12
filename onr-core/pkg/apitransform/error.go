package apitransform

import "fmt"

// UpstreamResponseError reports an upstream response that parsed correctly but
// carries no usable payload for the target schema (for example a Gemini image
// response with no inline image data). Builtins return it instead of emitting a
// success-shaped body so the failure keeps an upstream status class downstream
// rather than being reported as a client error.
type UpstreamResponseError struct {
	// StatusCode is the downstream HTTP status. Callers that cannot map it fall
	// back to their own default.
	StatusCode int
	// Type is the downstream error object "type" field.
	Type string
	// Code is the downstream error object "code" field.
	Code string
	// Message is a human-readable description without upstream payload content,
	// so it is safe for logs and client responses.
	Message string
}

func (e *UpstreamResponseError) Error() string {
	return fmt.Sprintf("upstream response error: %s", e.Message)
}

// Request mapping error codes. Values match the relay Go adaptors' channelmodel
// codes so clients see the same code on the DSL and Go routes.
const (
	CodeRequestInvalidParameter = "request_invalid_parameter"
	CodeRequestPromptMissing    = "request_prompt_missing"
	CodeRequestSizeNotSupported = "request_size_not_supported"
	CodeRequestNOutOfRange      = "request_n_out_of_range"

	CodeRequestMissingRequiredField = "request_missing_required_field"
)

// RequestMappingError reports a client request that a req_map builtin rejected,
// carrying the offending parameter and a stable code so callers can branch on
// the reason instead of parsing the message. Without it every rejection would
// collapse into the generic proxy_error code.
type RequestMappingError struct {
	// Code is a stable machine-readable reason, one of the CodeRequest*
	// constants above.
	Code string
	// Param is the request field that failed, used as the downstream error
	// object "param" field.
	Param string
	// Message is a human-readable description without the actual field value
	// beyond what the caller already sent, so it is safe for logs.
	Message string
}

func (e *RequestMappingError) Error() string {
	return e.Message
}

// newRequestMappingError builds a *RequestMappingError with a formatted message.
func newRequestMappingError(code, param, format string, args ...any) *RequestMappingError {
	return &RequestMappingError{
		Code:    code,
		Param:   param,
		Message: fmt.Sprintf(format, args...),
	}
}
