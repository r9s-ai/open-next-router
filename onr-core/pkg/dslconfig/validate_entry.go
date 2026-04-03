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
	providerName, err := findProviderName(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	providerName = normalizeProviderName(providerName)
	expected := normalizeProviderName(strings.TrimSuffix(filepath.Base(p), ".conf"))
	if providerName != expected {
		return ProviderFile{}, fmt.Errorf(
			"provider file %q declares provider %q, expected %q",
			p, providerName, expected,
		)
	}
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderBaseURL(p, providerName, routing); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderMatchAPIs(p, providerName, routing); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderRequestTransform(p, providerName, req); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderHeaders(p, providerName, headers); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderResponse(p, providerName, response); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderUsage(p, providerName, usage); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderFinishReason(p, providerName, finish); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderBalance(p, providerName, balance); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderModels(p, providerName, models); err != nil {
		return ProviderFile{}, err
	}
	return ProviderFile{
		Name:     providerName,
		Path:     p,
		Content:  content,
		Routing:  routing,
		Headers:  headers,
		Request:  req,
		Response: response,
		Error:    perr,
		Usage:    usage,
		Finish:   finish,
		Balance:  balance,
		Models:   models,
	}, nil
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

	loaded := make([]string, 0)
	warnings := make([]ValidationWarning, 0)
	seen := map[string]string{} // provider -> file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		pf, err := ValidateProviderFile(path)
		if err != nil {
			return LoadResult{}, err
		}
		if prev, ok := seen[pf.Name]; ok {
			return LoadResult{}, fmt.Errorf("duplicate provider name %q in %q (already in %q)", pf.Name, path, prev)
		}
		seen[pf.Name] = path
		loaded = append(loaded, pf.Name)
		warnings = append(warnings, collectDeprecatedDirectiveWarnings(path, pf.Content)...)
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
