package dslmetadata

const SchemaVersionV1 = "dslmetadata/v1"

type ProviderConfig struct {
	Metadata   *ProviderMetadata   `json:"metadata,omitempty"`
	Auth       *ProviderAuth       `json:"auth,omitempty"`
	Upstream   *ProviderUpstream   `json:"upstream,omitempty"`
	Routes     []ProviderRoute     `json:"routes,omitempty"`
	Request    *ProviderRequest    `json:"request,omitempty"`
	Models     *ProviderModels     `json:"models,omitempty"`
	Balance    *ProviderBalance    `json:"balance,omitempty"`
	UsageFacts *ProviderUsageFacts `json:"usage_facts,omitempty"`
}

type ProviderMetadata struct {
	ProviderFamily string `json:"providerFamily,omitempty"`
	SignalProfile  string `json:"signalProfile,omitempty"`
}

type ProviderAuth struct {
	Type           string `json:"type,omitempty"`
	Header         string `json:"header,omitempty"`
	Mode           string `json:"mode,omitempty"`
	Scope          string `json:"scope,omitempty"`
	TokenURL       string `json:"token_url,omitempty"`
	Service        string `json:"service,omitempty"`
	Credentials    string `json:"credentials,omitempty"`
	RequiresRegion bool   `json:"requires_region,omitempty"`
}

type ProviderUpstream struct {
	Transport string `json:"transport,omitempty"`
}

type ProviderRoute struct {
	API    string `json:"api"`
	Stream *bool  `json:"stream,omitempty"`
	Path   string `json:"path"`
}

type ProviderRequest struct {
	Defaults RequestTransform        `json:"defaults,omitempty"`
	Matches  []RequestTransformMatch `json:"matches,omitempty"`
}

type RequestTransformMatch struct {
	API       string           `json:"api"`
	Stream    *bool            `json:"stream,omitempty"`
	Transform RequestTransform `json:"transform"`
}

type RequestTransform struct {
	ModelMap           ModelMap                `json:"model_map,omitempty"`
	ValidationRules    []RequestValidationRule `json:"validation_rules,omitempty"`
	JSONOps            []JSONOp                `json:"json_ops,omitempty"`
	AfterReqMapJSONOps []JSONOp                `json:"after_req_map_json_ops,omitempty"`
	ReqMapMode         string                  `json:"req_map_mode,omitempty"`
}

// RequestValidationRule mirrors the raw req_* directive fields from dslconfig.
// Compiled execution plan fields are intentionally not exported.
type RequestValidationRule struct {
	Op     string `json:"op"`
	Source string `json:"source"`

	Path      string   `json:"path,omitempty"`
	Name      string   `json:"name,omitempty"`
	Type      string   `json:"type,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MinLen    *int     `json:"min_len,omitempty"`
	MaxLen    *int     `json:"max_len,omitempty"`
	Values    []string `json:"values,omitempty"`
	AllowNull bool     `json:"allow_null,omitempty"`
}

type ModelMap struct {
	Map         map[string]string `json:"map,omitempty"`
	DefaultExpr string            `json:"default_expr,omitempty"`
}

type JSONOp struct {
	Op string `json:"op"`

	Path          string   `json:"path,omitempty"`
	FromPath      string   `json:"from_path,omitempty"`
	ToPath        string   `json:"to_path,omitempty"`
	ValueExpr     string   `json:"value_expr,omitempty"`
	HeaderName    string   `json:"header_name,omitempty"`
	FieldName     string   `json:"field_name,omitempty"`
	Patterns      []string `json:"patterns,omitempty"`
	Separator     string   `json:"separator,omitempty"`
	Event         string   `json:"event,omitempty"`
	EventOptional bool     `json:"event_optional,omitempty"`
	MaxCount      int      `json:"max_count,omitempty"`
}

type HeaderOp struct {
	Op string `json:"op"`

	NameExpr  string   `json:"name_expr,omitempty"`
	ValueExpr string   `json:"value_expr,omitempty"`
	Patterns  []string `json:"patterns,omitempty"`
	Separator string   `json:"separator,omitempty"`
}

type ProviderModels struct {
	Mode string `json:"mode,omitempty"`

	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`

	IDPaths      []string `json:"id_paths,omitempty"`
	IDRegex      string   `json:"id_regex,omitempty"`
	IDAllowRegex string   `json:"id_allow_regex,omitempty"`

	Headers []HeaderOp `json:"headers,omitempty"`
}

type ProviderBalance struct {
	Mode string `json:"mode,omitempty"`

	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`

	BalancePath string `json:"balance_path,omitempty"`
	BalanceExpr string `json:"balance_expr,omitempty"`
	UsedPath    string `json:"used_path,omitempty"`
	UsedExpr    string `json:"used_expr,omitempty"`

	Unit string `json:"unit,omitempty"`

	SubscriptionPath string `json:"subscription_path,omitempty"`
	UsagePath        string `json:"usage_path,omitempty"`

	Headers []HeaderOp `json:"headers,omitempty"`
}

type ProviderUsageFacts struct {
	Defaults []UsageFact      `json:"defaults,omitempty"`
	Matches  []UsageFactMatch `json:"matches,omitempty"`
}

type UsageFactMatch struct {
	API    string      `json:"api"`
	Stream *bool       `json:"stream,omitempty"`
	Facts  []UsageFact `json:"facts,omitempty"`
}

type UsageFact struct {
	Dimension  string            `json:"dimension"`
	Unit       string            `json:"unit"`
	Source     string            `json:"source,omitempty"`
	Path       string            `json:"path,omitempty"`
	CountPath  string            `json:"count_path,omitempty"`
	SumPath    string            `json:"sum_path,omitempty"`
	LenPath    string            `json:"len_path,omitempty"`
	Expr       string            `json:"expr,omitempty"`
	Type       string            `json:"type,omitempty"`
	Status     string            `json:"status,omitempty"`
	Event      string            `json:"event,omitempty"`
	Fallback   bool              `json:"fallback,omitempty"`
	WhenPath   string            `json:"when_path,omitempty"`
	WhenEq     string            `json:"when_eq,omitempty"`
	Scale      float64           `json:"scale,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}
