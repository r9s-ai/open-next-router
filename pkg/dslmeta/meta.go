package dslmeta

import "time"

// Meta is the minimal context required by the DSL engine.
// It is intentionally small to keep open-next-router decoupled from other projects.
type Meta struct {
	// API is the logical API type, e.g. "chat.completions" / "claude.messages".
	API string

	// IsStream indicates whether the request is streaming.
	IsStream bool

	// BaseURL is the upstream base URL. If empty, the provider config default is used.
	BaseURL string

	// APIKey is the selected upstream key (token) used by auth directives.
	APIKey string

	// ActualModelName is the original request model.
	ActualModelName string

	// DSLModelMapped is the mapped model name after applying model_map.
	DSLModelMapped string

	// RequestURLPath is the request path (and query), e.g. "/v1/chat/completions?x=1".
	// DSL routing directives can rewrite it via set_path/set_query/del_query.
	RequestURLPath string

	StartTime time.Time
}
