package dslconfig

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
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

// Apply requires a non-nil meta and a valid ProviderRouting receiver.
func (p *ProviderRouting) Apply(meta *dslmeta.Meta) error {
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

// HasMatchAPI requires a valid ProviderRouting receiver.
func (p *ProviderRouting) HasMatchAPI(api string) bool {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		return true
	}
	return false
}

// HasMatch requires a non-nil meta and reports whether it matches any configured routing rule.
// It returns false when meta.API is empty.
func (p *ProviderRouting) HasMatch(meta *dslmeta.Meta) bool {
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return false
	}
	_, ok := p.selectMatch(api, meta.IsStream)
	return ok
}

func (p *ProviderRouting) selectMatch(api string, stream bool) (RoutingMatch, bool) {
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
	if strings.HasPrefix(raw, "template(") && strings.HasSuffix(raw, ")") {
		args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(raw, "template("), ")"))
		if len(args) != 1 || !isQuotedStringExpr(args[0]) {
			return raw
		}
		return evalTemplateString(unquoteString(strings.TrimSpace(args[0])), meta)
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
	if isQuotedStringExpr(raw) {
		return unquoteString(raw)
	}
	switch raw {
	case exprChannelBaseURL:
		return meta.BaseURL
	case exprChannelKey:
		return meta.APIKey
	case exprOAuthAccessToken:
		return meta.OAuthAccessToken
	case exprRequestModel:
		return meta.OriginModelName
	case exprRequestMapped:
		if meta.DSLModelMapped != "" {
			return meta.DSLModelMapped
		}
		return meta.OriginModelName
	default:
		return raw
	}
}

func evalTemplateString(tmpl string, meta *dslmeta.Meta) string {
	var b strings.Builder
	for i := 0; i < len(tmpl); {
		if strings.HasPrefix(tmpl[i:], `\${`) {
			b.WriteString("${")
			i += len(`\${`)
			continue
		}
		if !strings.HasPrefix(tmpl[i:], "${") {
			b.WriteByte(tmpl[i])
			i++
			continue
		}
		end := strings.IndexByte(tmpl[i+2:], '}')
		if end < 0 {
			b.WriteString(tmpl[i:])
			break
		}
		name := strings.TrimSpace(tmpl[i+2 : i+2+end])
		if expr, ok := normalizeTemplateVariable(name); ok {
			b.WriteString(evalStringExpr(expr, meta))
		}
		i += 2 + end + 1
	}
	return b.String()
}

func normalizeTemplateVariable(name string) (string, bool) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", false
	}
	if !strings.HasPrefix(n, "$") {
		n = "$" + n
	}
	if isBuiltinStringVariable(n) {
		return n, true
	}
	return "", false
}

func isBuiltinStringVariable(expr string) bool {
	switch strings.TrimSpace(expr) {
	case exprChannelBaseURL, exprChannelKey, exprOAuthAccessToken, exprRequestModel, exprRequestMapped:
		return true
	default:
		return false
	}
}

func isQuotedStringExpr(expr string) bool {
	raw := strings.TrimSpace(expr)
	if len(raw) < 2 {
		return false
	}
	return (raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')
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
