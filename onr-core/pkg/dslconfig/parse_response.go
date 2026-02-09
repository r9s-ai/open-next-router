package dslconfig

import "strings"

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
			case jsonOpSet:
				if err := parseRespJSONSetStmt(s, resp); err != nil {
					return err
				}
			case jsonOpSetIfAbsent:
				if err := parseRespJSONSetIfAbsentStmt(s, resp); err != nil {
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

func parseRespJSONSetStmt(s *scanner, resp *ResponseDirective) error {
	// json_set <jsonpath> <expr>;
	pathTok := s.nextNonTrivia()
	switch pathTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(pathTok, "json_set expects json path")
	}
	path := pathTok.text
	if pathTok.kind == tokString {
		path = unquoteString(pathTok.text)
	}
	valueExpr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:        jsonOpSet,
		Path:      strings.TrimSpace(path),
		ValueExpr: strings.TrimSpace(valueExpr),
	})
	return nil
}

func parseRespJSONSetIfAbsentStmt(s *scanner, resp *ResponseDirective) error {
	// json_set_if_absent <jsonpath> <expr>;
	pathTok := s.nextNonTrivia()
	switch pathTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(pathTok, "json_set_if_absent expects json path")
	}
	path := pathTok.text
	if pathTok.kind == tokString {
		path = unquoteString(pathTok.text)
	}
	valueExpr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:        jsonOpSetIfAbsent,
		Path:      strings.TrimSpace(path),
		ValueExpr: strings.TrimSpace(valueExpr),
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
	if err := consumeSemicolon(s, "json_del"); err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:   jsonOpDel,
		Path: strings.TrimSpace(path),
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
	if err := consumeSemicolon(s, "json_rename"); err != nil {
		return err
	}
	resp.JSONOps = append(resp.JSONOps, JSONOp{
		Op:       jsonOpRename,
		FromPath: strings.TrimSpace(from),
		ToPath:   strings.TrimSpace(to),
	})
	return nil
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
