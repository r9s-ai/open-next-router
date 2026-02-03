package dslconfig

import (
	"fmt"
	"strings"
)

const providerKeyword = "provider"

func parseProviderConfig(path string, content string) (ProviderRouting, ProviderHeaders, ProviderRequestTransform, ProviderResponse, ProviderError, ProviderUsage, ProviderFinishReason, error) {
	s := newScanner(path, content)
	var routing ProviderRouting
	var headers ProviderHeaders
	var req ProviderRequestTransform
	var response ProviderResponse
	var perr ProviderError
	var usage ProviderUsage
	var finish ProviderFinishReason
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokIdent && tok.text == providerKeyword {
			nameTok := s.nextNonTrivia()
			if nameTok.kind != tokString {
				return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, s.errAt(nameTok, "expected provider name string literal")
			}
			lb := s.nextNonTrivia()
			if lb.kind != tokLBrace {
				return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, s.errAt(lb, "expected '{' after provider name")
			}
			r, h, rq, resp, e, u, fr, err := parseProviderBody(s)
			if err != nil {
				return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, err
			}
			routing = r
			headers = h
			req = rq
			response = resp
			perr = e
			usage = u
			finish = fr
			continue
		}
	}
	return routing, headers, req, response, perr, usage, finish, nil
}

func parseProviderBody(s *scanner) (ProviderRouting, ProviderHeaders, ProviderRequestTransform, ProviderResponse, ProviderError, ProviderUsage, ProviderFinishReason, error) {
	var routing ProviderRouting
	var headers ProviderHeaders
	var req ProviderRequestTransform
	var response ProviderResponse
	var perr ProviderError
	var usage ProviderUsage
	var finish ProviderFinishReason
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, s.errAt(tok, "unexpected EOF in provider block")
		case tokRBrace:
			return routing, headers, req, response, perr, usage, finish, nil
		case tokIdent:
			switch tok.text {
			case "defaults":
				if err := parseDefaultsBlock(s, &routing, &headers, &req, &response, &perr, &usage, &finish); err != nil {
					return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, err
				}
			case "match":
				m, mh, mreq, mr, me, mu, mfr, err := parseMatchBlock(s)
				if err != nil {
					return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, err
				}
				routing.Matches = append(routing.Matches, m)
				headers.Matches = append(headers.Matches, mh)
				req.Matches = append(req.Matches, mreq)
				response.Matches = append(response.Matches, mr)
				perr.Matches = append(perr.Matches, me)
				usage.Matches = append(usage.Matches, mu)
				finish.Matches = append(finish.Matches, mfr)
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, err
				}
			}
		default:
			// ignore
		}
	}
}

func parseDefaultsBlock(s *scanner, routing *ProviderRouting, headers *ProviderHeaders, req *ProviderRequestTransform, response *ProviderResponse, perr *ProviderError, usage *ProviderUsage, finish *ProviderFinishReason) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after defaults")
	}
	return parsePhaseBlock(s, "defaults block", phaseHandlers{
		upstreamConfig: func() error { return parseUpstreamConfigBlock(s, routing) },
		auth:           func() error { return parseAuthPhase(s, &headers.Defaults) },
		request:        func() error { return parseRequestPhaseWithTransform(s, &headers.Defaults, &req.Defaults) },
		response:       func() error { return parseResponsePhase(s, &response.Defaults) },
		errPhase:       func() error { return parseErrorPhase(s, &perr.Defaults) },
		metrics:        func() error { return parseMetricsPhase(s, &usage.Defaults, &finish.Defaults) },
	})
}

func parseUpstreamConfigBlock(s *scanner, routing *ProviderRouting) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after upstream_config")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in upstream_config block")
		case tokRBrace:
			return nil
		case tokIdent:
			if tok.text == "base_url" {
				if err := consumeEquals(s); err != nil {
					return err
				}
				expr, err := consumeExprUntilSemicolon(s)
				if err != nil {
					return err
				}
				routing.BaseURLExpr = strings.TrimSpace(expr)
				continue
			}
			if err := skipStmtOrBlock(s); err != nil {
				return err
			}
		default:
			// ignore
		}
	}
}

