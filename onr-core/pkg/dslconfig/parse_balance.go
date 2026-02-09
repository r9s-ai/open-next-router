package dslconfig

import "strings"

const balanceExprKey = "balance"

func parseBalancePhase(s *scanner, cfg *BalanceQueryConfig) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after balance")
	}

	var hdr PhaseHeaders
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in balance phase")
		case tokRBrace:
			if len(hdr.Request) > 0 {
				cfg.Headers = append(cfg.Headers, hdr.Request...)
			}
			return nil
		case tokIdent:
			switch tok.text {
			case "balance_mode":
				mode, err := parseModeArgStmt(s, "balance_mode")
				if err != nil {
					return err
				}
				cfg.Mode = strings.TrimSpace(mode)
			case "method", "path", "balance_path", "used_path", "balance_unit", "subscription_path", "usage_path":
				v, err := parseBalanceFieldStmt(s, tok.text)
				if err != nil {
					return err
				}
				switch tok.text {
				case "method":
					cfg.Method = v
				case "path":
					cfg.Path = v
				case "balance_path":
					cfg.BalancePath = v
				case "used_path":
					cfg.UsedPath = v
				case "balance_unit":
					cfg.Unit = v
				case "subscription_path":
					cfg.SubscriptionPath = v
				case "usage_path":
					cfg.UsagePath = v
				}
			case balanceExprKey, "used":
				if err := consumeEquals(s); err != nil {
					return err
				}
				expr, err := consumeExprUntilSemicolon(s)
				if err != nil {
					return err
				}
				expr = strings.TrimSpace(expr)
				if tok.text == balanceExprKey {
					cfg.BalanceExpr = expr
				} else {
					cfg.UsedExpr = expr
				}
			case "set_header":
				if err := parseSetHeaderStmt(s, &hdr); err != nil {
					return err
				}
			case "del_header":
				if err := parseDelHeaderStmt(s, &hdr); err != nil {
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

func parseBalanceFieldStmt(s *scanner, key string) (string, error) {
	peek := s.nextNonTrivia()
	if peek.kind == tokOther && peek.text == "=" {
		return "", s.errAt(peek, key+" does not use '='; use: "+key+" <value>;")
	}
	expr, err := consumeExprUntilSemicolonWithFirst(s, peek)
	if err != nil {
		return "", err
	}
	val := strings.TrimSpace(expr)
	if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
		val = strings.TrimSpace(unquoteString(val))
	}
	return val, nil
}
