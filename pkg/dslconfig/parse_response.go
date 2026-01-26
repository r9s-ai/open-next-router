package dslconfig

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
