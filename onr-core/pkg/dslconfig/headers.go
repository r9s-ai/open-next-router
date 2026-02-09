package dslconfig

import (
	"net/http"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type ProviderHeaders struct {
	Defaults PhaseHeaders
	Matches  []MatchHeaders
}

type MatchHeaders struct {
	API    string
	Stream *bool

	Headers  PhaseHeaders
	Upstream UpstreamHeaders
}

type UpstreamHeaders struct {
	QueryPairs map[string]string
}

type PhaseHeaders struct {
	Auth    []HeaderOp
	Request []HeaderOp
	OAuth   OAuthConfig
}

type HeaderOp struct {
	Op string

	NameExpr  string
	ValueExpr string
}

func (p ProviderHeaders) Apply(meta *dslmeta.Meta, hdr http.Header) {
	if hdr == nil {
		return
	}
	phase, ok := p.Effective(meta)
	if !ok {
		return
	}
	for _, op := range append(append([]HeaderOp(nil), phase.Auth...), phase.Request...) {
		name := strings.TrimSpace(evalStringExpr(op.NameExpr, meta))
		switch op.Op {
		case "header_set":
			if name == "" {
				continue
			}
			hdr.Set(name, evalStringExpr(op.ValueExpr, meta))
		case "header_del":
			if name == "" {
				continue
			}
			hdr.Del(name)
		}
	}
}

func (p ProviderHeaders) Effective(meta *dslmeta.Meta) (PhaseHeaders, bool) {
	if meta == nil {
		return PhaseHeaders{}, false
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return PhaseHeaders{}, false
	}

	out := PhaseHeaders{
		Auth:    append([]HeaderOp(nil), p.Defaults.Auth...),
		Request: append([]HeaderOp(nil), p.Defaults.Request...),
		OAuth:   p.Defaults.OAuth,
	}

	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		out.Auth = append(out.Auth, m.Headers.Auth...)
		out.Request = append(out.Request, m.Headers.Request...)
		out.OAuth = out.OAuth.Merge(m.Headers.OAuth)
	}
	return out, true
}

func (p ProviderHeaders) selectMatch(api string, stream bool) (MatchHeaders, bool) {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return MatchHeaders{}, false
}
