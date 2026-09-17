package dsllang

import (
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jslex"
)

func formatDocument(text string, opts formattingOptions) string {
	islands, err := jslex.Islands(text)
	if err != nil {
		return text
	}
	if len(islands) > 0 {
		masked := text
		markers := make([]string, len(islands))
		for i := len(islands) - 1; i >= 0; i-- {
			island := islands[i]
			marker := fmt.Sprintf("__ONR_JS_ISLAND_%d__", i)
			for strings.Contains(text, marker) {
				marker += "_"
			}
			markers[i] = "\"" + marker + "\";"
			masked = masked[:island.Start] + markers[i] + masked[island.End:]
		}
		result := formatDSLDocument(masked, opts)
		for i, island := range islands {
			result = strings.ReplaceAll(result, markers[i], text[island.Start:island.End])
		}
		return result
	}
	return formatDSLDocument(text, opts)
}
func formatDSLDocument(text string, opts formattingOptions) string {
	if text == "" {
		return ""
	}

	indentUnit := indentUnitFromOptions(opts)
	hasTrailingNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if hasTrailingNewline && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	expanded := expandCompactLines(lines)
	out := make([]string, 0, len(expanded))
	indent := 0
	for _, raw := range expanded {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			out = append(out, "")
			continue
		}

		leadingClosers := countLeadingClosers(trimmed)
		lineIndent := indent - leadingClosers
		if lineIndent < 0 {
			lineIndent = 0
		}
		out = append(out, strings.Repeat(indentUnit, lineIndent)+trimmed)

		opens, closes := countBracesOutsideStringAndComment(trimmed)
		indent += opens - closes
		if indent < 0 {
			indent = 0
		}
	}

	result := strings.Join(out, "\n")
	if hasTrailingNewline {
		return result + "\n"
	}
	return result
}

func expandCompactLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}
		parts := splitCompactLine(line)
		if len(parts) == 0 {
			out = append(out, strings.TrimSpace(line))
			continue
		}
		out = append(out, parts...)
	}
	return out
}

func splitCompactLine(line string) []string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return []string{""}
	}

	var (
		parts    []string
		current  strings.Builder
		inString bool
	)

	appendCurrent := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			parts = append(parts, s)
		}
		current.Reset()
	}

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if inString {
			current.WriteByte(ch)
			if ch == '\\' && i+1 < len(line) {
				i++
				current.WriteByte(line[i])
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			current.WriteByte(ch)
			continue
		}

		if ch == '#' || (ch == '/' && i+1 < len(line) && line[i+1] == '/') {
			comment := strings.TrimSpace(line[i:])
			base := strings.TrimSpace(current.String())
			if base == "" {
				parts = append(parts, comment)
			} else {
				parts = append(parts, base+" "+comment)
			}
			return parts
		}

		switch ch {
		case '{':
			base := strings.TrimSpace(current.String())
			if base == "" {
				parts = append(parts, "{")
			} else {
				parts = append(parts, base+" {")
			}
			current.Reset()
		case ';':
			base := strings.TrimSpace(current.String())
			if base != "" {
				stmt := base + ";"
				comment := trailingLineComment(line, i+1)
				if comment != "" {
					stmt += " " + comment
					parts = append(parts, stmt)
					return parts
				}
				parts = append(parts, stmt)
			}
			current.Reset()
		case '}':
			appendCurrent()
			comment := trailingLineComment(line, i+1)
			if comment != "" {
				parts = append(parts, "} "+comment)
				return parts
			}
			parts = append(parts, "}")
		default:
			current.WriteByte(ch)
		}
	}

	appendCurrent()
	return parts
}

func trailingLineComment(line string, from int) string {
	if from < 0 || from >= len(line) {
		return ""
	}
	for i := from; i < len(line); i++ {
		ch := line[i]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			continue
		}
		if ch == '#' {
			return strings.TrimSpace(line[i:])
		}
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			return strings.TrimSpace(line[i:])
		}
		return ""
	}
	return ""
}

func indentUnitFromOptions(opts formattingOptions) string {
	if !opts.InsertSpaces {
		return "\t"
	}
	n := opts.TabSize
	if n <= 0 || n > 16 {
		n = 2
	}
	return strings.Repeat(" ", n)
}

func countLeadingClosers(line string) int {
	n := 0
	for i := 0; i < len(line); i++ {
		if line[i] != '}' {
			break
		}
		n++
	}
	return n
}

func countBracesOutsideStringAndComment(line string) (opens int, closes int) {
	inString := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inString {
			if ch == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '#' {
			break
		}
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		if ch == '{' {
			opens++
			continue
		}
		if ch == '}' {
			closes++
		}
	}
	return opens, closes
}

func endPosition(text string) Position {
	line := 0
	col := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			line++
			col = 0
			continue
		}
		col++
	}
	return Position{Line: line, Character: col}
}
