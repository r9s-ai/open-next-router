package dslconfig

import "strings"

var supportedErrorMapModes = map[string]struct{}{
	"openai": {},
}

func parseErrorPhase(s *scanner, errDir *ErrorDirective) error {
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return s.errAt(lb, "expected '{' after error")
	}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return s.errAt(tok, "unexpected EOF in error phase")
		case tokRBrace:
			return nil
		case tokIdent:
			switch tok.text {
			case "error_map":
				mode, err := parseModeArgStmt(s, "error_map")
				if err != nil {
					return err
				}
				mode = strings.ToLower(strings.TrimSpace(mode))
				if _, ok := supportedErrorMapModes[mode]; !ok {
					return s.errAt(tok, "unsupported error_map mode "+mode)
				}
				errDir.Op = "error_map"
				errDir.Mode = mode
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
