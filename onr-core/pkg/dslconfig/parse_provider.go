package dslconfig

import (
	"fmt"
	"strings"
)

const providerKeyword = "provider"

func parseProviderConfig(path string, content string) (ProviderRouting, ProviderHeaders, ProviderRequestTransform, ProviderResponse, ProviderError, ProviderUsage, ProviderFinishReason, ProviderBalance, ProviderModels, error) {
	s := newScanner(path, content)
	var routing ProviderRouting
	var headers ProviderHeaders
	var req ProviderRequestTransform
	var response ProviderResponse
	var perr ProviderError
	var usage ProviderUsage
	var finish ProviderFinishReason
	var balance ProviderBalance
	var models ProviderModels
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokIdent && tok.text == providerKeyword {
			nameTok := s.nextNonTrivia()
			if nameTok.kind != tokString {
				return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, s.errAt(nameTok, "expected provider name string literal")
			}
			lb := s.nextNonTrivia()
			if lb.kind != tokLBrace {
				return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, s.errAt(lb, "expected '{' after provider name")
			}
			r, h, rq, resp, e, u, fr, bq, mq, err := parseProviderBody(s)
			if err != nil {
				return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, err
			}
			routing = r
			headers = h
			req = rq
			response = resp
			perr = e
			usage = u
			finish = fr
			balance = bq
			models = mq
			continue
		}
	}
	return routing, headers, req, response, perr, usage, finish, balance, models, nil
}

func parseProviderBody(s *scanner) (ProviderRouting, ProviderHeaders, ProviderRequestTransform, ProviderResponse, ProviderError, ProviderUsage, ProviderFinishReason, ProviderBalance, ProviderModels, error) {
	var routing ProviderRouting
	var headers ProviderHeaders
	var req ProviderRequestTransform
	var response ProviderResponse
	var perr ProviderError
	var usage ProviderUsage
	var finish ProviderFinishReason
	var balance ProviderBalance
	var models ProviderModels
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, s.errAt(tok, "unexpected EOF in provider block")
		case tokRBrace:
			return routing, headers, req, response, perr, usage, finish, balance, models, nil
		case tokIdent:
			switch tok.text {
			case "defaults":
				if err := parseDefaultsBlock(s, &routing, &headers, &req, &response, &perr, &usage, &finish, &balance, &models); err != nil {
					return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, err
				}
			case "match":
				m, mh, mreq, mr, me, mu, mfr, err := parseMatchBlock(s)
				if err != nil {
					return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, err
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
					return ProviderRouting{}, ProviderHeaders{}, ProviderRequestTransform{}, ProviderResponse{}, ProviderError{}, ProviderUsage{}, ProviderFinishReason{}, ProviderBalance{}, ProviderModels{}, err
				}
			}
		default:
			// ignore
		}
	}
}

func parseDefaultsBlock(s *scanner, routing *ProviderRouting, headers *ProviderHeaders, req *ProviderRequestTransform, response *ProviderResponse, perr *ProviderError, usage *ProviderUsage, finish *ProviderFinishReason, balance *ProviderBalance, models *ProviderModels) error {
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
		balance:        func() error { return parseBalancePhase(s, &balance.Defaults) },
		models:         func() error { return parseModelsPhase(s, &models.Defaults) },
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
	balance        func() error
	models         func() error
	onUnknown      func() error
}

