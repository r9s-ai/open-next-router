package dsllang

import (
	"errors"
	"sort"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jslex"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
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
	line int
	col  int
}

type parser struct {
	tokens []token
	i      int
	diags  []Diagnostic
}

func analyze(text string) []Diagnostic {
	p := &parser{tokens: lex(text)}
	if _, err := jslex.Islands(text); err != nil {
		var syntax *jslex.SyntaxError
		if errors.As(err, &syntax) {
			prefix := text[:min(syntax.Offset, len(text))]
			line := strings.Count(prefix, "\n")
			col := len(prefix) - strings.LastIndex(prefix, "\n") - 1
			p.add(token{line: line, col: col}, syntax.Message)
		} else {
			p.add(token{}, err.Error())
		}
	}
	p.parseFile()
	return p.diags
}

func (p *parser) parseFile() {
	for {
		tok := p.peek()
		if tok.kind == tokEOF {
			return
		}
		if tok.kind != tokIdent {
			p.next()
			continue
		}
		if tok.text == "syntax" {
			p.next() // syntax
			v := p.next()
			if v.kind != tokString {
				p.add(v, "expected syntax version string literal")
			}
			semi := p.next()
			if semi.kind != tokSemicolon {
				p.add(semi, "expected ';' after syntax directive")
			}
			continue
		}
		if tok.text == "include" {
			p.next() // include
			p.parseIncludeDirective()
			continue
		}
		spec, ok := topLevelDirectives[tok.text]
		if !ok {
			p.add(tok, "unknown top-level directive: "+tok.text)
			p.skipStmtOrBlock()
			continue
		}
		p.next()
		if spec.block {
			if spec.parseHeaderUntilBrace {
				if !p.skipUntilLBrace(spec.name) {
					return
				}
			} else {
				lb := p.next()
				if lb.kind != tokLBrace {
					p.add(lb, "expected '{' after "+spec.name)
					continue
				}
			}
			if spec.sub != nil {
				p.parseBlock(spec.name, spec.sub)
			} else {
				p.skipBalancedBlock(spec.name)
			}
			continue
		}
		p.skipStatement(spec.name)
	}
}

func (p *parser) parseIncludeDirective() {
	seenArg := false
	for {
		tok := p.next()
		switch tok.kind {
		case tokSemicolon:
			if !seenArg {
				p.add(tok, "include expects a path")
			}
			return
		case tokEOF:
			p.add(tok, "expected ';' after include directive")
			return
		case tokRBrace:
			p.add(tok, "expected ';' after include directive")
			if p.i > 0 {
				p.i--
			}
			return
		case tokLBrace:
			p.add(tok, "include does not use '{ ... }'; expected ';'")
			p.skipBalancedBlock("include")
			return
		default:
			seenArg = true
		}
	}
}

func (p *parser) parseBlock(name string, directives map[string]directiveSpec) {
	for {
		tok := p.peek()
		switch tok.kind {
		case tokEOF:
			p.addPos(tok.line, tok.col, "missing closing '}' for "+name+" block")
			return
		case tokRBrace:
			p.next()
			return
		case tokIdent:
			spec, ok := directives[tok.text]
			if !ok {
				if allowed := allowedBlocksForDirective(tok.text); len(allowed) > 0 {
					p.add(tok, "directive "+tok.text+" is not allowed in "+name+" block; allowed in: "+strings.Join(allowed, ", ")+"; quick fix: move it into "+allowed[0]+" { ... }")
				} else {
					p.add(tok, "unknown directive in "+name+" block: "+tok.text)
				}
				p.skipStmtOrBlock()
				continue
			}
			p.next()
			if spec.block {
				if spec.parseHeaderUntilBrace {
					if !p.skipUntilLBrace(spec.name) {
						return
					}
				} else {
					lb := p.next()
					if lb.kind != tokLBrace {
						p.add(lb, "expected '{' after "+spec.name)
						continue
					}
				}
				if spec.sub != nil {
					p.parseBlock(spec.name, spec.sub)
				} else {
					p.skipBalancedBlock(spec.name)
				}
				continue
			}
			p.skipStatement(spec.name)
		default:
			p.next()
		}
	}
}

func (p *parser) skipUntilLBrace(name string) bool {
	for {
		tok := p.next()
		switch tok.kind {
		case tokLBrace:
			return true
		case tokEOF:
			p.add(tok, "expected '{' for "+name+" block")
			return false
		}
	}
}

func (p *parser) skipBalancedBlock(name string) {
	depth := 1
	for depth > 0 {
		tok := p.next()
		if tok.kind == tokEOF {
			p.add(tok, "missing closing '}' for "+name+" block")
			return
		}
		if tok.kind == tokLBrace {
			depth++
		}
		if tok.kind == tokRBrace {
			depth--
		}
	}
}

func (p *parser) skipStatement(name string) {
	for {
		tok := p.next()
		switch tok.kind {
		case tokSemicolon:
			return
		case tokLBrace:
			p.add(tok, name+" does not use '{ ... }'; expected ';'")
			p.skipBalancedBlock(name)
			return
		case tokRBrace:
			p.add(tok, "expected ';' after "+name)
			if p.i > 0 {
				// Put '}' back so outer parseBlock can consume it as a block terminator.
				p.i--
			}
			return
		case tokEOF:
			p.add(tok, "expected ';' after "+name)
			return
		}
	}
}

func (p *parser) skipStmtOrBlock() {
	tok := p.peek()
	if tok.kind == tokLBrace {
		p.next()
		p.skipBalancedBlock("unknown")
		return
	}
	for {
		tok := p.next()
		switch tok.kind {
		case tokSemicolon:
			return
		case tokLBrace:
			p.skipBalancedBlock("unknown")
			return
		case tokRBrace, tokEOF:
			return
		}
	}
}

