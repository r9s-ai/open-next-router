package dslconfig

import (
	"fmt"
	"strconv"
	"strings"
)

// BalanceExpr is a restricted arithmetic expression used by balance custom mode.
// Grammar (no parentheses):
//
//	expr  := term (('+'|'-') term)*
//	term  := jsonpath | number
type BalanceExpr struct {
	kind balanceExprKind

	value float64
	path  string

	left  *BalanceExpr
	right *BalanceExpr
}

type balanceExprKind int

const (
	balanceExprNumber balanceExprKind = iota
	balanceExprPath
	balanceExprAdd
	balanceExprSub
)

func (e *BalanceExpr) Eval(root map[string]any) float64 {
	if e == nil {
		return 0
	}
	switch e.kind {
	case balanceExprNumber:
		return e.value
	case balanceExprPath:
		return getFloatByPath(root, e.path)
	case balanceExprAdd:
		return e.left.Eval(root) + e.right.Eval(root)
	case balanceExprSub:
		return e.left.Eval(root) - e.right.Eval(root)
	default:
		return 0
	}
}

func ParseBalanceExpr(s string) (*BalanceExpr, error) {
	p := &balanceExprParser{src: strings.TrimSpace(s)}
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

type balanceExprParser struct {
	src string
	pos int
}

func (p *balanceExprParser) parseExpr() (*BalanceExpr, error) {
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
			left = &BalanceExpr{kind: balanceExprAdd, left: left, right: right}
		} else {
			left = &BalanceExpr{kind: balanceExprSub, left: left, right: right}
		}
	}
}

func (p *balanceExprParser) parseTerm() (*BalanceExpr, error) {
	p.skipSpaces()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	if p.src[p.pos] == '$' {
		path, err := p.parseJSONPath()
		if err != nil {
			return nil, err
		}
		return &BalanceExpr{kind: balanceExprPath, path: path}, nil
	}
	if isDigit(p.src[p.pos]) || p.src[p.pos] == '.' {
		n, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		return &BalanceExpr{kind: balanceExprNumber, value: n}, nil
	}
	return nil, fmt.Errorf("expected jsonpath or number at %d", p.pos)
}

func (p *balanceExprParser) parseNumber() (float64, error) {
	start := p.pos
	dotCnt := 0
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if isDigit(ch) {
			p.pos++
			continue
		}
		if ch == '.' {
			dotCnt++
			if dotCnt > 1 {
				break
			}
			p.pos++
			continue
		}
		break
	}
	raw := p.src[start:p.pos]
	if raw == "." {
		return 0, fmt.Errorf("invalid number %q", raw)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", raw)
	}
	return v, nil
}

func (p *balanceExprParser) parseJSONPath() (string, error) {
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

func (p *balanceExprParser) skipSpaces() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}
