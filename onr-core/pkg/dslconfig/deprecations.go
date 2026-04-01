package dslconfig

import (
	"fmt"
	"sort"
	"strings"
)

// ValidationWarning is a non-fatal validation diagnostic.
// It is intended for migration hints (for example, deprecated directives).
type ValidationWarning struct {
	File      string
	Line      int
	Column    int
	Directive string
	Message   string
}

func (w ValidationWarning) String() string {
	file := strings.TrimSpace(w.File)
	if file == "" {
		file = "<unknown>"
	}
	if w.Line > 0 && w.Column > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", file, w.Line, w.Column, strings.TrimSpace(w.Message))
	}
	return fmt.Sprintf("%s: %s", file, strings.TrimSpace(w.Message))
}

var deprecatedDirectiveAliasMap = map[string]string{}

func collectDeprecatedDirectiveWarnings(path, content string) []ValidationWarning {
	s := newScanner(path, content)
	out := make([]ValidationWarning, 0, 4)
	prevSig := tokEOF
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind != tokIdent {
			prevSig = tok.kind
			continue
		}
		// Treat identifier as directive keyword only at statement starts.
		if prevSig != tokEOF && prevSig != tokSemicolon && prevSig != tokLBrace && prevSig != tokRBrace {
			prevSig = tok.kind
			continue
		}
		replacement, ok := deprecatedDirectiveAliasMap[tok.text]
		if !ok {
			prevSig = tok.kind
			continue
		}
		line, col := s.lineCol(tok.pos)
		out = append(out, ValidationWarning{
			File:      path,
			Line:      line,
			Column:    col,
			Directive: tok.text,
			Message:   fmt.Sprintf("directive %q is deprecated; use %q", tok.text, replacement),
		})
		prevSig = tok.kind
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		if out[i].Column != out[j].Column {
			return out[i].Column < out[j].Column
		}
		return out[i].Directive < out[j].Directive
	})
	return out
}
