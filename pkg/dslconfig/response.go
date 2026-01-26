package dslconfig

import (
	"strings"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
)

type ResponseDirective struct {
	Op   string
	Mode string
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
	if strings.TrimSpace(out.Op) == "" {
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
	return out
}
