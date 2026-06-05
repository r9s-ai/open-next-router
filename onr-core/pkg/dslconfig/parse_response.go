package dslconfig

import (
	"strconv"
	"strings"
)

func parseResponsePhase(s *scanner, resp *ResponseDirective) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after response")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in response phase")
		case tokRBrace:
			return nil
		case tokIdent:
			switch tok.text {
			case "resp_passthrough":
				if err := parseRespPassthrough(s, resp); err != nil {
					return err
				}
			case "resp_map":
				if err := parseRespMap(s, resp); err != nil {
					return err
				}
			case "sse_parse":
				if err := parseSSEParse(s, resp); err != nil {
					return err
				}
			case "sse_collect":
				if err := parseSSECollect(s, resp); err != nil {
					return err
				}
			case jsonOpSet:
				if err := parseRespJSONSetStmt(s, resp, jsonOpSet); err != nil {
					return err
				}
			case jsonOpReplace:
				if err := parseRespJSONSetStmt(s, resp, jsonOpReplace); err != nil {
					return err
				}
			case jsonOpSetIfAbsent:
				if err := parseRespJSONSetStmt(s, resp, jsonOpSetIfAbsent); err != nil {
					return err
				}
			case jsonOpDel:
				if err := parseRespJSONDelStmt(s, resp); err != nil {
					return err
				}
			case jsonOpRename:
				if err := parseRespJSONRenameStmt(s, resp); err != nil {
					return err
				}
			case "sse_json_del_if":
				if err := parseSSEJSONDelIfStmt(s, resp); err != nil {
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

func parseRespPassthrough(s *scanner, resp *ResponseDirective) error {
	// resp_passthrough;
	if err := consumeSemicolon(s, "resp_passthrough"); err != nil {
		return err
	}
	resp.Op = "resp_passthrough"
	resp.Mode = ""
	return nil
}

func parseRespMap(s *scanner, resp *ResponseDirective) error {
	mode, err := parseModeArgStmt(s, "resp_map")
	if err != nil {
		return err
	}
	resp.Op = "resp_map"
	resp.Mode = mode
	return nil
}

func parseSSEParse(s *scanner, resp *ResponseDirective) error {
	mode, err := parseModeArgStmt(s, "sse_parse")
	if err != nil {
		return err
	}
	resp.Op = "sse_parse"
	resp.Mode = mode
	return nil
}

func parseSSECollect(s *scanner, resp *ResponseDirective) error {
	mode, err := parseModeArgStmt(s, "sse_collect")
	if err != nil {
		return err
	}
	resp.SSECollectMode = mode
	return nil
}

func parseRespJSONSetStmt(s *scanner, resp *ResponseDirective, opName string) error {
	// json_set/json_replace/json_set_if_absent <jsonpath> <expr> [event="..."] [max_count=n];
	pathTok := s.nextNonTrivia()
	switch pathTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(pathTok, opName+" expects json path")
	}
	path := pathTok.text
	if pathTok.kind == tokString {
		path = unquoteString(pathTok.text)
	}
	valueExpr, opts, err := consumeRespJSONExprAndOptions(s, opName)
	if err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:        opName,
		Path:      strings.TrimSpace(path),
		ValueExpr: strings.TrimSpace(valueExpr),
		Event:     opts.Event,
		MaxCount:  opts.MaxCount,
	})
	return nil
}

func parseRespJSONDelStmt(s *scanner, resp *ResponseDirective) error {
	// json_del <jsonpath>;
	pathTok := s.nextNonTrivia()
	switch pathTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(pathTok, "json_del expects json path")
	}
	path := pathTok.text
	if pathTok.kind == tokString {
		path = unquoteString(pathTok.text)
	}
	opts, err := consumeRespJSONOptionsOnly(s, "json_del")
	if err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:       jsonOpDel,
		Path:     strings.TrimSpace(path),
		Event:    opts.Event,
		MaxCount: opts.MaxCount,
	})
	return nil
}

func parseRespJSONRenameStmt(s *scanner, resp *ResponseDirective) error {
	// json_rename <from-jsonpath> <to-jsonpath>;
	fromTok := s.nextNonTrivia()
	switch fromTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(fromTok, "json_rename expects from path")
	}
	from := fromTok.text
	if fromTok.kind == tokString {
		from = unquoteString(fromTok.text)
	}
	toTok := s.nextNonTrivia()
	switch toTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(toTok, "json_rename expects to path")
	}
	to := toTok.text
	if toTok.kind == tokString {
		to = unquoteString(toTok.text)
	}
	opts, err := consumeRespJSONOptionsOnly(s, "json_rename")
	if err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:       jsonOpRename,
		FromPath: strings.TrimSpace(from),
		ToPath:   strings.TrimSpace(to),
		Event:    opts.Event,
		MaxCount: opts.MaxCount,
	})
	return nil
}

type respJSONOptions struct {
	Event    string
	MaxCount int
}

