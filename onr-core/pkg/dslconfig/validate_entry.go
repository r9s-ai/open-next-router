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
	rawModes, modePaths, _, err := loadGlobalUsageModesFromFile(globalConfigPathForProviderFile(p))
	if err != nil {
		return ProviderFile{}, err
	}
	rawFinishReasonModes, finishReasonModePaths, _, err := loadGlobalFinishReasonModesFromFile(globalConfigPathForProviderFile(p))
	if err != nil {
		return ProviderFile{}, err
	}
	rawModelsModes, modelsModePaths, _, err := loadGlobalModelsModesFromFile(globalConfigPathForProviderFile(p))
	if err != nil {
		return ProviderFile{}, err
	}
	rawBalanceModes, balanceModePaths, _, err := loadGlobalBalanceModesFromFile(globalConfigPathForProviderFile(p))
	if err != nil {
		return ProviderFile{}, err
	}
	localModes, err := parseGlobalUsageModes(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	localFinishReasonModes, err := parseGlobalFinishReasonModes(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	localModelsModes, err := parseGlobalModelsModes(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	localBalanceModes, err := parseGlobalBalanceModes(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	if rawModes == nil {
		rawModes = usageModeRegistry{}
	}
	if modePaths == nil {
		modePaths = map[string]string{}
	}
	if rawModelsModes == nil {
		rawModelsModes = modelsModeRegistry{}
	}
	if rawFinishReasonModes == nil {
		rawFinishReasonModes = finishReasonModeRegistry{}
	}
	if modelsModePaths == nil {
		modelsModePaths = map[string]string{}
	}
	if finishReasonModePaths == nil {
		finishReasonModePaths = map[string]string{}
	}
	if rawBalanceModes == nil {
		rawBalanceModes = balanceModeRegistry{}
	}
	if balanceModePaths == nil {
		balanceModePaths = map[string]string{}
	}
	for name, cfg := range localModes {
		if prev, ok := modePaths[name]; ok {
			return ProviderFile{}, fmt.Errorf("duplicate usage_mode %q in %q (already in %q)", name, p, prev)
		}
		rawModes[name] = cfg
		modePaths[name] = p
	}
	for name, cfg := range localFinishReasonModes {
		if prev, ok := finishReasonModePaths[name]; ok {
			return ProviderFile{}, fmt.Errorf("duplicate finish_reason_mode %q in %q (already in %q)", name, p, prev)
		}
		rawFinishReasonModes[name] = cfg
		finishReasonModePaths[name] = p
	}
	for name, cfg := range localModelsModes {
		if prev, ok := modelsModePaths[name]; ok {
			return ProviderFile{}, fmt.Errorf("duplicate models_mode %q in %q (already in %q)", name, p, prev)
		}
		rawModelsModes[name] = cfg
		modelsModePaths[name] = p
	}
	for name, cfg := range localBalanceModes {
		if prev, ok := balanceModePaths[name]; ok {
			return ProviderFile{}, fmt.Errorf("duplicate balance_mode %q in %q (already in %q)", name, p, prev)
		}
		rawBalanceModes[name] = cfg
		balanceModePaths[name] = p
	}
	resolvedModes, err := resolveUsageModeRegistry(modePaths, rawModes)
	if err != nil {
		return ProviderFile{}, err
	}
	resolvedFinishReasonModes, err := resolveFinishReasonModeRegistry(finishReasonModePaths, rawFinishReasonModes)
	if err != nil {
		return ProviderFile{}, err
	}
	resolvedModelsModes, err := resolveModelsModeRegistry(modelsModePaths, rawModelsModes)
	if err != nil {
		return ProviderFile{}, err
	}
	resolvedBalanceModes, err := resolveBalanceModeRegistry(balanceModePaths, rawBalanceModes)
	if err != nil {
		return ProviderFile{}, err
	}
	pf, hasProvider, err := validateAndBuildProviderFile(p, content, resolvedModes, resolvedFinishReasonModes, resolvedModelsModes, resolvedBalanceModes)
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
	rawModes, modePaths, globalContent, err := loadGlobalUsageModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	rawFinishReasonModes, finishReasonModePaths, _, err := loadGlobalFinishReasonModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	rawModelsModes, modelsModePaths, _, err := loadGlobalModelsModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	rawBalanceModes, balanceModePaths, _, err := loadGlobalBalanceModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	if rawModes == nil {
		rawModes = usageModeRegistry{}
	}
	if modePaths == nil {
		modePaths = map[string]string{}
	}
	if rawModelsModes == nil {
		rawModelsModes = modelsModeRegistry{}
	}
	if rawFinishReasonModes == nil {
		rawFinishReasonModes = finishReasonModeRegistry{}
	}
	if modelsModePaths == nil {
		modelsModePaths = map[string]string{}
	}
	if finishReasonModePaths == nil {
		finishReasonModePaths = map[string]string{}
	}
	if rawBalanceModes == nil {
		rawBalanceModes = balanceModeRegistry{}
	}
	if balanceModePaths == nil {
		balanceModePaths = map[string]string{}
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
		defs, err := parseGlobalUsageModes(path, content)
		if err != nil {
			return LoadResult{}, err
		}
		finishDefs, err := parseGlobalFinishReasonModes(path, content)
		if err != nil {
			return LoadResult{}, err
		}
		modelDefs, err := parseGlobalModelsModes(path, content)
		if err != nil {
			return LoadResult{}, err
		}
		balanceDefs, err := parseGlobalBalanceModes(path, content)
		if err != nil {
			return LoadResult{}, err
		}
		for name, cfg := range defs {
			if prev, ok := modePaths[name]; ok {
				return LoadResult{}, fmt.Errorf("duplicate usage_mode %q in %q (already in %q)", name, path, prev)
			}
			modePaths[name] = path
			rawModes[name] = cfg
		}
		for name, cfg := range finishDefs {
			if prev, ok := finishReasonModePaths[name]; ok {
				return LoadResult{}, fmt.Errorf("duplicate finish_reason_mode %q in %q (already in %q)", name, path, prev)
			}
			finishReasonModePaths[name] = path
			rawFinishReasonModes[name] = cfg
		}
		for name, cfg := range modelDefs {
			if prev, ok := modelsModePaths[name]; ok {
				return LoadResult{}, fmt.Errorf("duplicate models_mode %q in %q (already in %q)", name, path, prev)
			}
			modelsModePaths[name] = path
			rawModelsModes[name] = cfg
		}
		for name, cfg := range balanceDefs {
			if prev, ok := balanceModePaths[name]; ok {
				return LoadResult{}, fmt.Errorf("duplicate balance_mode %q in %q (already in %q)", name, path, prev)
			}
			balanceModePaths[name] = path
			rawBalanceModes[name] = cfg
		}
		candidates = append(candidates, candidate{path: path, content: content})
		warnings = append(warnings, collectDeprecatedDirectiveWarnings(path, content)...)
	}
	resolvedModes, err := resolveUsageModeRegistry(modePaths, rawModes)
	if err != nil {
		return LoadResult{}, err
	}
	resolvedFinishReasonModes, err := resolveFinishReasonModeRegistry(finishReasonModePaths, rawFinishReasonModes)
	if err != nil {
		return LoadResult{}, err
	}
	resolvedModelsModes, err := resolveModelsModeRegistry(modelsModePaths, rawModelsModes)
	if err != nil {
		return LoadResult{}, err
	}
	resolvedBalanceModes, err := resolveBalanceModeRegistry(balanceModePaths, rawBalanceModes)
	if err != nil {
		return LoadResult{}, err
	}
	loaded := make([]string, 0)
	seen := map[string]string{}
	for _, candidate := range candidates {
		pf, hasProvider, err := validateAndBuildProviderFile(candidate.path, candidate.content, resolvedModes, resolvedFinishReasonModes, resolvedModelsModes, resolvedBalanceModes)
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
