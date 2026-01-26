package dslconfig

import (
	"strconv"
	"strings"
)

func parseAuthPhase(s *scanner, phase *PhaseHeaders) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after auth")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in auth phase")
		case tokRBrace:
			return nil
		case tokIdent:
			switch tok.text {
			case "auth_bearer":
				if err := parseAuthBearerStmt(s, phase); err != nil {
					return err
				}
			case "auth_header_key":
				if err := parseAuthHeaderKeyStmt(s, phase); err != nil {
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

func parseRequestPhaseWithTransform(s *scanner, phase *PhaseHeaders, transform *RequestTransform) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after request")
	}

	handlers := map[string]func(*scanner, *PhaseHeaders, *RequestTransform) error{
		"set_header": func(s *scanner, phase *PhaseHeaders, _ *RequestTransform) error {
			return parseSetHeaderStmt(s, phase)
		},
		"del_header": func(s *scanner, phase *PhaseHeaders, _ *RequestTransform) error {
			return parseDelHeaderStmt(s, phase)
		},
		"model_map": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			return parseModelMapStmt(s, &t.ModelMap)
		},
		"model_map_default": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			return parseModelMapDefaultStmt(s, &t.ModelMap)
		},
		"json_set": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			return parseJSONSetStmt(s, t)
		},
		"json_del": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			return parseJSONDelStmt(s, t)
		},
		"json_rename": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			return parseJSONRenameStmt(s, t)
		},
	}

	removed := map[string]string{
		"header_set":       "header_set has been removed; use: set_header <Header-Name> <expr>;",
		"header_del":       "header_del has been removed; use: del_header <Header-Name>;",
		"proxy_set_header": "proxy_set_header is not supported; use: set_header <Header-Name> <expr>;",
	}

	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in request phase")
		case tokRBrace:
			return nil
		case tokIdent:
			if msg, ok := removed[tok.text]; ok {
				return s.errAt(tok, msg)
			}
			if handler, ok := handlers[tok.text]; ok {
				if err := handler(s, phase, transform); err != nil {
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

func parseModelMapStmt(s *scanner, cfg *ModelMapConfig) error {
	// model_map <from> <expr>;
	fromTok := s.nextNonTrivia()
	switch fromTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(fromTok, "model_map expects from model name")
	}
	from := fromTok.text
	if fromTok.kind == tokString {
		from = unquoteString(fromTok.text)
	}
	valueExpr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	from = strings.TrimSpace(from)
	if from == "" {
		return nil
	}
	if cfg.Map == nil {
		cfg.Map = map[string]string{}
	}
	cfg.Map[from] = strings.TrimSpace(valueExpr)
	return nil
}

func parseModelMapDefaultStmt(s *scanner, cfg *ModelMapConfig) error {
	// model_map_default <expr>;
	expr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	cfg.DefaultExpr = strings.TrimSpace(expr)
	return nil
}

func parseJSONSetStmt(s *scanner, t *RequestTransform) error {
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
	t.JSONOps = append(t.JSONOps, JSONOp{
		Op:        "json_set",
		Path:      strings.TrimSpace(path),
		ValueExpr: strings.TrimSpace(valueExpr),
	})
	return nil
}

func parseJSONDelStmt(s *scanner, t *RequestTransform) error {
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
	t.JSONOps = append(t.JSONOps, JSONOp{
		Op:   "json_del",
		Path: strings.TrimSpace(path),
	})
	return nil
}

func parseJSONRenameStmt(s *scanner, t *RequestTransform) error {
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
	t.JSONOps = append(t.JSONOps, JSONOp{
		Op:       "json_rename",
		FromPath: strings.TrimSpace(from),
		ToPath:   strings.TrimSpace(to),
	})
	return nil
}

func parseAuthBearerStmt(s *scanner, phase *PhaseHeaders) error {
	// auth_bearer;
	if err := consumeSemicolon(s, "auth_bearer"); err != nil {
		return err
	}
	phase.Auth = append(phase.Auth, HeaderOp{
		Op:        "header_set",
		NameExpr:  `"Authorization"`,
		ValueExpr: `concat("Bearer ", ` + exprChannelKey + `)`,
	})
	return nil
}

func parseAuthHeaderKeyStmt(s *scanner, phase *PhaseHeaders) error {
	// auth_header_key <Header-Name>;
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(nameTok, "auth_header_key expects header name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	if err := consumeSemicolon(s, "auth_header_key"); err != nil {
		return err
	}
	phase.Auth = append(phase.Auth, HeaderOp{
		Op:        "header_set",
		NameExpr:  strconv.Quote(strings.TrimSpace(name)),
		ValueExpr: exprChannelKey,
	})
	return nil
}

func parseSetHeaderStmt(s *scanner, phase *PhaseHeaders) error {
	// set_header <Header-Name> <expr>;
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(nameTok, "set_header expects header name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	valueExpr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	phase.Request = append(phase.Request, HeaderOp{
		Op:        "header_set",
		NameExpr:  strconv.Quote(strings.TrimSpace(name)),
		ValueExpr: strings.TrimSpace(valueExpr),
	})
	return nil
}

func parseDelHeaderStmt(s *scanner, phase *PhaseHeaders) error {
	// del_header <Header-Name>;
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(nameTok, "del_header expects header name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	if err := consumeSemicolon(s, "del_header"); err != nil {
		return err
	}
	phase.Request = append(phase.Request, HeaderOp{
		Op:       "header_del",
		NameExpr: strconv.Quote(strings.TrimSpace(name)),
	})
	return nil
}
