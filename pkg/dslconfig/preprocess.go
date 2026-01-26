package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxIncludeDepth = 20
)

func preprocessIncludes(rootPath string, content string) (string, error) {
	visited := map[string]bool{}
	return preprocessIncludesInner(rootPath, content, visited, 0)
}

func preprocessIncludesInner(path string, content string, visited map[string]bool, depth int) (string, error) {
	if depth > maxIncludeDepth {
		return "", fmt.Errorf("include depth exceeded (%d) at %q", maxIncludeDepth, path)
	}
	if visited[path] {
		return "", fmt.Errorf("include cycle detected at %q", path)
	}
	visited[path] = true
	defer func() { visited[path] = false }()

	var out strings.Builder
	s := newScanner(path, content)
	cursor := 0
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokIdent && tok.text == "include" {
			// flush content before include stmt
			if tok.pos > cursor && tok.pos <= len(content) {
				out.WriteString(content[cursor:tok.pos])
			}
			strTok := s.nextNonTrivia()
			if strTok.kind != tokString {
				return "", s.errAt(strTok, "expected string literal after include")
			}
			semi := s.nextNonTrivia()
			if semi.kind != tokSemicolon {
				return "", s.errAt(semi, "expected ';' after include path")
			}
			cursor = semi.pos + len(semi.text)

			includePath := unquoteString(strTok.text)
			if includePath == "" {
				return "", s.errAt(strTok, "include path is empty")
			}
			full := includePath
			if !filepath.IsAbs(includePath) {
				full = filepath.Join(filepath.Dir(path), includePath)
			}
			// #nosec G304 -- include files are limited to the configured providers directory tree (provider files + relative includes).
			b, err := os.ReadFile(full)
			if err != nil {
				return "", fmt.Errorf("read include file %q (from %q): %w", full, path, err)
			}
			expanded, err := preprocessIncludesInner(full, string(b), visited, depth+1)
			if err != nil {
				return "", err
			}
			out.WriteString(expanded)
			out.WriteString("\n")
			continue
		}
	}
	if cursor < len(content) {
		out.WriteString(content[cursor:])
	}
	return out.String(), nil
}

func unquoteString(raw string) string {
	if len(raw) < 2 {
		return raw
	}
	if raw[0] != '"' || raw[len(raw)-1] != '"' {
		return raw
	}
	inner := raw[1 : len(raw)-1]
	inner = strings.ReplaceAll(inner, `\\`, `\`)
	inner = strings.ReplaceAll(inner, `\"`, `"`)
	inner = strings.ReplaceAll(inner, `\n`, "\n")
	inner = strings.ReplaceAll(inner, `\t`, "\t")
	return inner
}
