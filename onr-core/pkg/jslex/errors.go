package jslex

import (
	"fmt"
	"strings"

	"github.com/dop251/goja/parser"
)

// SyntaxError uses a byte offset in the original, unwrapped source.
type SyntaxError struct {
	Offset  int
	Message string
}

func (e *SyntaxError) Error() string { return e.Message }
func bodySyntaxError(source, prefix string, err error) error {
	list, ok := err.(parser.ErrorList)
	if !ok || len(list) == 0 {
		return &SyntaxError{Message: err.Error()}
	}
	first := list[0]
	line, col := first.Position.Line, first.Position.Column
	if line == 1 {
		col -= len(prefix)
	}
	offset := 0
	for n := 1; n < line; n++ {
		next := strings.IndexByte(source[offset:], '\n')
		if next < 0 {
			offset = len(source)
			break
		}
		offset += next + 1
	}
	offset += max(col-1, 0)
	offset = min(offset, len(source))
	return &SyntaxError{Offset: offset, Message: fmt.Sprintf("JavaScript: %s", first.Message)}
}
