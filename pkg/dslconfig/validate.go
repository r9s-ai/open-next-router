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
	routing, headers, req, response, perr, usage, finish, err := parseProviderConfig(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderBaseURL(p, providerName, routing); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderRequestTransform(p, providerName, req); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderUsage(p, providerName, usage); err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderFinishReason(p, providerName, finish); err != nil {
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
	}, nil
}

func validateProviderRequestTransform(path, providerName string, req ProviderRequestTransform) error {
	if err := validateRequestTransform(path, providerName, "defaults.request", req.Defaults); err != nil {
		return err
	}
	for i, m := range req.Matches {
		scope := fmt.Sprintf("match[%d].request", i)
		if err := validateRequestTransform(path, providerName, scope, m.Transform); err != nil {
			return err
		}
	}
	return nil
}

func validateRequestTransform(path, providerName, scope string, t RequestTransform) error {
	mode := strings.ToLower(strings.TrimSpace(t.ReqMapMode))
	if mode == "" {
		return nil
	}
	switch mode {
	case "openai_chat_to_openai_responses":
		return nil
	default:
		return fmt.Errorf("provider %q in %q: %s unsupported req_map mode %q", providerName, path, scope, t.ReqMapMode)
	}
}

func validateProviderUsage(path, providerName string, usage ProviderUsage) error {
	if err := validateUsageExtractConfig(path, providerName, "defaults.metrics", usage.Defaults); err != nil {
		return err
	}
	for i, m := range usage.Matches {
		scope := fmt.Sprintf("match[%d].metrics", i)
		if err := validateUsageExtractConfig(path, providerName, scope, m.Extract); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderFinishReason(path, providerName string, finish ProviderFinishReason) error {
	if err := validateFinishReasonExtractConfig(path, providerName, "defaults.metrics", finish.Defaults); err != nil {
		return err
	}
	for i, m := range finish.Matches {
		scope := fmt.Sprintf("match[%d].metrics", i)
		if err := validateFinishReasonExtractConfig(path, providerName, scope, m.Extract); err != nil {
			return err
		}
	}
	return nil
}

func validateFinishReasonExtractConfig(path, providerName, scope string, cfg FinishReasonExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	p := strings.TrimSpace(cfg.FinishReasonPath)

	if mode == "" && p == "" {
		return nil
	}
	switch mode {
	case "", "openai", "anthropic", "gemini":
		// ok
	case usageModeCustom:
		if p == "" {
			return fmt.Errorf("provider %q in %q: %s finish_reason_extract custom requires finish_reason_path", providerName, path, scope)
		}
	default:
		return fmt.Errorf("provider %q in %q: %s unsupported finish_reason_extract mode %q", providerName, path, scope, cfg.Mode)
	}
	if p != "" && !strings.HasPrefix(p, "$.") {
		return fmt.Errorf("provider %q in %q: %s finish_reason_path must start with $. ", providerName, path, scope)
	}
	return nil
}

func validateUsageExtractConfig(path, providerName, scope string, cfg UsageExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil
	}
	if mode != usageModeCustom {
		return nil
	}
	if cfg.InputTokensExpr == nil && strings.TrimSpace(cfg.InputTokensPath) == "" {
		return fmt.Errorf("provider %q in %q: %s requires input_tokens (expr) or input_tokens_path", providerName, path, scope)
	}
	if cfg.OutputTokensExpr == nil && strings.TrimSpace(cfg.OutputTokensPath) == "" {
		return fmt.Errorf("provider %q in %q: %s requires output_tokens (expr) or output_tokens_path", providerName, path, scope)
	}

	if cfg.InputTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.InputTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s input_tokens_path must start with $. ", providerName, path, scope)
	}
	if cfg.OutputTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.OutputTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s output_tokens_path must start with $. ", providerName, path, scope)
	}
	if cfg.CacheReadTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.CacheReadTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s cache_read_tokens_path must start with $. ", providerName, path, scope)
	}
	if cfg.CacheWriteTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.CacheWriteTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s cache_write_tokens_path must start with $. ", providerName, path, scope)
	}
	return nil
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
	}

	sort.Strings(loaded)
	return LoadResult{LoadedProviders: loaded}, nil
}
