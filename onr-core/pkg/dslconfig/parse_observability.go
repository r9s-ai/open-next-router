package dslconfig

import (
	"fmt"
	"strings"
	"unicode"
)

func parseObservabilityBlock(s *scanner) (ProviderObservability, error) {
	var out ProviderObservability
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return out, s.errAt(lb, "expected '{' after observability")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return ProviderObservability{}, s.errAt(tok, "unexpected EOF in observability block")
		case tokRBrace:
			return out, nil
		case tokIdent:
			if tok.text != "upstream_request_id" {
				return ProviderObservability{}, s.errAt(tok, fmt.Sprintf("unknown observability directive %q", tok.text))
			}
			headers, err := parseUpstreamRequestIDHeaders(s)
			if err != nil {
				return ProviderObservability{}, err
			}
			out.UpstreamRequestID = &UpstreamRequestIDRule{Headers: headers}
		default:
			return ProviderObservability{}, s.errAt(tok, "unexpected token in observability block")
		}
	}
}

func parseUpstreamRequestIDHeaders(s *scanner) ([]string, error) {
	const maxHeaders = 16
	headers := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokString:
			name := strings.TrimSpace(unquoteString(tok.text))
			if name == "" || !validHeaderFieldName(name) {
				return nil, s.errAt(tok, fmt.Sprintf("invalid upstream request ID header name %q", name))
			}
			key := strings.ToLower(name)
			if _, ok := seen[key]; ok {
				return nil, s.errAt(tok, fmt.Sprintf("duplicate upstream request ID header %q", name))
			}
			seen[key] = struct{}{}
			headers = append(headers, name)
			if len(headers) > maxHeaders {
				return nil, s.errAt(tok, fmt.Sprintf("too many upstream request ID headers (maximum %d)", maxHeaders))
			}
		case tokSemicolon:
			return headers, nil
		case tokEOF:
			return nil, s.errAt(tok, "expected ';' after upstream_request_id")
		case tokRBrace:
			return nil, s.errAt(tok, "expected ';' after upstream_request_id")
		default:
			return nil, s.errAt(tok, "upstream_request_id expects string header names followed by ';'")
		}
	}
}

func validHeaderFieldName(name string) bool {
	for _, r := range name {
		if r <= 0x20 || r >= 0x7f || !unicode.IsPrint(r) || strings.ContainsRune("()<>@,;:\\\"/[]?={} \t", r) {
			return false
		}
	}
	return true
}
