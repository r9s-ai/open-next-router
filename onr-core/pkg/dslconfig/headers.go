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

	Headers PhaseHeaders
}

type HeaderValueFilterRule struct {
	Name      string
	Patterns  []string
	Separator string
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
	Patterns  []string
	Separator string
}

func (p ProviderHeaders) Apply(meta *dslmeta.Meta, srcHdr http.Header, dstHdr http.Header) {
	if dstHdr == nil {
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
			dstHdr.Set(name, evalStringExpr(op.ValueExpr, meta))
		case "header_del":
			if name == "" {
				continue
			}
			dstHdr.Del(name)
		case "header_pass":
			if name == "" || srcHdr == nil {
				continue
			}
			values := srcHdr.Values(name)
			if len(values) == 0 {
				continue
			}
			dstHdr.Del(name)
			for _, value := range values {
				dstHdr.Add(name, value)
			}
		case "header_filter_values":
			applyHeaderValueFilterRule(dstHdr, HeaderValueFilterRule{
				Name:      name,
				Patterns:  append([]string(nil), op.Patterns...),
				Separator: op.Separator,
			})
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

func applyHeaderValueFilterRule(hdr http.Header, rule HeaderValueFilterRule) {
	name := strings.TrimSpace(rule.Name)
	if name == "" {
		return
	}
	values := hdr.Values(name)
	if len(values) == 0 {
		return
	}
	items := splitHeaderValues(values, rule.Separator)
	if len(items) == 0 {
		hdr.Del(name)
		return
	}
	kept := items[:0]
	for _, item := range items {
		if matchesAnyHeaderValuePattern(item, rule.Patterns) {
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == 0 {
		hdr.Del(name)
		return
	}
	hdr.Set(name, strings.Join(kept, headerValueJoinSeparator(rule.Separator)))
}

func splitHeaderValues(values []string, separator string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, splitHeaderValueItems(value, separator)...)
	}
	return out
}

func splitHeaderValueItems(raw, separator string) []string {
	sep := separator
	if strings.TrimSpace(sep) == "" {
		sep = ","
	}
	parts := strings.Split(raw, sep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func headerValueJoinSeparator(separator string) string {
	sep := separator
	if strings.TrimSpace(sep) == "" || sep == "," {
		return ", "
	}
	return sep + " "
}

func matchesAnyHeaderValuePattern(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if wildcardMatch(pattern, value) {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern, value string) bool {
	if pattern == "" {
		return value == ""
	}
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return value == pattern
	}
	pos := 0
	if parts[0] != "" {
		if !strings.HasPrefix(value, parts[0]) {
			return false
		}
		pos = len(parts[0])
	}
	for i := 1; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	last := parts[len(parts)-1]
	if last == "" {
		return true
	}
	if len(parts) == 2 && parts[0] == "" {
		return strings.HasSuffix(value, last)
	}
	return strings.HasSuffix(value, last) && pos <= len(value)-len(last)
}
