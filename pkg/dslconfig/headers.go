package dslconfig

import (
	"net/http"
	"strings"

	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
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
}

type HeaderOp struct {
	Op string

	NameExpr  string
	ValueExpr string
}

func (p ProviderHeaders) Apply(meta *dslmeta.Meta, hdr http.Header) {
	if meta == nil || hdr == nil {
		return
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return
	}

	ops := make([]HeaderOp, 0, len(p.Defaults.Auth)+len(p.Defaults.Request))
	ops = append(ops, p.Defaults.Auth...)
	ops = append(ops, p.Defaults.Request...)

	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		ops = append(ops, m.Headers.Auth...)
		ops = append(ops, m.Headers.Request...)
	}

	for _, op := range ops {
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
