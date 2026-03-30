package dslmeta

import (
	"bytes"
	"encoding/json"
	"mime"
	"mime/multipart"
	"strings"
	"sync"
	"time"
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

	// ActualModelName is the original request model.
	ActualModelName string

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

	// DerivedUsage carries runtime-derived usage signals that are not directly
	// available from request/response JSON, such as audio duration from binary
	// response bodies.
	DerivedUsage map[string]any

	StartTime time.Time

	requestRootOnce sync.Once
	requestRoot     map[string]any
}

// RequestRoot lazily parses and caches the request body as a request-side root object.
// It supports JSON objects and multipart form values.
func (m *Meta) RequestRoot() map[string]any {
	if m == nil {
		return nil
	}
	m.requestRootOnce.Do(func() {
		m.requestRoot = parseRequestRoot(m.RequestBody, m.RequestContentType)
	})
	return m.requestRoot
}

// SetRequestRoot preloads the cached request-side root when it has already been
// parsed by an upstream caller.
func (m *Meta) SetRequestRoot(root map[string]any) {
	if m == nil {
		return
	}
	m.requestRoot = root
	m.requestRootOnce.Do(func() {})
}

func parseRequestRoot(body []byte, contentType string) map[string]any {
	if len(body) == 0 {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data") {
		return parseMultipartRequestRoot(body, contentType)
	}
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}
	root, _ := data.(map[string]any)
	return root
}

func parseMultipartRequestRoot(body []byte, contentType string) map[string]any {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return nil
	}
	defer form.RemoveAll()

	root := make(map[string]any)
	for k, vals := range form.Value {
		switch len(vals) {
		case 0:
			continue
		case 1:
			root[k] = vals[0]
		default:
			items := make([]any, 0, len(vals))
			for _, v := range vals {
				items = append(items, v)
			}
			root[k] = items
		}
	}
	if len(root) == 0 {
		return nil
	}
	return root
}
