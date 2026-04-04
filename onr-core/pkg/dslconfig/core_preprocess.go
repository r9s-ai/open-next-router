package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
			pathTok := s.nextNonTrivia()
			includePath, semi, err := parseIncludePath(s, content, pathTok)
			if err != nil {
				return "", err
			}
			cursor = semi.pos + len(semi.text)
			if includePath == "" {
				return "", s.errAt(pathTok, "include path is empty")
			}
			files, err := expandIncludeTargets(path, includePath)
			if err != nil {
				return "", s.errAt(pathTok, err.Error())
			}
			for _, full := range files {
				// #nosec G304 -- include files are limited to the configured DSL tree (provider files + relative includes).
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
			}
			continue
		}
	}
	if cursor < len(content) {
		out.WriteString(content[cursor:])
	}
	return out.String(), nil
}

func parseIncludePath(s *scanner, content string, first token) (string, token, error) {
	if first.kind == tokEOF {
		return "", token{}, s.errAt(first, "unexpected EOF after include")
	}
	if first.kind == tokSemicolon {
		return "", token{}, s.errAt(first, "include path is empty")
	}
	if first.kind == tokString {
		semi := s.nextNonTrivia()
		if semi.kind != tokSemicolon {
			return "", token{}, s.errAt(semi, "expected ';' after include path")
		}
		return strings.TrimSpace(unquoteString(first.text)), semi, nil
	}
	start := first.pos
	end := first.pos + len(first.text)
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return "", token{}, s.errAt(tok, "expected ';' after include path")
		}
		if tok.kind == tokSemicolon {
			return strings.TrimSpace(content[start:end]), tok, nil
		}
		end = tok.pos + len(tok.text)
	}
}

func expandIncludeTargets(basePath string, includePath string) ([]string, error) {
	full := strings.TrimSpace(includePath)
	if full == "" {
		return nil, fmt.Errorf("include path is empty")
	}
	if !filepath.IsAbs(full) {
		full = filepath.Join(filepath.Dir(basePath), full)
	}
	if hasGlobPattern(full) {
		matches, err := filepath.Glob(full)
		if err != nil {
			return nil, fmt.Errorf("invalid include glob %q", includePath)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("include glob %q matched no files", includePath)
		}
		files := make([]string, 0, len(matches))
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, fmt.Errorf("stat include target %q: %w", match, err)
			}
			if info.IsDir() {
				dirFiles, err := expandIncludeDir(match)
				if err != nil {
					return nil, err
				}
				files = append(files, dirFiles...)
				continue
			}
			files = append(files, match)
		}
		sort.Strings(files)
		return files, nil
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, fmt.Errorf("stat include target %q: %w", includePath, err)
	}
	if info.IsDir() {
		return expandIncludeDir(full)
	}
	return []string{full}, nil
}

func expandIncludeDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read include dir %q: %w", dir, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func hasGlobPattern(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func unquoteString(raw string) string {
	if len(raw) < 2 {
		return raw
	}
	quote := raw[0]
	if (quote != '"' && quote != '\'') || raw[len(raw)-1] != quote {
		return raw
	}
	inner := raw[1 : len(raw)-1]
	inner = strings.ReplaceAll(inner, `\\`, `\`)
	if quote == '"' {
		inner = strings.ReplaceAll(inner, `\"`, `"`)
	} else {
		inner = strings.ReplaceAll(inner, `\'`, `'`)
	}
	inner = strings.ReplaceAll(inner, `\n`, "\n")
	inner = strings.ReplaceAll(inner, `\t`, "\t")
	return inner
}
