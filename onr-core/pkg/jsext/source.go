package jsext

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
)

// Translate the one-line function wrapper back to the original source. The
// first body line can start after a DSL brace; later columns need no adjustment.
func sourceError(path, body string, line, column int, err error) error {
	sourceLine, sourceColumn := 2, 1
	message := err.Error()
	var syntax *goja.CompilerSyntaxError
	var exception *goja.Exception
	switch {
	case errors.As(err, &syntax):
		if syntax.File != nil {
			pos := syntax.File.Position(syntax.Offset)
			sourceLine, sourceColumn = pos.Line, pos.Column
			message = syntax.Message
		}
	case errors.As(err, &exception):
		if stack := exception.Stack(); len(stack) > 0 {
			pos := stack[0].Position()
			sourceLine, sourceColumn = pos.Line, pos.Column
		}
	default:
		if list, ok := err.(parser.ErrorList); ok && len(list) > 0 {
			sourceLine, sourceColumn = list[0].Position.Line, list[0].Position.Column
			message = list[0].Message
		}
	}
	// Errors at the synthetic closing wrapper belong to the original EOF.
	lastBodyLine := strings.Count(body, "\n")
	if sourceLine-2 > lastBodyLine {
		sourceLine = lastBodyLine + 2
		sourceColumn = len(body) - strings.LastIndex(body, "\n")
	}
	if sourceLine <= 2 {
		sourceColumn += max(column, 1) - 1
	}
	sourceLine = max(line, 1) + max(sourceLine-2, 0)
	return &sourceDiagnostic{message: fmt.Sprintf("%s:%d:%d: %s", path, sourceLine, sourceColumn, message), cause: err}
}

type sourceDiagnostic struct {
	message string
	cause   error
}

func (e *sourceDiagnostic) Error() string { return e.message }
func (e *sourceDiagnostic) Unwrap() error { return e.cause }