func consumeRespJSONExprAndOptions(s *scanner, directive string) (string, respJSONOptions, error) {
	tokens, err := consumeTokensUntilSemicolon(s, directive)
	if err != nil {
		return "", respJSONOptions{}, err
	}
	optStart := firstRespJSONOptionIndex(tokens)
	if optStart < 0 {
		expr := strings.TrimSpace(joinTokenText(tokens))
		if expr == "" {
			return "", respJSONOptions{}, s.errAt(token{pos: s.lastPos}, directive+" expects value expression")
		}
		return expr, respJSONOptions{}, nil
	}
	expr := strings.TrimSpace(joinTokenText(tokens[:optStart]))
	if expr == "" {
		return "", respJSONOptions{}, s.errAt(tokens[optStart], directive+" expects value expression")
	}
	opts, err := parseRespJSONOptions(s, tokens[optStart:], directive)
	return expr, opts, err
}

func consumeRespJSONOptionsOnly(s *scanner, directive string) (respJSONOptions, error) {
	tokens, err := consumeTokensUntilSemicolon(s, directive)
	if err != nil {
		return respJSONOptions{}, err
	}
	return parseRespJSONOptions(s, tokens, directive)
}

func consumeTokensUntilSemicolon(s *scanner, directive string) ([]token, error) {
	var tokens []token
	for {
		tok := s.next()
		switch tok.kind {
		case tokEOF:
			return nil, s.errAt(tok, "unexpected EOF in "+directive)
		case tokSemicolon:
			return tokens, nil
		default:
			tokens = append(tokens, tok)
		}
	}
}

func firstRespJSONOptionIndex(tokens []token) int {
	for i := range tokens {
		if isRespJSONOptionStart(tokens, i) {
			return i
		}
	}
	return -1
}

func isRespJSONOptionStart(tokens []token, idx int) bool {
	if idx < 0 || idx >= len(tokens) || tokens[idx].kind != tokIdent {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(tokens[idx].text))
	if key != "event" && key != "max_count" {
		return false
	}
	next := nextNonTriviaIndex(tokens, idx+1)
	return next >= 0 && tokens[next].kind == tokOther && tokens[next].text == "="
}

func parseRespJSONOptions(s *scanner, tokens []token, directive string) (respJSONOptions, error) {
	opts := respJSONOptions{}
	for i := nextNonTriviaIndex(tokens, 0); i >= 0 && i < len(tokens); {
		if tokens[i].kind != tokIdent {
			return opts, s.errAt(tokens[i], directive+" expects response json option")
		}
		key := strings.ToLower(strings.TrimSpace(tokens[i].text))
		if key != "event" && key != "max_count" {
			return opts, s.errAt(tokens[i], "unsupported "+directive+" option "+key)
		}
		eq := nextNonTriviaIndex(tokens, i+1)
		if eq < 0 || tokens[eq].kind != tokOther || tokens[eq].text != "=" {
			return opts, s.errAt(tokens[i], "expected '=' after "+key)
		}
		valStart := nextNonTriviaIndex(tokens, eq+1)
		if valStart < 0 {
			return opts, s.errAt(tokens[eq], key+" expects value")
		}
		nextOpt := nextRespJSONOptionIndex(tokens, valStart+1)
		valEnd := len(tokens)
		if nextOpt >= 0 {
			valEnd = nextOpt
		}
		raw := strings.TrimSpace(joinTokenText(tokens[valStart:valEnd]))
		if raw == "" {
			return opts, s.errAt(tokens[valStart], key+" expects value")
		}
		switch key {
		case "event":
			opts.Event = strings.TrimSpace(unquoteIfString(raw))
		case "max_count":
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				return opts, s.errAt(tokens[valStart], "max_count expects non-negative integer")
			}
			opts.MaxCount = n
		}
		i = nextOpt
	}
	return opts, nil
}

func nextRespJSONOptionIndex(tokens []token, start int) int {
	for i := start; i < len(tokens); i++ {
		if isRespJSONOptionStart(tokens, i) {
			return i
		}
	}
	return -1
}

func nextNonTriviaIndex(tokens []token, start int) int {
	for i := start; i < len(tokens); i++ {
		if tokens[i].kind == tokWhitespace || tokens[i].kind == tokComment {
			continue
		}
		return i
	}
	return -1
}

func joinTokenText(tokens []token) string {
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteString(tok.text)
	}
	return b.String()
}

func unquoteIfString(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 {
		if (raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'') {
			return unquoteString(raw)
		}
	}
	return raw
}

func parseSSEJSONDelIfStmt(s *scanner, resp *ResponseDirective) error {
	// sse_json_del_if <cond-jsonpath> <equals-string> <del-jsonpath>;
	condTok := s.nextNonTrivia()
	switch condTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(condTok, "sse_json_del_if expects condition json path")
	}
	condPath := condTok.text
	if condTok.kind == tokString {
		condPath = unquoteString(condTok.text)
	}

	eqTok := s.nextNonTrivia()
	if eqTok.kind != tokString {
		return s.errAt(eqTok, "sse_json_del_if expects equals string literal")
	}
	equals := strings.TrimSpace(unquoteString(eqTok.text))

	delTok := s.nextNonTrivia()
	switch delTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(delTok, "sse_json_del_if expects delete json path")
	}
	delPath := delTok.text
	if delTok.kind == tokString {
		delPath = unquoteString(delTok.text)
	}

	if err := consumeSemicolon(s, "sse_json_del_if"); err != nil {
		return err
	}
	resp.SSEJSONDelIf = append(resp.SSEJSONDelIf, SSEJSONDelIfRule{
		CondPath: strings.TrimSpace(condPath),
		Equals:   equals,
		DelPath:  strings.TrimSpace(delPath),
	})
	return nil
}
