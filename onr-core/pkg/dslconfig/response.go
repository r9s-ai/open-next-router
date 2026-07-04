package dslconfig

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type ResponseDirective struct {
	Op   string
	Mode string

	// SSECollectMode collects an upstream SSE response into the same upstream
	// protocol's non-stream JSON object before optional resp_map/JSONOps.
	SSECollectMode string

	// JSONOps are optional JSON mutations applied to the downstream response body (best-effort).
	//
	// - Non-stream: applied to the whole JSON object response.
	// - Stream (SSE): applied to each event's joined "data:" JSON object payload.
	//
	// Ops are additive and executed in order.
	JSONOps []JSONOp

	// SSEJSONDelIf rules apply only to SSE streams. When the condition matches,
	// the del path is deleted from the event JSON object payload.
	//
	// Rules are executed in order, before JSONOps.
	SSEJSONDelIf []SSEJSONDelIfRule

	// BodyExtract decodes one JSON response field into the raw downstream body
	// (non-stream). Usage/error extraction must read the original JSON before
	// the body transform is applied.
	BodyExtract *RespBodyExtractRule

	// ContentTypeRule resolves the downstream Content-Type from a JSON response
	// field (non-stream, used together with BodyExtract).
	ContentTypeRule *RespContentTypeRule

	// SSEBinaryExtract converts upstream SSE JSON chunks into a raw binary
	// downstream stream (e.g. hex audio chunks -> binary audio stream).
	SSEBinaryExtract *SSEBinaryExtractRule

	// ErrorWhen rules detect in-body upstream errors on HTTP 2xx responses
	// (error phase). Matching responses are normalized via error_map.
	ErrorWhen []ErrorWhenRule
}

// RespBodyExtractRule describes resp_body_extract: read the string at Path from
// the response JSON object and decode it into the downstream binary body.
type RespBodyExtractRule struct {
	Path   string
	Decode string // "hex" or "base64"
}

// RespContentTypeRule describes resp_content_type: resolve downstream
// Content-Type from the string at FromPath, interpreted per Kind.
type RespContentTypeRule struct {
	FromPath string
	Kind     string // "audio": value is an audio format name (mp3/wav/...)
	Default  string // fallback format when FromPath is missing/empty
}

// SSEBinaryExtractRule describes sse_binary_extract: for each SSE JSON chunk,
// decode the string at Path and write it to the downstream body as raw bytes.
// When the value at StopPath equals StopEquals, the stream ends.
type SSEBinaryExtractRule struct {
	Path       string
	Decode     string // "hex" or "base64"
	StopPath   string
	StopEquals string
}

// ErrorWhenRule describes error_when: on HTTP 2xx responses, when the value at
// Path matches (Equals) or differs from (NotEquals) the reference value, the
// response is treated as an upstream error with the given downstream Status.
// Exactly one of Equals/NotEquals is set by the parser.
type ErrorWhenRule struct {
	Path      string
	Equals    string
	NotEquals string
	Status    int // downstream HTTP status; parser defaults to 400
}

type ProviderResponse struct {
	Defaults ResponseDirective
	Matches  []MatchResponse
}

type MatchResponse struct {
	API    string
	Stream *bool

	Response ResponseDirective
}

// Select requires a non-nil meta and a valid ProviderResponse receiver.
// It returns a request-scoped copy assembled from defaults and the matched override.
// Callers must treat the shared provider config as read-only across requests.
func (p *ProviderResponse) Select(meta *dslmeta.Meta) (*ResponseDirective, bool) {
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return nil, false
	}
	out := p.Defaults
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		out = mergeResponseDirective(out, m.Response)
	}
	if strings.TrimSpace(out.Op) == "" && strings.TrimSpace(out.SSECollectMode) == "" && len(out.JSONOps) == 0 && len(out.SSEJSONDelIf) == 0 &&
		out.BodyExtract == nil && out.ContentTypeRule == nil && out.SSEBinaryExtract == nil && len(out.ErrorWhen) == 0 {
		return nil, false
	}
	return &out, true
}

func (p *ProviderResponse) selectMatch(api string, stream bool) (MatchResponse, bool) {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return MatchResponse{}, false
}

func mergeResponseDirective(base ResponseDirective, override ResponseDirective) ResponseDirective {
	out := base
	if len(base.SSEJSONDelIf) > 0 {
		out.SSEJSONDelIf = append([]SSEJSONDelIfRule(nil), base.SSEJSONDelIf...)
	}
	if len(base.JSONOps) > 0 {
		out.JSONOps = append([]JSONOp(nil), base.JSONOps...)
	}
	if strings.TrimSpace(override.Op) != "" {
		out.Op = override.Op
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.SSECollectMode) != "" {
		out.SSECollectMode = override.SSECollectMode
		if strings.TrimSpace(out.Op) == "resp_passthrough" {
			out.Op = ""
			out.Mode = ""
		}
	}
	if len(override.SSEJSONDelIf) > 0 {
		out.SSEJSONDelIf = append(out.SSEJSONDelIf, override.SSEJSONDelIf...)
	}
	if len(override.JSONOps) > 0 {
		out.JSONOps = append(out.JSONOps, override.JSONOps...)
	}
	if len(base.ErrorWhen) > 0 {
		out.ErrorWhen = append([]ErrorWhenRule(nil), base.ErrorWhen...)
	}
	if len(override.ErrorWhen) > 0 {
		out.ErrorWhen = append(out.ErrorWhen, override.ErrorWhen...)
	}
	if override.BodyExtract != nil {
		out.BodyExtract = override.BodyExtract
	}
	if override.ContentTypeRule != nil {
		out.ContentTypeRule = override.ContentTypeRule
	}
	if override.SSEBinaryExtract != nil {
		out.SSEBinaryExtract = override.SSEBinaryExtract
	}
	return out
}