func parsePhaseBlock(s *scanner, blockName string, h phaseHandlers) error {
	dispatch := buildPhaseDispatch(h)
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in "+blockName)
		case tokRBrace:
			return nil
		case tokIdent:
			if fn, ok := dispatch[tok.text]; ok {
				if err := fn(); err != nil {
					return err
				}
				continue
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

func buildPhaseDispatch(h phaseHandlers) map[string]func() error {
	dispatch := make(map[string]func() error, 8)
	if h.upstreamConfig != nil {
		dispatch["upstream_config"] = h.upstreamConfig
	}
	if h.upstream != nil {
		dispatch["upstream"] = h.upstream
	}
	if h.auth != nil {
		dispatch["auth"] = h.auth
	}
	if h.request != nil {
		dispatch["request"] = h.request
	}
	if h.response != nil {
		dispatch["response"] = h.response
	}
	if h.errPhase != nil {
		dispatch["error"] = h.errPhase
	}
	if h.metrics != nil {
		dispatch["metrics"] = h.metrics
	}
	if h.balance != nil {
		dispatch["balance"] = h.balance
	}
	if h.models != nil {
		dispatch["models"] = h.models
	}
	return dispatch
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
			case "usage_fact":
				if err := parseUsageFactStmt(s, usage); err != nil {
					return err
				}
			case "input_tokens_expr", "output_tokens_expr", "cache_read_tokens_expr", "cache_write_tokens_expr", "total_tokens_expr":
				if err := parseUsageExtractAssignStmt(s, usage, tok.text); err != nil {
					return err
				}
			case "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "total_tokens":
				return s.errAt(tok, fmt.Sprintf("%s has been removed; use %s", tok.text, legacyUsageExprReplacement(tok.text)))
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
	// finish_reason_path <jsonpath> [fallback=true|false] [event="<name>"] [event_optional=true|false];
	peek := s.nextNonTrivia()
	if peek.kind == tokOther && peek.text == "=" {
		return s.errAt(peek, "finish_reason_path does not use '='; use: finish_reason_path <jsonpath>;")
	}
	path, err := parseUsageFactScalar(peek, "finish_reason_path")
	if err != nil {
		return err
	}
	fallback := false
	event := ""
	eventOptional := false
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokSemicolon:
			cfg.addFinishReasonPathRule(strings.TrimSpace(path), fallback, event, eventOptional)
			return nil
		case tokIdent:
			key := strings.ToLower(strings.TrimSpace(tok.text))
			eq := s.nextNonTrivia()
			if eq.kind != tokOther || eq.text != "=" {
				return s.errAt(eq, "expected '=' after finish_reason_path option "+key)
			}
			valTok := s.nextNonTrivia()
			switch key {
			case "fallback":
				val, err := parseUsageFactBoolValue(s, valTok)
				if err != nil {
					return err
				}
				fallback = val
			case "event":
				val, err := parseUsageFactStringValue(s, valTok, "event")
				if err != nil {
					return err
				}
				event = val
			case "event_optional":
				val, err := parseUsageFactBoolValue(s, valTok)
				if err != nil {
					return s.errAt(valTok, "event_optional expects true or false")
				}
				eventOptional = val
			default:
				return s.errAt(tok, "unsupported finish_reason_path option "+key)
			}
		default:
			return s.errAt(tok, "expected finish_reason_path option or ';'")
		}
	}
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
	case "input_tokens_expr":
		cfg.InputTokensExpr = expr
	case "output_tokens_expr":
		cfg.OutputTokensExpr = expr
	case "cache_read_tokens_expr":
		cfg.CacheReadTokensExpr = expr
	case "cache_write_tokens_expr":
		cfg.CacheWriteTokensExpr = expr
	case "total_tokens_expr":
		cfg.TotalTokensExpr = expr
	}
	return nil
}

func legacyUsageExprReplacement(key string) string {
	switch key {
	case "input_tokens":
		return "input_tokens_expr = <expr>;"
	case "output_tokens":
		return "output_tokens_expr = <expr>;"
	case "cache_read_tokens":
		return "cache_read_tokens_expr = <expr>;"
	case "cache_write_tokens":
		return "cache_write_tokens_expr = <expr>;"
	case "total_tokens":
		return "total_tokens_expr = <expr>;"
	default:
		return "<directive>_expr = <expr>;"
	}
}

func parseUsageFactStmt(s *scanner, cfg *UsageExtractConfig) error {
	// usage_fact <dimension> <unit> <key>=<value>...;
	dimTok := s.nextNonTrivia()
	if dimTok.kind == tokLBrace {
		return s.errAt(dimTok, "usage_fact does not use braces")
	}
	dimension, err := parseUsageFactScalar(dimTok, "usage_fact dimension")
	if err != nil {
		return err
	}
	unitTok := s.nextNonTrivia()
	unit, err := parseUsageFactScalar(unitTok, "usage_fact unit")
	if err != nil {
		return err
	}
	fact := usageFactConfig{
		Dimension: dimension,
		Unit:      unit,
		Attrs:     map[string]string{},
	}
	if strings.TrimSpace(fact.Dimension) == "" || strings.TrimSpace(fact.Unit) == "" {
		return s.errAt(unitTok, "usage_fact requires both dimension and unit")
	}
	primitiveSet := false
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in usage_fact")
		case tokSemicolon:
			if !primitiveSet {
				return s.errAt(tok, "usage_fact requires one of path, count_path, sum_path or expr")
			}
			if !usageFactKeyAllowed(fact.Dimension, fact.Unit) {
				return s.errAt(tok, "usage_fact dimension/unit not allowed: "+usageFactKeyString(normalizeUsageFactKey(fact.Dimension, fact.Unit)))
			}
			cfg.facts = append(cfg.facts, fact)
			cfg.factGroups = nil
			cfg.explicitFactKeys = nil
			*cfg = prepareUsageExtractConfig(*cfg)
			return nil
		case tokIdent:
			key := tok.text
			if err := consumeEquals(s); err != nil {
				return err
			}
			valTok := s.nextNonTrivia()
			if handled, err := parseUsageFactPrimitiveOption(s, tok, key, valTok, &fact, &primitiveSet); handled {
				if err != nil {
					return err
				}
				continue
			}
			if err := parseUsageFactMetaOption(s, tok, key, valTok, &fact); err != nil {
				return err
			}
		default:
			return s.errAt(tok, "expected usage_fact option or ';'")
		}
	}
}

