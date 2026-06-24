package dslmeta

import (
	"net/http"
	"sync"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
)

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

	// OAuthAccessToken is the runtime OAuth access token resolved by oauth directives.
	OAuthAccessToken string

	// OAuthCacheKey is the runtime cache identity for OAuth token refresh/invalidation.
	OAuthCacheKey string

	// CredentialFile is the local credential file path used by credential-aware providers.
	CredentialFile string

	// CredentialJSON is the credential JSON content supplied by the caller.
	CredentialJSON string

	// CredentialProjectID is the normalized project_id parsed from the credential.
	CredentialProjectID string

	// ChannelLocation is the provider location/region attached to the selected channel.
	ChannelLocation string

	// OriginModelName is the original request model.
	OriginModelName string

	// DSLModelMapped is the mapped model name after applying model_map.
	DSLModelMapped string

	// RequestURLPath is the request path (and query), e.g. "/v1/chat/completions?x=1".
	// DSL routing directives can rewrite it via set_path/set_query/del_query.
	RequestURLPath string

	// RequestContentType is the raw Content-Type of the incoming request body.
	RequestContentType string

	// RequestBody holds the original request body bytes when available.
	// It is used by request-side usage extraction.
	RequestBody []byte

	// RequestHeaders holds the original downstream request headers when available.
	RequestHeaders http.Header

	// DerivedUsage carries runtime-derived usage signals that are not directly
	// available from request/response JSON, such as audio duration from binary
	// response bodies.
	DerivedUsage map[string]any

	StartTime time.Time

	requestRootOnce sync.Once
	requestRoot     map[string]any
}

func Clone(src *Meta) *Meta {
	if src == nil {
		return &Meta{}
	}
	out := &Meta{
		API:                 src.API,
		IsStream:            src.IsStream,
		BaseURL:             src.BaseURL,
		APIKey:              src.APIKey,
		OAuthAccessToken:    src.OAuthAccessToken,
		OAuthCacheKey:       src.OAuthCacheKey,
		CredentialFile:      src.CredentialFile,
		CredentialJSON:      src.CredentialJSON,
		CredentialProjectID: src.CredentialProjectID,
		ChannelLocation:     src.ChannelLocation,
		OriginModelName:     src.OriginModelName,
		DSLModelMapped:      src.DSLModelMapped,
		RequestURLPath:      src.RequestURLPath,
		RequestContentType:  src.RequestContentType,
		RequestBody:         src.RequestBody,
		RequestHeaders:      cloneHeader(src.RequestHeaders),
		DerivedUsage:        cloneMap(src.DerivedUsage),
		StartTime:           src.StartTime,
	}
	if src.requestRoot != nil {
		out.SetRequestRoot(src.requestRoot)
	}
	return out
}

// RequestRoot requires a non-nil Meta receiver.
// It lazily parses and caches the request body as a request-side root object.
// It supports JSON objects and multipart form values.
func (m *Meta) RequestRoot() map[string]any {
	m.requestRootOnce.Do(func() {
		m.requestRoot = parseRequestRoot(m.RequestBody, m.RequestContentType)
	})
	return m.requestRoot
}

// SetRequestRoot requires a non-nil Meta receiver.
// It preloads the cached request-side root when it has already been parsed by an upstream caller.
func (m *Meta) SetRequestRoot(root map[string]any) {
	m.requestRoot = root
	m.requestRootOnce.Do(func() {})
}

func parseRequestRoot(body []byte, contentType string) map[string]any {
	return requestcanon.ParseRoot(body, contentType)
}

func cloneHeader(in http.Header) http.Header {
	if in == nil {
		return nil
	}
	out := make(http.Header, len(in))
	for k, vals := range in {
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
