package dslconfig

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type ResponseDirective struct {
	Op   string
	Mode string

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

func (p ProviderResponse) Select(meta *dslmeta.Meta) (ResponseDirective, bool) {
	if meta == nil {
		return ResponseDirective{}, false
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return ResponseDirective{}, false
	}
	out := p.Defaults
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		out = mergeResponseDirective(out, m.Response)
	}
	if strings.TrimSpace(out.Op) == "" && len(out.JSONOps) == 0 && len(out.SSEJSONDelIf) == 0 {
		return ResponseDirective{}, false
	}
	return out, true
}

func (p ProviderResponse) selectMatch(api string, stream bool) (MatchResponse, bool) {
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
	if strings.TrimSpace(override.Op) != "" {
		out.Op = override.Op
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if len(override.SSEJSONDelIf) > 0 {
		out.SSEJSONDelIf = append(out.SSEJSONDelIf, override.SSEJSONDelIf...)
	}
	if len(override.JSONOps) > 0 {
		out.JSONOps = append(out.JSONOps, override.JSONOps...)
	}
	return out
}
