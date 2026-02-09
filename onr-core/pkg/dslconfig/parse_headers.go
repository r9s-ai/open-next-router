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

	handlers := map[string]func(*scanner, *PhaseHeaders) error{
		"auth_bearer":       parseAuthBearerStmt,
		"auth_header_key":   parseAuthHeaderKeyStmt,
		"auth_oauth_bearer": parseAuthOAuthBearerStmt,
		"oauth_mode": func(s *scanner, phase *PhaseHeaders) error {
			mode, err := parseModeArgStmt(s, "oauth_mode")
			if err != nil {
				return err
			}
			phase.OAuth.Mode = strings.TrimSpace(mode)
			return nil
		},
		"oauth_token_url": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthExprStmt(s, "oauth_token_url", &phase.OAuth.TokenURLExpr)
		},
		"oauth_client_id": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthExprStmt(s, "oauth_client_id", &phase.OAuth.ClientIDExpr)
		},
		"oauth_client_secret": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthExprStmt(s, "oauth_client_secret", &phase.OAuth.ClientSecretExpr)
		},
		"oauth_refresh_token": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthExprStmt(s, "oauth_refresh_token", &phase.OAuth.RefreshTokenExpr)
		},
		"oauth_scope": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthExprStmt(s, "oauth_scope", &phase.OAuth.ScopeExpr)
		},
		"oauth_audience": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthExprStmt(s, "oauth_audience", &phase.OAuth.AudienceExpr)
		},
		"oauth_method": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthMethodStmt(s, &phase.OAuth)
		},
		"oauth_content_type": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthContentTypeStmt(s, &phase.OAuth)
		},
		"oauth_token_path": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthLiteralStmt(s, "oauth_token_path", &phase.OAuth.TokenPath)
		},
		"oauth_expires_in_path": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthLiteralStmt(s, "oauth_expires_in_path", &phase.OAuth.ExpiresInPath)
		},
		"oauth_token_type_path": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthLiteralStmt(s, "oauth_token_type_path", &phase.OAuth.TokenTypePath)
		},
		"oauth_timeout_ms": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthIntStmt(s, "oauth_timeout_ms", &phase.OAuth.TimeoutMs)
		},
		"oauth_refresh_skew_sec": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthIntStmt(s, "oauth_refresh_skew_sec", &phase.OAuth.RefreshSkewSec)
		},
		"oauth_fallback_ttl_sec": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthIntStmt(s, "oauth_fallback_ttl_sec", &phase.OAuth.FallbackTTLSeconds)
		},
		"oauth_form": func(s *scanner, phase *PhaseHeaders) error {
			return parseOAuthFormStmt(s, &phase.OAuth)
		},
	}

	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in auth phase")
		case tokRBrace:
			return nil
		case tokIdent:
			if handler, ok := handlers[tok.text]; ok {
				if err := handler(s, phase); err != nil {
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
			return parseJSONSetStmt(s, t, jsonOpSet)
		},
		"json_set_if_absent": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			return parseJSONSetStmt(s, t, jsonOpSetIfAbsent)
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
		"req_map": func(s *scanner, _ *PhaseHeaders, t *RequestTransform) error {
			if t == nil {
				return skipStmtOrBlock(s)
			}
			mode, err := parseModeArgStmt(s, "req_map")
			if err != nil {
				return err
			}
			t.ReqMapMode = mode
			return nil
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

func parseJSONSetStmt(s *scanner, t *RequestTransform, op string) error {
	// json_set/json_set_if_absent <jsonpath> <expr>;
	opName := strings.TrimSpace(op)
	if opName == "" {
		opName = jsonOpSet
	}
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
	valueExpr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	t.JSONOps = append(t.JSONOps, JSONOp{
		Op:        opName,
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
		Op:   jsonOpDel,
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
		Op:       jsonOpRename,
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

func parseAuthOAuthBearerStmt(s *scanner, phase *PhaseHeaders) error {
	if err := consumeSemicolon(s, "auth_oauth_bearer"); err != nil {
		return err
	}
	phase.Auth = append(phase.Auth, HeaderOp{
		Op:        "header_set",
		NameExpr:  `"Authorization"`,
		ValueExpr: `concat("Bearer ", ` + exprOAuthAccessToken + `)`,
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

func parseOAuthExprStmt(s *scanner, _ string, out *string) error {
	expr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	*out = strings.TrimSpace(expr)
	return nil
}

func parseOAuthLiteralStmt(s *scanner, directive string, out *string) error {
	tok := s.nextNonTrivia()
	if tok.kind == tokOther && tok.text == "=" {
		return s.errAt(tok, directive+" does not use '='; use: "+directive+" <value>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, tok)
	if err != nil {
		return err
	}
	v := strings.TrimSpace(expr)
	if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		v = strings.TrimSpace(unquoteString(v))
	}
	*out = v
	return nil
}

func parseOAuthMethodStmt(s *scanner, cfg *OAuthConfig) error {
	tok := s.nextNonTrivia()
	if tok.kind == tokOther && tok.text == "=" {
		return s.errAt(tok, "oauth_method does not use '='; use: oauth_method <GET|POST>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, tok)
	if err != nil {
		return err
	}
	v := strings.ToUpper(strings.TrimSpace(expr))
	v = strings.Trim(v, "\"")
	if v == "" {
		return s.errAt(tok, "oauth_method requires value")
	}
	cfg.Method = v
	return nil
}

func parseOAuthContentTypeStmt(s *scanner, cfg *OAuthConfig) error {
	tok := s.nextNonTrivia()
	if tok.kind == tokOther && tok.text == "=" {
		return s.errAt(tok, "oauth_content_type does not use '='; use: oauth_content_type <form|json>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, tok)
	if err != nil {
		return err
	}
	v := strings.ToLower(strings.TrimSpace(expr))
	v = strings.Trim(v, "\"")
	if v == "" {
		return s.errAt(tok, "oauth_content_type requires value")
	}
	cfg.ContentType = v
	return nil
}

func parseOAuthIntStmt(s *scanner, directive string, out **int) error {
	tok := s.nextNonTrivia()
	if tok.kind == tokOther && tok.text == "=" {
		return s.errAt(tok, directive+" does not use '='; use: "+directive+" <int>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, tok)
	if err != nil {
		return err
	}
	v := strings.TrimSpace(strings.Trim(expr, `"`))
	n, err := strconv.Atoi(v)
	if err != nil {
		return s.errAt(tok, directive+" expects integer")
	}
	*out = &n
	return nil
}

func parseOAuthFormStmt(s *scanner, cfg *OAuthConfig) error {
	keyTok := s.nextNonTrivia()
	switch keyTok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return s.errAt(keyTok, "oauth_form expects key")
	}
	key := keyTok.text
	if keyTok.kind == tokString {
		key = unquoteString(keyTok.text)
	}
	expr, err := consumeExprUntilSemicolon(s)
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return s.errAt(keyTok, "oauth_form key is empty")
	}
	cfg.Form = append(cfg.Form, OAuthFormField{
		Key:       key,
		ValueExpr: strings.TrimSpace(expr),
	})
	return nil
}