func (p *parser) next() token {
	if p.i >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	t := p.tokens[p.i]
	p.i++
	return t
}

func (p *parser) peek() token {
	if p.i >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[p.i]
}

func (p *parser) add(tok token, msg string) {
	p.addPos(tok.line, tok.col, msg)
}

func (p *parser) addPos(line, col int, msg string) {
	p.diags = append(p.diags, Diagnostic{
		Range: Range{
			Start: Position{Line: line, Character: col},
			End:   Position{Line: line, Character: col + 1},
		},
		Severity: 1,
		Source:   "onr-lsp",
		Message:  msg,
	})
}

type directiveSpec struct {
	name                  string
	block                 bool
	parseHeaderUntilBrace bool
	sub                   map[string]directiveSpec
}

var (
	topLevelDirectives    map[string]directiveSpec
	directiveAllowedInMap map[string][]string
)

func init() {
	initDirectiveSpecsFromMetadata()
}

func initDirectiveSpecsFromMetadata() {
	meta := dslspec.DirectiveMetadataList()
	directiveAllowedInMap = map[string][]string{}
	seenDirectiveBlocks := map[string]map[string]struct{}{}

	for _, item := range meta {
		block := normalizeMetaBlock(item.Block)
		name := strings.TrimSpace(item.Name)
		if block == "" || name == "" {
			continue
		}
		if _, ok := seenDirectiveBlocks[name]; !ok {
			seenDirectiveBlocks[name] = map[string]struct{}{}
		}
		if _, ok := seenDirectiveBlocks[name][block]; ok {
			continue
		}
		seenDirectiveBlocks[name][block] = struct{}{}
		directiveAllowedInMap[name] = append(directiveAllowedInMap[name], block)
	}

	cache := map[string]map[string]directiveSpec{}
	var buildBlockSpecs func(block string, visiting map[string]bool) map[string]directiveSpec
	buildBlockSpecs = func(block string, visiting map[string]bool) map[string]directiveSpec {
		if cached, ok := cache[block]; ok {
			return cached
		}
		if visiting[block] {
			return nil
		}
		visiting[block] = true
		defer delete(visiting, block)

		names := dslspec.DirectivesByBlock(block)
		specs := make(map[string]directiveSpec, len(names))
		for _, name := range names {
			spec := directiveSpec{name: name}
			if blockAllowsChildBlock(block, name) {
				spec.block = true
				spec.parseHeaderUntilBrace = blockDirectiveNeedsHeader(block, name)
				spec.sub = buildBlockSpecs(name, visiting)
			}
			specs[name] = spec
		}
		cache[block] = specs
		return specs
	}

	topLevelDirectives = buildBlockSpecs("top", map[string]bool{})
}

func normalizeMetaBlock(block string) string {
	b := strings.TrimSpace(strings.ToLower(block))
	if b == "_top" {
		return "top"
	}
	return b
}

func allowedBlocksForDirective(d string) []string {
	name := strings.TrimSpace(d)
	if name == "" {
		return nil
	}
	allowed := directiveAllowedInMap[name]
	if len(allowed) == 0 {
		return nil
	}
	out := make([]string, 0, len(allowed))
	for _, block := range allowed {
		if block == "top" {
			continue
		}
		out = append(out, block)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func blockAllowsChildBlock(parent, child string) bool {
	return dslspec.DirectiveIsBlockInBlock(child, parent)
}

func blockDirectiveNeedsHeader(parent, name string) bool {
	return dslspec.DirectiveBlockHasHeaderInBlock(name, parent)
}

func lex(input string) []token {
	input, _ = jslex.Mask(input)
	var out []token
	line, col := 0, 0

	emit := func(kind tokenKind, text string, l, c int) {
		out = append(out, token{kind: kind, text: text, line: l, col: c})
	}

	for i := 0; i < len(input); {
		ch := input[i]
		if ch == '\n' {
			line++
			col = 0
			i++
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\r' {
			col++
			i++
			continue
		}
		if ch == '#' {
			for i < len(input) && input[i] != '\n' {
				i++
				col++
			}
			continue
		}
		if ch == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i < len(input) && input[i] != '\n' {
				i++
				col++
			}
			continue
		}

		startLine, startCol := line, col
		switch ch {
		case '{':
			emit(tokLBrace, "{", startLine, startCol)
			i++
			col++
			continue
		case '}':
			emit(tokRBrace, "}", startLine, startCol)
			i++
			col++
			continue
		case ';':
			emit(tokSemicolon, ";", startLine, startCol)
			i++
			col++
			continue
		case '"', '\'':
			quote := ch
			j := i + 1
			c2 := col + 1
			for j < len(input) {
				if input[j] == '\\' && j+1 < len(input) {
					j += 2
					c2 += 2
					continue
				}
				if input[j] == quote {
					j++
					c2++
					break
				}
				if input[j] == '\n' {
					break
				}
				j++
				c2++
			}
			emit(tokString, input[i:j], startLine, startCol)
			col = c2
			i = j
			continue
		default:
			if isIdentStart(ch) {
				j := i + 1
				for j < len(input) && isIdentPart(input[j]) {
					j++
				}
				txt := input[i:j]
				emit(tokIdent, txt, startLine, startCol)
				col += j - i
				i = j
				continue
			}
			emit(tokOther, string(ch), startLine, startCol)
			i++
			col++
		}
	}
	out = append(out, token{kind: tokEOF, line: line, col: col})
	return out
}

func isIdentStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9') || strings.ContainsRune(".-", rune(b))
}
