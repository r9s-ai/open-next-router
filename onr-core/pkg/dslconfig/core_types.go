package dslconfig

// Usage is a minimal token usage structure extracted from upstream responses.
// It mirrors the fields used by next-router and common OpenAI-style responses.
type Usage struct {
	InputTokens      int `json:"input_tokens,omitempty"`
	OutputTokens     int `json:"output_tokens,omitempty"`
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`

	InputTokenDetails *ResponseTokenDetails `json:"input_tokens_details,omitempty"`

	FlatFields map[string]any `json:"-"`
	DebugFacts []UsageFact    `json:"-"`
}

type ResponseTokenDetails struct {
	CachedTokens     int `json:"cached_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

// UsageFact is the canonical usage item produced after extraction.
// It can also carry rule metadata for debug output.
type UsageFact struct {
	Dimension  string            `json:"dimension,omitempty"`
	Unit       string            `json:"unit,omitempty"`
	Quantity   float64           `json:"quantity,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Source     string            `json:"source,omitempty"`

	Fallback  bool   `json:"fallback,omitempty"`
	Path      string `json:"path,omitempty"`
	CountPath string `json:"count_path,omitempty"`
	SumPath   string `json:"sum_path,omitempty"`
	Expr      string `json:"expr,omitempty"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"`
}

// UsageExecutionPlan is the normalized internal plan used after usage_extract
// syntax sugar and compatibility-layer fields are compiled into explicit rules.
type UsageExecutionPlan struct {
	Mode            string      `json:"mode,omitempty"`
	Facts           []UsageFact `json:"facts,omitempty"`
	TotalTokensExpr string      `json:"total_tokens_expr,omitempty"`
}

type MatchUsageExecutionPlan struct {
	API    string             `json:"api,omitempty"`
	Stream *bool              `json:"stream,omitempty"`
	Plan   UsageExecutionPlan `json:"plan,omitempty"`
}

type ProviderUsageExecutionPlan struct {
	Defaults UsageExecutionPlan        `json:"defaults,omitempty"`
	Matches  []MatchUsageExecutionPlan `json:"matches,omitempty"`
}
