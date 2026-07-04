package dsllang

import "github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"

func collectHover(text string, pos Position) (*Hover, bool) {
	word, rng := wordAt(text, pos)
	if word == "" {
		return nil, false
	}
	block := CurrentBlock(text, pos)
	doc, ok := dslspec.DirectiveHoverInBlock(word, block)
	if !ok {
		return nil, false
	}
	return &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: doc,
		},
		Range: &rng,
		Word:  word,
		Block: block,
	}, true
}

func wordAt(text string, pos Position) (string, Range) {
	line := lineAt(text, pos.Line)
	if line == "" {
		return "", Range{}
	}
	ch := pos.Character
	if ch < 0 {
		ch = 0
	}
	if ch > len(line) {
		ch = len(line)
	}
	left := ch
	for left > 0 && isWordChar(line[left-1]) {
		left--
	}
	right := ch
	for right < len(line) && isWordChar(line[right]) {
		right++
	}
	if left == right {
		return "", Range{}
	}
	return line[left:right], Range{
		Start: Position{Line: pos.Line, Character: left},
		End:   Position{Line: pos.Line, Character: right},
	}
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '.'
}

func lineAt(text string, line int) string {
	if line < 0 {
		return ""
	}
	start := 0
	current := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '\n' {
			continue
		}
		if current == line {
			return text[start:i]
		}
		current++
		start = i + 1
	}
	if current == line {
		return text[start:]
	}
	return ""
}