func parseMatchBlock(s *scanner) (RoutingMatch, MatchHeaders, MatchRequestTransform, MatchResponse, MatchError, MatchUsage, MatchFinishReason, error) {
	var m RoutingMatch
	var h MatchHeaders
	var req MatchRequestTransform
	var r MatchResponse
	var e MatchError
	var u MatchUsage
	var fr MatchFinishReason
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return RoutingMatch{}, MatchHeaders{}, MatchRequestTransform{}, MatchResponse{}, MatchError{}, MatchUsage{}, MatchFinishReason{}, s.errAt(tok, "unexpected EOF in match header")
		}
		if tok.kind == tokLBrace {
			break
		}
		if tok.kind != tokIdent {
			continue
		}
		key := tok.text
		op := s.nextNonTrivia()
		if op.kind != tokOther || (op.text != "=" && op.text != "!=") {
			continue
		}
		valTok := s.nextNonTrivia()
		switch key {
		case "api":
			if valTok.kind != tokString {
				return RoutingMatch{}, MatchHeaders{}, MatchRequestTransform{}, MatchResponse{}, MatchError{}, MatchUsage{}, MatchFinishReason{}, s.errAt(valTok, "match api expects string literal")
			}
			m.API = strings.TrimSpace(unquoteString(valTok.text))
			h.API = m.API
			req.API = m.API
			r.API = m.API
			e.API = m.API
			u.API = m.API
			fr.API = m.API
		case "stream":
			if valTok.kind != tokIdent {
				return RoutingMatch{}, MatchHeaders{}, MatchRequestTransform{}, MatchResponse{}, MatchError{}, MatchUsage{}, MatchFinishReason{}, s.errAt(valTok, "match stream expects true/false")
			}
			switch valTok.text {
			case "true":
				v := true
				m.Stream = &v
				h.Stream = &v
				req.Stream = &v
				r.Stream = &v
				e.Stream = &v
				u.Stream = &v
				fr.Stream = &v
			case "false":
				v := false
				m.Stream = &v
				h.Stream = &v
				req.Stream = &v
				r.Stream = &v
				e.Stream = &v
				u.Stream = &v
				fr.Stream = &v
			default:
				return RoutingMatch{}, MatchHeaders{}, MatchRequestTransform{}, MatchResponse{}, MatchError{}, MatchUsage{}, MatchFinishReason{}, s.errAt(valTok, "match stream expects true/false")
			}
		default:
			// ignore other keys in v0.1 parser
		}
	}

	m.QueryPairs = map[string]string{}
	if err := parseMatchBody(s, &m, &h, &req, &r, &e, &u, &fr); err != nil {
		return RoutingMatch{}, MatchHeaders{}, MatchRequestTransform{}, MatchResponse{}, MatchError{}, MatchUsage{}, MatchFinishReason{}, err
	}
	return m, h, req, r, e, u, fr, nil
}

func parseMatchBody(s *scanner, m *RoutingMatch, h *MatchHeaders, req *MatchRequestTransform, r *MatchResponse, e *MatchError, u *MatchUsage, fr *MatchFinishReason) error {
	h.Upstream.QueryPairs = map[string]string{}
	return parsePhaseBlock(s, "match block", phaseHandlers{
		upstream:  func() error { return parseUpstreamPhase(s, m) },
		auth:      func() error { return parseAuthPhase(s, &h.Headers) },
		request:   func() error { return parseRequestPhaseWithTransform(s, &h.Headers, &req.Transform) },
		response:  func() error { return parseResponsePhase(s, &r.Response) },
		errPhase:  func() error { return parseErrorPhase(s, &e.Response) },
		metrics:   func() error { return parseMetricsPhase(s, &u.Extract, &fr.Extract) },
		onUnknown: func() error { return skipStmtOrBlock(s) },
	})
}

type phaseHandlers struct {
	upstreamConfig func() error
	upstream       func() error
	auth           func() error
	request        func() error
	response       func() error
	errPhase       func() error
	metrics        func() error
	onUnknown      func() error
}

