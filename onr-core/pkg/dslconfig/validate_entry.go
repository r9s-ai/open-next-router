package dslconfig

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func validateProviderBaseURL(path, providerName string, routing ProviderRouting) error {
	raw := strings.TrimSpace(routing.BaseURLExpr)
	if raw == "" {
		return fmt.Errorf("provider %q in %q: upstream_config.base_url is required", providerName, path)
	}
	// v0.1: base_url must be a string literal to avoid expression complexity and ambiguity.
	if raw == exprChannelBaseURL {
		return fmt.Errorf("provider %q in %q: upstream_config.base_url must be a string literal, got %q", providerName, path, raw)
	}
	if !strings.HasPrefix(raw, "\"") || !strings.HasSuffix(raw, "\"") {
		return fmt.Errorf("provider %q in %q: upstream_config.base_url must be a string literal, got %q", providerName, path, raw)
	}
	v := strings.TrimSpace(unquoteString(raw))
	if v == "" {
		return fmt.Errorf("provider %q in %q: upstream_config.base_url must be non-empty", providerName, path)
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("provider %q in %q: upstream_config.base_url must be an absolute URL, got %q", providerName, path, v)
	}
	return nil
}

// ValidateProviderFile validates a single provider config file.
// It expands includes, validates the declared provider name matches the filename,
// and parses supported blocks (routing + headers).
func ValidateProviderFile(path string) (ProviderFile, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return ProviderFile{}, fmt.Errorf("provider file path is empty")
	}
	if filepath.Ext(p) != ".conf" {
		return ProviderFile{}, fmt.Errorf("provider file must have .conf extension: %s", p)
	}
	// #nosec G304 -- validation reads a user-specified path by design.
	contentBytes, err := os.ReadFile(p)
	if err != nil {
		return ProviderFile{}, fmt.Errorf("read provider file %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(contentBytes))
	if err != nil {
		return ProviderFile{}, err
	}
	modeState, _, err := loadGlobalModeRegistryState(globalConfigPathForProviderFile(p))
	if err != nil {
		return ProviderFile{}, err
	}
	localModeState, err := parseLocalModeRegistryState(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	if err := modeState.merge(p, localModeState); err != nil {
		return ProviderFile{}, err
	}
	resolvedState, err := modeState.resolve()
	if err != nil {
		return ProviderFile{}, err
	}
	pf, hasProvider, err := validateAndBuildProviderFile(p, content, resolvedState.usage, resolvedState.finishReason, resolvedState.models, resolvedState.balance)
	if err != nil {
		return ProviderFile{}, err
	}
	if !hasProvider {
		return ProviderFile{}, fmt.Errorf("%s: no provider block found", p)
	}
	return pf, nil
}

// ValidateProvidersDir validates all *.conf files in a directory.
// Unlike ReloadFromDir (runtime), validation is strict: any error fails the whole validation.
func ValidateProvidersDir(providersDir string) (LoadResult, error) {
	dir := strings.TrimSpace(providersDir)
	if dir == "" {
		return LoadResult{}, fmt.Errorf("providers dir is empty")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return LoadResult{}, fmt.Errorf("read providers dir %q: %w", dir, err)
	}

	type candidate struct {
		path    string
		content string
	}
	candidates := make([]candidate, 0)
	warnings := make([]ValidationWarning, 0)
	modeState, globalContent, err := loadGlobalModeRegistryState(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	if strings.TrimSpace(globalContent) != "" {
		warnings = append(warnings, collectDeprecatedDirectiveWarnings(globalConfigPathForProvidersDir(dir), globalContent)...)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return LoadResult{}, fmt.Errorf("read provider file %q: %w", path, err)
		}
		content, err := preprocessIncludes(path, string(contentBytes))
		if err != nil {
			return LoadResult{}, err
		}
		localModeState, err := parseLocalModeRegistryState(path, content)
		if err != nil {
			return LoadResult{}, err
		}
		if err := modeState.merge(path, localModeState); err != nil {
			return LoadResult{}, err
		}
		candidates = append(candidates, candidate{path: path, content: content})
		warnings = append(warnings, collectDeprecatedDirectiveWarnings(path, content)...)
	}
	resolvedState, err := modeState.resolve()
	if err != nil {
		return LoadResult{}, err
	}
	loaded := make([]string, 0)
	seen := map[string]string{}
	for _, candidate := range candidates {
		pf, hasProvider, err := validateAndBuildProviderFile(candidate.path, candidate.content, resolvedState.usage, resolvedState.finishReason, resolvedState.models, resolvedState.balance)
		if err != nil {
			return LoadResult{}, err
		}
		if !hasProvider {
			continue
		}
		if prev, ok := seen[pf.Name]; ok {
			return LoadResult{}, fmt.Errorf("duplicate provider name %q in %q (already in %q)", pf.Name, candidate.path, prev)
		}
		seen[pf.Name] = candidate.path
		loaded = append(loaded, pf.Name)
	}

	sort.Strings(loaded)
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].File != warnings[j].File {
			return warnings[i].File < warnings[j].File
		}
		if warnings[i].Line != warnings[j].Line {
			return warnings[i].Line < warnings[j].Line
		}
		if warnings[i].Column != warnings[j].Column {
			return warnings[i].Column < warnings[j].Column
		}
		return warnings[i].Directive < warnings[j].Directive
	})
	return LoadResult{LoadedProviders: loaded, Warnings: warnings}, nil
}

func ValidateProvidersPath(path string) (LoadResult, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return LoadResult{}, fmt.Errorf("providers path is empty")
	}
	info, err := os.Stat(p)
	if err != nil {
		return LoadResult{}, err
	}
	if info.IsDir() {
		return ValidateProvidersDir(p)
	}
	return ValidateProvidersFile(p)
}