func parseUsageFactPrimitiveOption(s *scanner, keyTok token, key string, valTok token, fact *usageFactConfig, primitiveSet *bool) (bool, error) {
	switch key {
	case "path", "count_path", "sum_path", "expr":
	default:
		return false, nil
	}
	if *primitiveSet {
		return true, s.errAt(keyTok, "usage_fact allows only one of path, count_path, sum_path or expr")
	}
	val, err := parseUsageFactStringValue(s, valTok, key)
	if err != nil {
		return true, err
	}
	switch key {
	case "path":
		fact.Path = val
	case "count_path":
		fact.CountPath = val
	case "sum_path":
		fact.SumPath = val
	case "expr":
		expr, err := ParseUsageExpr(val)
		if err != nil {
			return true, fmt.Errorf("invalid usage_fact expr %q: %w", val, err)
		}
		fact.Expr = expr
	}
	*primitiveSet = true
	return true, nil
}

func parseUsageFactMetaOption(s *scanner, keyTok token, key string, valTok token, fact *usageFactConfig) error {
	switch {
	case key == "type":
		val, err := parseUsageFactStringValue(s, valTok, "type")
		if err != nil {
			return err
		}
		fact.Type = val
		return nil
	case key == "status":
		val, err := parseUsageFactStringValue(s, valTok, "status")
		if err != nil {
			return err
		}
		fact.Status = val
		return nil
	case key == "event":
		val, err := parseUsageFactStringValue(s, valTok, "event")
		if err != nil {
			return err
		}
		fact.Event = val
		return nil
	case key == "event_optional":
		val, err := parseUsageFactBoolValue(s, valTok)
		if err != nil {
			return s.errAt(valTok, "event_optional expects true or false")
		}
		fact.EventOptional = val
		return nil
	case key == "source":
		val, err := parseUsageFactStringValue(s, valTok, "source")
		if err != nil {
			return err
		}
		fact.Source = val
		return nil
	case key == "fallback":
		val, err := parseUsageFactBoolValue(s, valTok)
		if err != nil {
			return err
		}
		fact.Fallback = val
		return nil
	case strings.HasPrefix(key, "attr."):
		val, err := parseUsageFactStringValue(s, valTok, key)
		if err != nil {
			return err
		}
		attrKey := strings.TrimPrefix(key, "attr.")
		if strings.TrimSpace(attrKey) == "" {
			return s.errAt(keyTok, "usage_fact attr name is empty")
		}
		fact.Attrs[attrKey] = val
		return nil
	default:
		return s.errAt(keyTok, "unsupported usage_fact option "+key)
	}
}

func parseUsageFactScalar(tok token, what string) (string, error) {
	switch tok.kind {
	case tokIdent, tokString:
		val := tok.text
		if tok.kind == tokString {
			val = unquoteString(tok.text)
		}
		val = strings.TrimSpace(val)
		if val == "" {
			return "", fmt.Errorf("%s is empty", what)
		}
		return val, nil
	default:
		return "", fmt.Errorf("%s expects identifier or string", what)
	}
}

func parseUsageFactStringValue(s *scanner, tok token, key string) (string, error) {
	switch tok.kind {
	case tokString:
		return strings.TrimSpace(unquoteString(tok.text)), nil
	case tokIdent:
		return strings.TrimSpace(tok.text), nil
	default:
		return "", s.errAt(tok, key+" expects string literal or identifier")
	}
}

func parseUsageFactBoolValue(s *scanner, tok token) (bool, error) {
	switch tok.kind {
	case tokIdent:
		switch strings.ToLower(strings.TrimSpace(tok.text)) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	case tokString:
		switch strings.ToLower(strings.TrimSpace(unquoteString(tok.text))) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, s.errAt(tok, "fallback expects true or false")
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