func parsePhaseBlock(s *scanner, blockName string, h phaseHandlers) error {
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in "+blockName)
		case tokRBrace:
			return nil
		case tokIdent:
			switch tok.text {
			case "upstream_config":
				if h.upstreamConfig != nil {
					if err := h.upstreamConfig(); err != nil {
						return err
					}
					continue
				}
			case "upstream":
				if h.upstream != nil {
					if err := h.upstream(); err != nil {
						return err
					}
					continue
				}
			case "auth":
				if h.auth != nil {
					if err := h.auth(); err != nil {
						return err
					}
					continue
				}
			case "request":
				if h.request != nil {
					if err := h.request(); err != nil {
						return err
					}
					continue
				}
			case "response":
				if h.response != nil {
					if err := h.response(); err != nil {
						return err
					}
					continue
				}
			case "error":
				if h.errPhase != nil {
					if err := h.errPhase(); err != nil {
						return err
					}
					continue
				}
			case "metrics":
				if h.metrics != nil {
					if err := h.metrics(); err != nil {
						return err
					}
					continue
				}
			}
			if h.onUnknown != nil {
				if err := h.onUnknown(); err != nil {
					return err
				}
				continue
			}
			if err := skipStmtOrBlock(s); err != nil {
				return err
			}
		default:
			// ignore
		}
	}
}

func parseMetricsPhase(s *scanner, usage *UsageExtractConfig, finish *FinishReasonExtractConfig) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after metrics")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in metrics phase")
		case tokRBrace:
			return nil
		case tokIdent:
			switch tok.text {
			case "usage_extract":
				if err := parseUsageExtractStmt(s, usage); err != nil {
					return err
				}
			case "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "total_tokens":
				if err := parseUsageExtractAssignStmt(s, usage, tok.text); err != nil {
					return err
				}
			case "input_tokens_path", "output_tokens_path", "cache_read_tokens_path", "cache_write_tokens_path":
				// allow statement-style overrides inside metrics block (mainly for mode=custom).
				if err := parseUsageExtractFieldStmt(s, usage, tok.text); err != nil {
					return err
				}
			case "finish_reason_extract":
				if err := parseFinishReasonExtractStmt(s, finish); err != nil {
					return err
				}
			case "finish_reason_path":
				if err := parseFinishReasonPathStmt(s, finish); err != nil {
					return err
				}
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return err
				}
			}
		default:
			// ignore
		}
	}
}

func parseFinishReasonExtractStmt(s *scanner, cfg *FinishReasonExtractConfig) error {
	// finish_reason_extract <mode>;
	modeTok := s.nextNonTrivia()
	if modeTok.kind == tokLBrace {
		return s.errAt(modeTok, "finish_reason_extract does not use braces; use: finish_reason_extract <mode>;")
	}
	if strings.TrimSpace(cfg.Mode) != "" {
		return s.errAt(modeTok, "finish_reason_extract may appear only once in a metrics block")
	}
	switch modeTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(modeTok, "finish_reason_extract expects mode")
	}
	mode := modeTok.text
	if modeTok.kind == tokString {
		mode = unquoteString(modeTok.text)
	}
	cfg.Mode = strings.TrimSpace(mode)
	return consumeSemicolon(s, "finish_reason_extract")
}

func parseFinishReasonPathStmt(s *scanner, cfg *FinishReasonExtractConfig) error {
	// finish_reason_path <jsonpath>;
	peek := s.nextNonTrivia()
	if peek.kind == tokOther && peek.text == "=" {
		return s.errAt(peek, "finish_reason_path does not use '='; use: finish_reason_path <jsonpath>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, peek)
	if err != nil {
		return err
	}
	val := strings.TrimSpace(expr)
	if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
		val = strings.TrimSpace(unquoteString(val))
	}
	cfg.FinishReasonPath = val
	return nil
}

func parseUsageExtractStmt(s *scanner, cfg *UsageExtractConfig) error {
	// usage_extract <mode>;
	modeTok := s.nextNonTrivia()
	if modeTok.kind == tokLBrace {
		return s.errAt(modeTok, "usage_extract does not use braces; use: usage_extract <mode>;")
	}
	if strings.TrimSpace(cfg.Mode) != "" {
		return s.errAt(modeTok, "usage_extract may appear only once in a metrics block")
	}
	switch modeTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(modeTok, "usage_extract expects mode")
	}
	mode := modeTok.text
	if modeTok.kind == tokString {
		mode = unquoteString(modeTok.text)
	}
	cfg.Mode = strings.TrimSpace(mode)
	return consumeSemicolon(s, "usage_extract")
}

