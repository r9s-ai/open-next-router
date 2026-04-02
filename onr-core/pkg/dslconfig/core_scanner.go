package dslconfig

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokWhitespace
	tokComment
	tokIdent
	tokString
	tokLBrace
	tokRBrace
	tokSemicolon
	tokOther
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

type scanner struct {
	path    string
	input   string
	i       int
	lastPos int
}

func newScanner(path, input string) *scanner {
	return &scanner{path: path, input: input}
}

func (s *scanner) nextNonTrivia() token {
	for {
		tok := s.next()
		if tok.kind == tokWhitespace || tok.kind == tokComment {
			continue
		}
		return tok
	}
}

func (s *scanner) next() token {
	s.lastPos = s.i
	if s.i >= len(s.input) {
		return token{kind: tokEOF, text: "", pos: s.i}
	}
	if tok, ok := s.scanWhitespace(); ok {
		return tok
	}
	if tok, ok := s.scanComment(); ok {
		return tok
	}
	if tok, ok := s.scanString(); ok {
		return tok
	}
	if tok, ok := s.scanSingleChar(); ok {
		return tok
	}
	if tok, ok := s.scanIdent(); ok {
		return tok
	}
	return s.scanOther()
}

func (s *scanner) scanWhitespace() (token, bool) {
	if !isSpace(s.input[s.i]) {
		return token{}, false
	}
	start := s.i
	for s.i < len(s.input) && isSpace(s.input[s.i]) {
		s.i++
	}
	return token{kind: tokWhitespace, text: s.input[start:s.i], pos: start}, true
}

func (s *scanner) scanComment() (token, bool) {
	switch {
	case s.input[s.i] == '#':
		start := s.i
		for s.i < len(s.input) && s.input[s.i] != '\n' {
			s.i++
		}
		return token{kind: tokComment, text: s.input[start:s.i], pos: start}, true
	case s.hasPrefix("//"):
		start := s.i
		s.i += 2
		for s.i < len(s.input) && s.input[s.i] != '\n' {
			s.i++
		}
		return token{kind: tokComment, text: s.input[start:s.i], pos: start}, true
	case s.hasPrefix("/*"):
		start := s.i
		s.i += 2
		for s.i+1 < len(s.input) && (s.input[s.i] != '*' || s.input[s.i+1] != '/') {
			s.i++
		}
		if s.i+1 < len(s.input) {
			s.i += 2
		}
		return token{kind: tokComment, text: s.input[start:s.i], pos: start}, true
	default:
		return token{}, false
	}
}

func (s *scanner) scanString() (token, bool) {
	quote := s.input[s.i]
	if quote != '"' && quote != '\'' {
		return token{}, false
	}
	start := s.i
	s.i++
	for s.i < len(s.input) {
		c := s.input[s.i]
		if c == '\\' {
			s.i += 2
			continue
		}
		s.i++
		if c == quote {
			break
		}
	}
	return token{kind: tokString, text: s.input[start:s.i], pos: start}, true
}

func (s *scanner) scanSingleChar() (token, bool) {
	start := s.i
	switch s.input[s.i] {
	case '{':
		s.i++
		return token{kind: tokLBrace, text: "{", pos: start}, true
	case '}':
		s.i++
		return token{kind: tokRBrace, text: "}", pos: start}, true
	case ';':
		s.i++
		return token{kind: tokSemicolon, text: ";", pos: start}, true
	default:
		return token{}, false
	}
}

func (s *scanner) scanIdent() (token, bool) {
	if !isIdentStart(rune(s.input[s.i])) {
		return token{}, false
	}
	start := s.i
	s.i++
	for s.i < len(s.input) && isIdentPart(rune(s.input[s.i])) {
		s.i++
	}
	return token{kind: tokIdent, text: s.input[start:s.i], pos: start}, true
}

func (s *scanner) scanOther() token {
	s.i++
	return token{kind: tokOther, text: s.input[s.i-1 : s.i], pos: s.i - 1}
}

func (s *scanner) hasPrefix(prefix string) bool {
	return strings.HasPrefix(s.input[s.i:], prefix)
}

func (s *scanner) errAt(tok token, msg string) error {
	line, col := s.lineCol(tok.pos)
	return fmt.Errorf("%s:%d:%d: %s", s.path, line, col, msg)
}

func (s *scanner) lineCol(pos int) (line int, col int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(s.input) {
		pos = len(s.input)
	}
	line = 1
	lastNL := -1
	for i := 0; i < pos; i++ {
		if s.input[i] == '\n' {
			line++
			lastNL = i
		}
	}
	col = pos - lastNL
	return line, col
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	if isIdentStart(r) {
		return true
	}
	if unicode.IsDigit(r) {
		return true
	}
	switch r {
	case '-', ':', '.':
		return true
	}
	return false
}

func findProviderName(path string, content string) (string, error) {
	s := newScanner(path, content)
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return "", fmt.Errorf("%s: no provider block found", path)
		}
		if tok.kind != tokIdent || tok.text != "provider" {
			continue
		}
		nameTok := s.nextNonTrivia()
		if nameTok.kind != tokString {
			return "", s.errAt(nameTok, "expected string literal after provider")
		}
		lb := s.nextNonTrivia()
		if lb.kind != tokLBrace {
			return "", s.errAt(lb, "expected '{' after provider name")
		}
		name := strings.TrimSpace(unquoteString(nameTok.text))
		if name == "" {
			return "", s.errAt(nameTok, "provider name is empty")
		}
		return name, nil
	}
}
