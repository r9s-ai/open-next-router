package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProviderBlock struct {
	Name    string
	Start   int
	End     int
	Content string
}

func ListProviderBlocks(path string, content string) ([]ProviderBlock, error) {
	s := newScanner(path, content)
	blocks := make([]ProviderBlock, 0)
	depth := 0
	for {
		tok := s.next()
		switch tok.kind {
		case tokEOF:
			return blocks, nil
		case tokLBrace:
			depth++
		case tokRBrace:
			if depth > 0 {
				depth--
			}
		case tokIdent:
			if depth != 0 || tok.text != "provider" {
				continue
			}
			block, err := scanProviderBlock(s, tok, content)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}
}

func ExtractProviderBlockOptional(path string, content string, providerName string) (string, bool, error) {
	want := NormalizeProviderName(providerName)
	blocks, err := ListProviderBlocks(path, content)
	if err != nil {
		return "", false, err
	}
	for _, block := range blocks {
		if block.Name == want {
			return block.Content, true, nil
		}
	}
	return "", false, nil
}

func UpsertProviderBlock(path string, content string, providerName string, providerBlock string) (string, error) {
	want := NormalizeProviderName(providerName)
	blockContent := strings.TrimSpace(providerBlock)
	if blockContent == "" {
		return "", fmt.Errorf("provider block is empty")
	}
	foundName, ok, err := FindProviderNameOptional(path, blockContent)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s: no provider block found", path)
	}
	if NormalizeProviderName(foundName) != want {
		return "", fmt.Errorf("provider block name %q does not match target %q", foundName, providerName)
	}
	blocks, err := ListProviderBlocks(path, content)
	if err != nil {
		return "", err
	}
	normalizedBlock := blockContent + "\n"
	for _, block := range blocks {
		if block.Name != want {
			continue
		}
		return content[:block.Start] + normalizedBlock + content[block.End:], nil
	}
	base := strings.TrimRight(content, " \t\r\n")
	if base == "" {
		return normalizedBlock, nil
	}
	return base + "\n\n" + normalizedBlock, nil
}

func ListIncludedFiles(path string, content string) ([]string, error) {
	visited := map[string]bool{}
	collected := map[string]struct{}{}
	if err := listIncludedFilesInner(path, content, visited, collected, 0); err != nil {
		return nil, err
	}
	files := make([]string, 0, len(collected))
	for file := range collected {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}

func scanProviderBlock(s *scanner, providerTok token, content string) (ProviderBlock, error) {
	nameTok := s.nextNonTrivia()
	if nameTok.kind != tokString {
		return ProviderBlock{}, s.errAt(nameTok, "expected string literal after provider")
	}
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return ProviderBlock{}, s.errAt(lb, "expected '{' after provider name")
	}
	depth := 1
	end := lb.pos + len(lb.text)
	for depth > 0 {
		tok := s.next()
		if tok.kind == tokEOF {
			return ProviderBlock{}, s.errAt(tok, "unexpected EOF in provider block")
		}
		switch tok.kind {
		case tokLBrace:
			depth++
		case tokRBrace:
			depth--
			if depth == 0 {
				end = tok.pos + len(tok.text)
			}
		}
	}
	name := NormalizeProviderName(unquoteString(nameTok.text))
	return ProviderBlock{
		Name:    name,
		Start:   providerTok.pos,
		End:     end,
		Content: content[providerTok.pos:end],
	}, nil
}

func listIncludedFilesInner(path string, content string, visited map[string]bool, collected map[string]struct{}, depth int) error {
	if depth > maxIncludeDepth {
		return fmt.Errorf("include depth exceeded (%d) at %q", maxIncludeDepth, path)
	}
	if visited[path] {
		return nil
	}
	visited[path] = true
	defer func() { visited[path] = false }()

	s := newScanner(path, content)
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return nil
		}
		if tok.kind != tokIdent || tok.text != "include" {
			continue
		}
		pathTok := s.nextNonTrivia()
		includePath, _, err := parseIncludePath(s, content, pathTok)
		if err != nil {
			return err
		}
		files, err := expandIncludeTargets(path, includePath)
		if err != nil {
			return s.errAt(pathTok, err.Error())
		}
		for _, full := range files {
			collected[full] = struct{}{}
			// #nosec G304 -- include files are resolved from explicit DSL include targets.
			b, err := os.ReadFile(full)
			if err != nil {
				return fmt.Errorf("read include file %q (from %q): %w", full, path, err)
			}
			if err := listIncludedFilesInner(full, string(b), visited, collected, depth+1); err != nil {
				return err
			}
		}
	}
}

func ProviderBlockFromFileContent(path string, content string) (ProviderBlock, error) {
	blocks, err := ListProviderBlocks(path, content)
	if err != nil {
		return ProviderBlock{}, err
	}
	if len(blocks) == 0 {
		return ProviderBlock{}, fmt.Errorf("%s: no provider block found", path)
	}
	if len(blocks) > 1 {
		return ProviderBlock{}, fmt.Errorf("%s: expected exactly one provider block, got %d", filepath.Base(path), len(blocks))
	}
	return blocks[0], nil
}