func parseUsageExtractFieldStmt(s *scanner, cfg *UsageExtractConfig, key string) error {
	// <key> <string>;
	peek := s.nextNonTrivia()
	if peek.kind == tokOther && peek.text == "=" {
		return s.errAt(peek, key+" does not use '='; use: "+key+" <jsonpath>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, peek)
	if err != nil {
		return err
	}
	val := strings.TrimSpace(expr)
	if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
		val = strings.TrimSpace(unquoteString(val))
	}
	switch key {
	case "input_tokens_path":
		cfg.InputTokensPath = val
	case "output_tokens_path":
		cfg.OutputTokensPath = val
	case "cache_read_tokens_path":
		cfg.CacheReadTokensPath = val
	case "cache_write_tokens_path":
		cfg.CacheWriteTokensPath = val
	}
	return nil
}

func parseUsageExtractAssignStmt(s *scanner, cfg *UsageExtractConfig, key string) error {
	// <key> = <expr>;
	if err := consumeEquals(s); err != nil {
		return err
	}
	exprStr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	expr, err := ParseUsageExpr(exprStr)
	if err != nil {
		return fmt.Errorf("invalid %s expr %q: %w", key, exprStr, err)
	}
	switch key {
	case "input_tokens":
		cfg.InputTokensExpr = expr
	case "output_tokens":
		cfg.OutputTokensExpr = expr
	case "cache_read_tokens":
		cfg.CacheReadTokensExpr = expr
	case "cache_write_tokens":
		cfg.CacheWriteTokensExpr = expr
	case "total_tokens":
		cfg.TotalTokensExpr = expr
	}
	return nil
}

func parseUpstreamPhase(s *scanner, m *RoutingMatch) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after upstream")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in upstream phase")
		case tokRBrace:
			return nil
		case tokIdent:
			switch tok.text {
			case "set_path":
				if err := parseDirectiveSetPathStmt(s, m); err != nil {
					return err
				}
			case "set_query":
				if err := parseDirectiveSetQueryStmt(s, m); err != nil {
					return err
				}
			case "del_query":
				if err := parseDirectiveDelQueryStmt(s, m); err != nil {
					return err
				}
			case "query_set":
				return s.errAt(tok, "query_set has been removed; use: set_query <name> <expr>;")
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return err
				}
			}
		default:
			// ignore
		}
	}
}

func parseDirectiveSetPathStmt(s *scanner, m *RoutingMatch) error {
	// set_path <expr>;
	first := s.nextNonTrivia()
	if first.kind == tokLBrace {
		return s.errAt(first, "set_path does not use braces; use: set_path <expr>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, first)
	if err != nil {
		return err
	}
	m.SetPath = strings.TrimSpace(expr)
	return nil
}

func parseDirectiveSetQueryStmt(s *scanner, m *RoutingMatch) error {
	// set_query <name> <expr>;
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(nameTok, "set_query expects name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	valueExpr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	if m.QueryPairs == nil {
		m.QueryPairs = map[string]string{}
	}
	name = strings.TrimSpace(name)
	if name != "" {
		m.QueryPairs[name] = strings.TrimSpace(valueExpr)
	}
	return nil
}

func parseDirectiveDelQueryStmt(s *scanner, m *RoutingMatch) error {
	// del_query <name>;
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(nameTok, "del_query expects name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	if err := consumeSemicolon(s, "del_query"); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	m.QueryDels = append(m.QueryDels, name)
	return nil
}
