// Package jslex identifies JavaScript function bodies embedded in the DSL.
package jslex

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

// BodyEnd returns the exclusive end of the first braced function body. The
// JavaScript parser, rather than a brace counter, distinguishes regex literals,
// division, comments and nested template substitutions. Trailing DSL errors are
// ignored only after the isolated body has passed a second, strict parse.
func BodyEnd(source string) (int, error) {
	const prefix = "function __onr(ctx)"
	if len(source) == 0 || source[0] != '{' {
		return 0, fmt.Errorf("expected JavaScript body")
	}
	tree, parseErr := parser.ParseFile(nil, "", prefix+source, 0)
	if tree == nil || len(tree.Body) == 0 {
		if parseErr != nil {
			return 0, bodySyntaxError(source, prefix, parseErr)
		}
		return 0, &SyntaxError{Message: "unclosed JavaScript body"}
	}
	fn, ok := tree.Body[0].(*ast.FunctionDeclaration)
	if !ok || fn.Function.Body == nil {
		return 0, fmt.Errorf("invalid JavaScript body")
	}
	end := int(fn.Function.Body.RightBrace) - len(prefix)
	if end < 2 || end > len(source) || source[end-1] != '}' {
		if parseErr != nil {
			return 0, bodySyntaxError(source, prefix, parseErr)
		}
		return 0, &SyntaxError{Message: "unclosed JavaScript body"}
	}
	if _, err := parser.ParseFile(nil, "", prefix+source[:end], 0); err != nil {
		return 0, bodySyntaxError(source, prefix, err)
	}
	return end, nil
}

type Island struct{ Start, End int }

// Islands skips DSL strings and comments, returning only *_by_js_block bodies.
func Islands(source string) ([]Island, error) {
	var out []Island
	pending := false
	for i := 0; i < len(source); {
		c := source[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
			continue
		}
		if c == '#' || (c == '/' && i+1 < len(source) && source[i+1] == '/') {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if pending && c == '{' {
			n, err := BodyEnd(source[i:])
			if err != nil {
				var syntax *SyntaxError
				if errors.As(err, &syntax) {
					return out, &SyntaxError{Offset: i + syntax.Offset, Message: syntax.Message}
				}
				return out, &SyntaxError{Offset: i, Message: err.Error()}
			}
			out = append(out, Island{i, i + n})
			i += n
			pending = false
			continue
		}
		pending = false
		if c == '\'' || c == '"' {
			i = skipQuoted(source, i)
			continue
		}

		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			start := i
			for i < len(source) && (identifierPart(source[i])) {
				i++
			}
			pending = strings.HasSuffix(source[start:i], "_by_js_block")
			continue
		}
		i++
	}
	return out, nil
}

// Mask preserves offsets and newlines for DSL tools without interpreting JS.
func Mask(source string) (string, error) {
	islands, err := Islands(source)
	b := []byte(source)
	for _, s := range islands {
		for i := s.Start + 1; i < s.End-1; i++ {
			if b[i] != '\n' && b[i] != '\r' {
				b[i] = ' '
			}
		}
	}
	return string(b), err
}

func skipQuoted(source string, i int) int {
	q := source[i]
	i++
	for i < len(source) {
		c := source[i]
		i++
		if c == '\\' && i < len(source) {
			i++
			continue
		}
		if c == q {
			break
		}
	}
	return i
}
func identifierPart(c byte) bool {
	return c == '_' || c == '-' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
