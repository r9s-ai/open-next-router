package dslconfig

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
)

type ProviderRouting struct {
	BaseURLExpr string
	Matches     []RoutingMatch
}

type RoutingMatch struct {
	API    string
	Stream *bool

	SetPath    string
	QueryPairs map[string]string
	QueryDels  []string
}

func (p ProviderRouting) Apply(meta *dslmeta.Meta) error {
	if meta == nil {
		return fmt.Errorf("meta is nil")
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return nil
	}
	match, ok := p.selectMatch(api, meta.IsStream)
	if !ok {
		return nil
	}

	if v := strings.TrimSpace(p.BaseURLExpr); v != "" {
		// Convention: provider base_url is the default. If meta.BaseURL is already set, keep it as higher priority.
		if strings.TrimSpace(meta.BaseURL) == "" {
			// v0.1: only constant string override is supported for now.
			meta.BaseURL = evalStringExpr(v, meta)
		}
	}

	u, err := url.Parse(meta.RequestURLPath)
	if err != nil {
		return fmt.Errorf("parse request url path %q: %w", meta.RequestURLPath, err)
	}
	if match.SetPath != "" {
		u.Path = evalStringExpr(match.SetPath, meta)
	}
	q := u.Query()
	for _, k := range match.QueryDels {
		if strings.TrimSpace(k) == "" {
			continue
		}
		q.Del(k)
	}
	for k, v := range match.QueryPairs {
		q.Set(k, evalStringExpr(v, meta))
	}
	u.RawQuery = q.Encode()
	meta.RequestURLPath = u.String()
	return nil
}

func (p ProviderRouting) HasMatch(meta *dslmeta.Meta) bool {
	if meta == nil {
		return false
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return false
	}
	_, ok := p.selectMatch(api, meta.IsStream)
	return ok
}

func (p ProviderRouting) selectMatch(api string, stream bool) (RoutingMatch, bool) {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return RoutingMatch{}, false
}

func evalStringExpr(expr string, meta *dslmeta.Meta) string {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return ""
	}
	// Minimal concat support used by auth_bearer implementation.
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")")
		parts := splitTopLevelArgs(inner)
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(evalStringExpr(p, meta))
		}
		return b.String()
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		return unquoteString(raw)
	}
	switch raw {
	case exprChannelBaseURL:
		return meta.BaseURL
	case exprChannelKey:
		return meta.APIKey
	case exprRequestModel:
		return meta.ActualModelName
	case exprRequestMapped:
		if meta.DSLModelMapped != "" {
			return meta.DSLModelMapped
		}
		return meta.ActualModelName
	default:
		return raw
	}
}

func splitTopLevelArgs(s string) []string {
	var parts []string
	var b strings.Builder
	depth := 0
	inString := false
	escaped := false
	flush := func() {
		p := strings.TrimSpace(b.String())
		b.Reset()
		if p != "" {
			parts = append(parts, p)
		}
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
			b.WriteByte(ch)
		case '(':
			depth++
			b.WriteByte(ch)
		case ')':
			if depth > 0 {
				depth--
			}
			b.WriteByte(ch)
		case ',':
			if depth == 0 {
				flush()
				continue
			}
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	flush()
	return parts
}
