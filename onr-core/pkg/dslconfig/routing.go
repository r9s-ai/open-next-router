package dslconfig

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type ProviderRouting struct {
	BaseURLExpr string
	Transport   string
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
			baseURL, err := evalRoutingStringExpr(v, meta)
			if err != nil {
				return fmt.Errorf("evaluate base_url: %w", err)
			}
			meta.BaseURL = baseURL
		}
	}
	if transport := strings.ToLower(strings.TrimSpace(p.Transport)); transport != "" {
		meta.UpstreamTransport = transport
	}
	u, err := url.Parse(meta.RequestURLPath)
	if err != nil {
		return fmt.Errorf("parse request url path %q: %w", meta.RequestURLPath, err)
	}
	if match.SetPath != "" {
		path, err := evalRoutingStringExpr(match.SetPath, meta)
		if err != nil {
			return fmt.Errorf("evaluate set_path: %w", err)
		}
		u.Path = path
	}
	q := u.Query()
	for _, k := range match.QueryDels {
		if strings.TrimSpace(k) == "" {
			continue
		}
		q.Del(k)
	}
	for k, v := range match.QueryPairs {
		value, err := evalRoutingStringExpr(v, meta)
		if err != nil {
			return fmt.Errorf("evaluate set_query %q: %w", k, err)
		}
		q.Set(k, value)
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

// ReferencesVariable reports whether routing expressions contain a DSL variable such as "channel.location".
func (p *ProviderRouting) ReferencesVariable(variable string) bool {
	if referencesVariable(p.BaseURLExpr, variable) {
		return true
	}
	for _, match := range p.Matches {
		if referencesVariable(match.SetPath, variable) {
			return true
		}
		for _, value := range match.QueryPairs {
			if referencesVariable(value, variable) {
				return true
			}
		}
	}
	return false
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

func referencesVariable(expr string, variable string) bool {
	variable = strings.TrimSpace(strings.TrimPrefix(variable, "$"))
	if variable == "" {
		return false
	}
	expr = strings.TrimSpace(expr)
	return strings.Contains(expr, variable) || strings.Contains(expr, "$"+variable)
}

func evalStringExpr(expr string, meta *dslmeta.Meta) string {
	return EvalStringExpr(expr, meta)
}

// EvalStringExpr evaluates a DSL string expression against request/channel metadata.
func EvalStringExpr(expr string, meta *dslmeta.Meta) string {
	v, _ := evalStringExprValue(expr, meta, false)
	return v
}

func evalRoutingStringExpr(expr string, meta *dslmeta.Meta) (string, error) {
	return evalStringExprValue(expr, meta, true)
}

func evalStringExprValue(expr string, meta *dslmeta.Meta, requireNonEmptyVariables bool) (string, error) {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "template(") && strings.HasSuffix(raw, ")") {
		args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(raw, "template("), ")"))
		if len(args) != 1 || !isQuotedStringExpr(args[0]) {
			return raw, nil
		}
		return evalTemplateString(unquoteString(strings.TrimSpace(args[0])), meta, requireNonEmptyVariables)
	}
	// Minimal concat support used by auth_bearer implementation.
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")")
		parts := splitTopLevelArgs(inner)
		var b strings.Builder
		for i, p := range parts {
			part, err := evalStringExprValue(p, meta, requireNonEmptyVariables)
			if err != nil {
				return "", fmt.Errorf("concat argument %d: %w", i, err)
			}
			b.WriteString(part)
		}
		return b.String(), nil
	}
	if isQuotedStringExpr(raw) {
		return unquoteString(raw), nil
	}
	if isBuiltinStringVariable(raw) {
		value := evalBuiltinStringVariable(raw, meta)
		if requireNonEmptyVariables && strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("template variable %q is empty", strings.TrimPrefix(strings.TrimSpace(raw), "$"))
		}
		return value, nil
	}
	return raw, nil
}

func evalTemplateString(tmpl string, meta *dslmeta.Meta, requireNonEmptyVariables bool) (string, error) {
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
			value := evalBuiltinStringVariable(expr, meta)
			if requireNonEmptyVariables && strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("template variable %q is empty", strings.TrimPrefix(strings.TrimSpace(expr), "$"))
			}
			b.WriteString(value)
		} else if requireNonEmptyVariables {
			return "", fmt.Errorf("unsupported template variable %q", name)
		}
		i += 2 + end + 1
	}
	return b.String(), nil
}

func evalBuiltinStringVariable(expr string, meta *dslmeta.Meta) string {
	if meta == nil {
		return ""
	}
	switch strings.TrimSpace(expr) {
	case exprChannelBaseURL:
		return meta.BaseURL
	case exprChannelKey:
		return meta.APIKey
	case exprChannelLocation:
		return meta.ChannelLocation
	case exprCredentialProjID:
		return meta.CredentialProjectID
	case exprOAuthAccessToken:
		return meta.OAuthAccessToken
	case exprRequestModel:
		return meta.OriginModelName
	case exprRequestMapped:
		if meta.DSLModelMapped != "" {
			return meta.DSLModelMapped
		}
		return meta.OriginModelName
	case exprTaskID:
		return meta.Task.ID
	case exprTaskUpstreamID:
		return meta.Task.UpstreamID
	default:
		return ""
	}
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
	case exprChannelBaseURL, exprChannelKey, exprChannelLocation, exprCredentialProjID, exprOAuthAccessToken, exprRequestModel, exprRequestMapped, exprTaskID, exprTaskUpstreamID:
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
	var quote byte
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
		if quote != 0 {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
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
