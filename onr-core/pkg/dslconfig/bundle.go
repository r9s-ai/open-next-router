package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BundleProvidersPath expands a provider DSL source into a single merged file body.
//
// Supported inputs:
// - providers directory
// - merged/root DSL file such as onr.conf or providers.conf
func BundleProvidersPath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", fmt.Errorf("providers path is empty")
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return bundleProvidersDir(p)
	}
	return bundleProvidersFile(p)
}

func bundleProvidersFile(path string) (string, error) {
	// #nosec G304 -- bundle reads a user-specified DSL path by design.
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read providers file %q: %w", path, err)
	}
	content, err := preprocessIncludes(path, string(body))
	if err != nil {
		return "", err
	}
	return normalizeBundledContent(content), nil
}

func bundleProvidersDir(dir string) (string, error) {
	globalContent, includedProviderFiles, err := bundleGlobalConfigForProvidersDir(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read providers dir %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != providerConfExt {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	parts := make([]string, 0, len(names)+1)
	if strings.TrimSpace(globalContent) != "" {
		parts = append(parts, globalContent)
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		content, err := bundleProviderFileForDir(path, includedProviderFiles)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(content) != "" {
			parts = append(parts, content)
		}
	}
	return joinBundleParts(parts), nil
}

func bundleGlobalConfigForProvidersDir(path string) (string, map[string]struct{}, error) {
	includedProviderFiles := map[string]struct{}{}
	p := strings.TrimSpace(path)
	if p == "" {
		return "", includedProviderFiles, nil
	}
	// #nosec G304 -- bundle reads the conventional sibling global DSL file when present.
	body, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", includedProviderFiles, nil
		}
		return "", nil, fmt.Errorf("read global config %q: %w", p, err)
	}
	content, err := preprocessIncludesWithoutTopLevelProviders(p, string(body), includedProviderFiles)
	if err != nil {
		return "", nil, err
	}
	return normalizeBundledContent(content), includedProviderFiles, nil
}

func bundleProviderFileForDir(path string, includedProviderFiles map[string]struct{}) (string, error) {
	// #nosec G304 -- provider files are loaded from an explicit providers directory.
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read provider file %q: %w", path, err)
	}
	rawContent := string(body)
	expandedContent, err := preprocessIncludes(path, rawContent)
	if err != nil {
		return "", err
	}
	if _, ok := includedProviderFiles[filepath.Clean(path)]; !ok {
		return normalizeBundledContent(expandedContent), nil
	}
	content, err := extractDeclaredProviderBlocks(path, rawContent, expandedContent)
	if err != nil {
		return "", err
	}
	return normalizeBundledContent(content), nil
}

func preprocessIncludesWithoutTopLevelProviders(path string, content string, includedProviderFiles map[string]struct{}) (string, error) {
	visited := map[string]bool{}
	return preprocessIncludesWithoutTopLevelProvidersInner(path, content, visited, includedProviderFiles, 0)
}

func preprocessIncludesWithoutTopLevelProvidersInner(path string, content string, visited map[string]bool, includedProviderFiles map[string]struct{}, depth int) (string, error) {
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
			if tok.pos > cursor && tok.pos <= len(content) {
				out.WriteString(content[cursor:tok.pos])
			}
			pathTok := s.nextNonTrivia()
			includePath, semi, err := parseIncludePath(s, content, pathTok)
			if err != nil {
				return "", err
			}
			cursor = semi.pos + len(semi.text)
			files, err := expandIncludeTargets(path, includePath)
			if err != nil {
				return "", s.errAt(pathTok, err.Error())
			}
			for _, full := range files {
				// #nosec G304 -- include files are resolved from explicit DSL include targets.
				b, err := os.ReadFile(full)
				if err != nil {
					return "", fmt.Errorf("read include file %q (from %q): %w", full, path, err)
				}
				if hasProviderBlocks, err := fileHasProviderBlocks(full, string(b)); err != nil {
					return "", err
				} else if hasProviderBlocks {
					includedProviderFiles[filepath.Clean(full)] = struct{}{}
				}
				expanded, err := preprocessIncludesWithoutTopLevelProvidersInner(full, string(b), visited, includedProviderFiles, depth+1)
				if err != nil {
					return "", err
				}
				filtered, err := removeTopLevelProviderBlocks(full, expanded)
				if err != nil {
					return "", err
				}
				if strings.TrimSpace(filtered) == "" {
					continue
				}
				out.WriteString(filtered)
				out.WriteString("\n")
			}
			continue
		}
	}
	if cursor < len(content) {
		out.WriteString(content[cursor:])
	}
	return removeTopLevelProviderBlocks(path, out.String())
}

func fileHasProviderBlocks(path string, content string) (bool, error) {
	blocks, err := ListProviderBlocks(path, content)
	if err != nil {
		return false, err
	}
	return len(blocks) > 0, nil
}

func removeTopLevelProviderBlocks(path string, content string) (string, error) {
	blocks, err := ListProviderBlocks(path, content)
	if err != nil {
		return "", err
	}
	if len(blocks) == 0 {
		return content, nil
	}
	var out strings.Builder
	cursor := 0
	for _, block := range blocks {
		if block.Start > cursor {
			out.WriteString(content[cursor:block.Start])
		}
		cursor = block.End
	}
	if cursor < len(content) {
		out.WriteString(content[cursor:])
	}
	return out.String(), nil
}

func extractDeclaredProviderBlocks(path string, rawContent string, expandedContent string) (string, error) {
	declaredBlocks, err := ListProviderBlocks(path, rawContent)
	if err != nil {
		return "", err
	}
	if len(declaredBlocks) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(declaredBlocks))
	for _, block := range declaredBlocks {
		providerBlock, ok, err := ExtractProviderBlockOptional(path, expandedContent, block.Name)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("provider %q declared in %q not found after include expansion", block.Name, path)
		}
		parts = append(parts, strings.TrimSpace(providerBlock))
	}
	return joinBundleParts(parts), nil
}

func joinBundleParts(parts []string) string {
	var out strings.Builder
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(strings.TrimSpace(part))
	}
	if out.Len() == 0 {
		return ""
	}
	out.WriteString("\n")
	return out.String()
}

func normalizeBundledContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}
