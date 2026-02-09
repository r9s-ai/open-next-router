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
}

type ResponseTokenDetails struct {
	CachedTokens     int `json:"cached_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}
