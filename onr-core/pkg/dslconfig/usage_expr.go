package dslconfig

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// UsageExpr is a restricted arithmetic expression used by usage_extract custom.
// Grammar (no parentheses):
//
//	expr  := term (('+'|'-') term)*
//	term  := jsonpath | int
//
// jsonpath is limited to the same subset supported by getIntByPath.
type UsageExpr struct {
	kind usageExprKind

	value int
	path  string

	left  *UsageExpr
	right *UsageExpr
}

type usageExprKind int

const (
	usageExprInt usageExprKind = iota
	usageExprPath
	usageExprAdd
	usageExprSub
)

func (e *UsageExpr) Eval(root map[string]any) int {
	if e == nil {
		return 0
	}
	switch e.kind {
	case usageExprInt:
		return e.value
	case usageExprPath:
		return jsonutil.GetIntByPath(root, e.path)
	case usageExprAdd:
		return e.left.Eval(root) + e.right.Eval(root)
	case usageExprSub:
		return e.left.Eval(root) - e.right.Eval(root)
	default:
		return 0
	}
}

func ParseUsageExpr(s string) (*UsageExpr, error) {
	p := &usageExprParser{src: strings.TrimSpace(s)}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.skipSpaces()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("unexpected token at %d", p.pos)
	}
	return expr, nil
}

type usageExprParser struct {
	src string
	pos int
}

func (p *usageExprParser) parseExpr() (*UsageExpr, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.src) {
			return left, nil
		}
		op := p.src[p.pos]
		if op != '+' && op != '-' {
			return left, nil
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		if op == '+' {
			left = &UsageExpr{kind: usageExprAdd, left: left, right: right}
		} else {
			left = &UsageExpr{kind: usageExprSub, left: left, right: right}
		}
	}
}

func (p *usageExprParser) parseTerm() (*UsageExpr, error) {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	if p.src[p.pos] == '$' {
		path, err := p.parseJSONPath()
		if err != nil {
			return nil, err
		}
		return &UsageExpr{kind: usageExprPath, path: path}, nil
	}
	if isDigit(p.src[p.pos]) {
		n, err := p.parseInt()
		if err != nil {
			return nil, err
		}
		return &UsageExpr{kind: usageExprInt, value: n}, nil
	}
	return nil, fmt.Errorf("expected jsonpath or int at %d", p.pos)
}

func (p *usageExprParser) parseInt() (int, error) {
	start := p.pos
	for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		p.pos++
	}
	v, err := strconv.Atoi(p.src[start:p.pos])
	if err != nil {
		return 0, fmt.Errorf("invalid int %q", p.src[start:p.pos])
	}
	return v, nil
}

func (p *usageExprParser) parseJSONPath() (string, error) {
	start := p.pos
	if !strings.HasPrefix(p.src[start:], "$.") {
		return "", fmt.Errorf("jsonpath must start with $. at %d", p.pos)
	}
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '+' || ch == '-' {
			break
		}
		if !isAllowedJSONPathChar(ch) {
			return "", fmt.Errorf("invalid jsonpath char %q at %d", ch, p.pos)
		}
		p.pos++
	}
	path := strings.TrimSpace(p.src[start:p.pos])
	if path == "$." {
		return "", fmt.Errorf("jsonpath is empty")
	}
	return path, nil
}

func (p *usageExprParser) skipSpaces() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAllowedJSONPathChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '$', '.', '_', '-', '[', ']', '*':
		return true
	default:
		return false
	}
}
