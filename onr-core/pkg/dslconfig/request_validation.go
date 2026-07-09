package dslconfig

// Request validation rule ops. Op holds the rule kind without the "req_" directive prefix.
const (
	ReqRuleRequired = "required"
	ReqRuleForbid   = "forbid"
	ReqRuleType     = "type"
	ReqRuleRange    = "range"
	ReqRuleLen      = "len"
	ReqRuleEnum     = "enum"
)

// Request validation rule sources.
const (
	ReqValidationSourceBody   = "body"
	ReqValidationSourceHeader = "header"
	ReqValidationSourceQuery  = "query"
)

// Request validation types accepted by req_type (body source only).
const (
	ReqTypeNull    = "null"
	ReqTypeBool    = "bool"
	ReqTypeNumber  = "number"
	ReqTypeInteger = "integer"
	ReqTypeString  = "string"
	ReqTypeArray   = "array"
	ReqTypeObject  = "object"
)

// RequestValidationRule is one req_* validation directive from the request phase.
//
// The raw directive fields (Op..AllowNull) are filled by the parser. The execution
// plan fields (PathParts, CanonicalName, LiteralValues, StringValueSet) are compiled
// once during config validation; runtime code must use them instead of re-parsing
// Path or re-processing Values per request.
type RequestValidationRule struct {
	// Op is one of the ReqRule* constants.
	Op string
	// Source is one of the ReqValidationSource* constants.
	Source string
	// Path is the body object-path (Source == body), e.g. "$.messages".
	Path string
	// Name is the header or query parameter name (Source == header|query).
	Name string
	// Type is the req_type target type, one of the ReqType* constants.
	Type string
	// Min/Max are req_range bounds; at least one is set for a valid range rule.
	Min *float64
	Max *float64
	// MinLen/MaxLen are req_len bounds; at least one is set for a valid len rule.
	MinLen *int
	MaxLen *int
	// Values holds req_enum candidates as JSON literal text: quoted strings keep
	// canonical double quotes (strconv.Quote), bare literals keep their raw text.
	Values []string
	// AllowNull lets req_required accept JSON null (body source only).
	AllowNull bool

	// PathParts is the pre-parsed Path (Source == body).
	PathParts []string
	// CanonicalName is the pre-canonicalized header name (Source == header).
	CanonicalName string
	// LiteralValues holds body enum candidates as typed Go values
	// (string / float64 / bool / nil) for allocation-free runtime comparison.
	LiteralValues []any
	// StringValueSet holds header/query enum candidates as a string set.
	StringValueSet map[string]struct{}
}
