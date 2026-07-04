package dslconfig

import (
	"fmt"
	"strconv"
	"strings"
)

// parseNumberValueTokens parses one decimal number starting at first.
// Numbers may be quoted ("0.001") or bare (0.25, -3); bare numbers are split
// into single-char tokens by the scanner, so consecutive non-trivia tokens are
// re-joined until whitespace/semicolon.
func parseNumberValueTokens(s *scanner, first token) (float64, error) {
	raw := first.text
	if first.kind == tokString {
		raw = unquoteString(first.text)
	} else {
		for {
			save := s.i
			tok := s.next()
			if tok.kind == tokOther || tok.kind == tokIdent {
				raw += tok.text
				continue
			}
			s.i = save
			break
		}
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", raw)
	}
	return f, nil
}

func consumeEquals(s *scanner) error {
	tok := s.nextNonTrivia()
	if tok.kind != tokOther || tok.text != "=" {
		return s.errAt(tok, "expected '='")
	}
	return nil
}

func consumeSemicolon(s *scanner, what string) error {
	tok := s.nextNonTrivia()
	if tok.kind != tokSemicolon {
		return s.errAt(tok, "expected ';' after "+what)
	}
	return nil
}

func consumeExprUntilSemicolon(s *scanner) (string, error) {
	var b strings.Builder
	for {
		tok := s.next()
		if tok.kind == tokEOF {
			return "", s.errAt(tok, "unexpected EOF in expr")
		}
		if tok.kind == tokSemicolon {
			break
		}
		b.WriteString(tok.text)
	}
	return strings.TrimSpace(b.String()), nil
}

func consumeExprUntilSemicolonWithFirst(s *scanner, first token) (string, error) {
	var b strings.Builder
	b.WriteString(first.text)
	for {
		tok := s.next()
		if tok.kind == tokEOF {
			return "", s.errAt(tok, "unexpected EOF in expr")
		}
		if tok.kind == tokSemicolon {
			break
		}
		b.WriteString(tok.text)
	}
	return strings.TrimSpace(b.String()), nil
}

func parseModeArgStmt(s *scanner, directive string) (string, error) {
	// <directive> <mode>;
	tok := s.nextNonTrivia()
	if tok.kind == tokLBrace {
		return "", s.errAt(tok, directive+" does not use braces; use: "+directive+" <mode>;")
	}
	switch tok.kind {
	case tokIdent, tokString:
		// ok
	default:
		return "", s.errAt(tok, directive+" expects mode")
	}
	mode := tok.text
	if tok.kind == tokString {
		mode = unquoteString(tok.text)
	}
	if err := consumeSemicolon(s, directive); err != nil {
		return "", err
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "", s.errAt(tok, directive+" requires mode")
	}
	return mode, nil
}

func skipStmtOrBlock(s *scanner) error {
	tok := s.nextNonTrivia()
	if tok.kind == tokSemicolon {
		return nil
	}
	if tok.kind == tokLBrace {
		if err := skipBalancedBraces(s); err != nil {
			return err
		}
		return nil
	}
	for tok.kind != tokSemicolon && tok.kind != tokEOF {
		if tok.kind == tokLBrace {
			if err := skipBalancedBraces(s); err != nil {
				return err
			}
			return nil
		}
		tok = s.next()
	}
	if tok.kind == tokEOF {
		return s.errAt(tok, "unexpected EOF while skipping statement")
	}
	return nil
}

func skipBalancedBraces(s *scanner) error {
	depth := 1
	for depth > 0 {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return s.errAt(tok, "unexpected EOF while skipping block")
		}
		switch tok.kind {
		case tokLBrace:
			depth++
		case tokRBrace:
			depth--
		}
	}
	return nil
}
