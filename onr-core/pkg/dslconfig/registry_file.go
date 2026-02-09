package dslconfig

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// ReloadFromFile loads multiple providers from a single merged config file (e.g. providers.conf).
//
// Rules:
// - Each provider name must match the provider name pattern.
// - Provider names must be unique within the file (duplicates are an error).
// - Each provider must pass the same validations as single-file providers:
//   - upstream_config.base_url is required and must be a string literal absolute URL
//   - usage_extract / finish_reason_extract configs are validated
func (r *Registry) ReloadFromFile(path string) (LoadResult, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return LoadResult{}, fmt.Errorf("providers file path is empty")
	}
	// #nosec G304 -- provider files are loaded from a configured path by design.
	b, err := os.ReadFile(p)
	if err != nil {
		return LoadResult{}, fmt.Errorf("read providers file %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(b))
	if err != nil {
		return LoadResult{}, err
	}

	next, loaded, err := parseProvidersFromMergedFile(p, content)
	if err != nil {
		return LoadResult{}, err
	}

	r.mu.Lock()
	r.providers = next
	r.mu.Unlock()

	return LoadResult{LoadedProviders: loaded, SkippedFiles: nil}, nil
}

// ValidateProvidersFile validates a merged providers config file (providers.conf).
// It returns the loaded provider names if validation succeeds.
func ValidateProvidersFile(path string) (LoadResult, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return LoadResult{}, fmt.Errorf("providers file path is empty")
	}
	// #nosec G304 -- validation reads a user-specified path by design.
	b, err := os.ReadFile(p)
	if err != nil {
		return LoadResult{}, fmt.Errorf("read providers file %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(b))
	if err != nil {
		return LoadResult{}, err
	}
	_, loaded, err := parseProvidersFromMergedFile(p, content)
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{LoadedProviders: loaded, SkippedFiles: nil}, nil
}

func parseProvidersFromMergedFile(path string, content string) (map[string]ProviderFile, []string, error) {
	s := newScanner(path, content)
	next := map[string]ProviderFile{}
	loaded := make([]string, 0)
	syntaxVersion := ""

	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind != tokIdent {
			continue
		}
		switch tok.text {
		case "syntax":
			vTok := s.nextNonTrivia()
			if vTok.kind != tokString {
				return nil, nil, s.errAt(vTok, "expected syntax version string literal")
			}
			semi := s.nextNonTrivia()
			if semi.kind != tokSemicolon {
				return nil, nil, s.errAt(semi, "expected ';' after syntax")
			}
			v := strings.TrimSpace(unquoteString(vTok.text))
			if v == "" {
				return nil, nil, s.errAt(vTok, "syntax version is empty")
			}
			if syntaxVersion == "" {
				syntaxVersion = v
			} else if syntaxVersion != v {
				return nil, nil, fmt.Errorf("syntax version mismatch in %q: %q vs %q", path, syntaxVersion, v)
			}
		case "provider":
			nameTok := s.nextNonTrivia()
			if nameTok.kind != tokString {
				return nil, nil, s.errAt(nameTok, "expected provider name string literal")
			}
			providerName := normalizeProviderName(unquoteString(nameTok.text))
			if err := validateProviderName(providerName); err != nil {
				return nil, nil, fmt.Errorf("provider %q in %q: %w", providerName, path, err)
			}
			if _, exists := next[providerName]; exists {
				return nil, nil, fmt.Errorf("duplicate provider %q in %q", providerName, path)
			}
			lb := s.nextNonTrivia()
			if lb.kind != tokLBrace {
				return nil, nil, s.errAt(lb, "expected '{' after provider name")
			}
			routing, headers, req, response, perr, usage, finish, balance, err := parseProviderBody(s)
			if err != nil {
				return nil, nil, err
			}
			if err := validateProviderBaseURL(path, providerName, routing); err != nil {
				return nil, nil, err
			}
			if err := validateProviderHeaders(path, providerName, headers); err != nil {
				return nil, nil, err
			}
			if err := validateProviderUsage(path, providerName, usage); err != nil {
				return nil, nil, err
			}
			if err := validateProviderFinishReason(path, providerName, finish); err != nil {
				return nil, nil, err
			}
			if err := validateProviderBalance(path, providerName, balance); err != nil {
				return nil, nil, err
			}

			next[providerName] = ProviderFile{
				Name:     providerName,
				Path:     path,
				Content:  "", // merged file; avoid duplicating large content per provider
				Routing:  routing,
				Headers:  headers,
				Request:  req,
				Response: response,
				Error:    perr,
				Usage:    usage,
				Finish:   finish,
				Balance:  balance,
			}
			loaded = append(loaded, providerName)
		default:
			// ignore unknown top-level directives for forward compatibility
		}
	}

	sort.Strings(loaded)
	return next, loaded, nil
}
