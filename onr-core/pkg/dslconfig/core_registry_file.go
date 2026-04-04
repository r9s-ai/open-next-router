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
	globalModes, globalModePaths, _, err := loadGlobalUsageModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	globalFinishReasonModes, globalFinishReasonModePaths, _, err := loadGlobalFinishReasonModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	globalModelsModes, globalModelsModePaths, _, err := loadGlobalModelsModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	globalBalanceModes, globalBalanceModePaths, _, err := loadGlobalBalanceModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	next, loaded, err := parseProvidersFromMergedFile(p, content, globalModes, globalModePaths, globalFinishReasonModes, globalFinishReasonModePaths, globalModelsModes, globalModelsModePaths, globalBalanceModes, globalBalanceModePaths)
	if err != nil {
		return LoadResult{}, err
	}

	r.mu.Lock()
	r.providers = next
	r.mu.Unlock()

	return LoadResult{LoadedProviders: loaded, SkippedFiles: nil, SkippedReasons: nil}, nil
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
	globalModes, globalModePaths, _, err := loadGlobalUsageModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	globalFinishReasonModes, globalFinishReasonModePaths, _, err := loadGlobalFinishReasonModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	globalModelsModes, globalModelsModePaths, _, err := loadGlobalModelsModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	globalBalanceModes, globalBalanceModePaths, _, err := loadGlobalBalanceModesFromFile(globalConfigPathForMergedProvidersFile(p))
	if err != nil {
		return LoadResult{}, err
	}
	_, loaded, err := parseProvidersFromMergedFile(p, content, globalModes, globalModePaths, globalFinishReasonModes, globalFinishReasonModePaths, globalModelsModes, globalModelsModePaths, globalBalanceModes, globalBalanceModePaths)
	if err != nil {
		return LoadResult{}, err
	}
	return LoadResult{LoadedProviders: loaded, SkippedFiles: nil, SkippedReasons: nil}, nil
}

func parseProvidersFromMergedFile(path string, content string, inheritedModes usageModeRegistry, inheritedModePaths map[string]string, inheritedFinishReasonModes finishReasonModeRegistry, inheritedFinishReasonModePaths map[string]string, inheritedModelsModes modelsModeRegistry, inheritedModelsModePaths map[string]string, inheritedBalanceModes balanceModeRegistry, inheritedBalanceModePaths map[string]string) (map[string]ProviderFile, []string, error) {
	rawModes, err := parseGlobalUsageModes(path, content)
	if err != nil {
		return nil, nil, err
	}
	rawFinishReasonModes, err := parseGlobalFinishReasonModes(path, content)
	if err != nil {
		return nil, nil, err
	}
	rawModelsModes, err := parseGlobalModelsModes(path, content)
	if err != nil {
		return nil, nil, err
	}
	rawBalanceModes, err := parseGlobalBalanceModes(path, content)
	if err != nil {
		return nil, nil, err
	}
	if inheritedModes == nil {
		inheritedModes = usageModeRegistry{}
	}
	if inheritedModePaths == nil {
		inheritedModePaths = map[string]string{}
	}
	if inheritedFinishReasonModes == nil {
		inheritedFinishReasonModes = finishReasonModeRegistry{}
	}
	if inheritedFinishReasonModePaths == nil {
		inheritedFinishReasonModePaths = map[string]string{}
	}
	if inheritedModelsModes == nil {
		inheritedModelsModes = modelsModeRegistry{}
	}
	if inheritedModelsModePaths == nil {
		inheritedModelsModePaths = map[string]string{}
	}
	if inheritedBalanceModes == nil {
		inheritedBalanceModes = balanceModeRegistry{}
	}
	if inheritedBalanceModePaths == nil {
		inheritedBalanceModePaths = map[string]string{}
	}
	mergedModes := usageModeRegistry{}
	mergedModePaths := map[string]string{}
	mergedFinishReasonModes := finishReasonModeRegistry{}
	mergedFinishReasonModePaths := map[string]string{}
	mergedModelsModes := modelsModeRegistry{}
	mergedModelsModePaths := map[string]string{}
	mergedBalanceModes := balanceModeRegistry{}
	mergedBalanceModePaths := map[string]string{}
	for name, cfg := range inheritedModes {
		mergedModes[name] = cfg
	}
	for name, modePath := range inheritedModePaths {
		mergedModePaths[name] = modePath
	}
	for name, cfg := range inheritedFinishReasonModes {
		mergedFinishReasonModes[name] = cfg
	}
	for name, modePath := range inheritedFinishReasonModePaths {
		mergedFinishReasonModePaths[name] = modePath
	}
	for name, cfg := range inheritedModelsModes {
		mergedModelsModes[name] = cfg
	}
	for name, modePath := range inheritedModelsModePaths {
		mergedModelsModePaths[name] = modePath
	}
	for name, cfg := range inheritedBalanceModes {
		mergedBalanceModes[name] = cfg
	}
	for name, modePath := range inheritedBalanceModePaths {
		mergedBalanceModePaths[name] = modePath
	}
	for name, cfg := range rawModes {
		if prev, ok := mergedModePaths[name]; ok {
			return nil, nil, fmt.Errorf("duplicate usage_mode %q in %q (already in %q)", name, path, prev)
		}
		mergedModes[name] = cfg
		mergedModePaths[name] = path
	}
	for name, cfg := range rawFinishReasonModes {
		if prev, ok := mergedFinishReasonModePaths[name]; ok {
			return nil, nil, fmt.Errorf("duplicate finish_reason_mode %q in %q (already in %q)", name, path, prev)
		}
		mergedFinishReasonModes[name] = cfg
		mergedFinishReasonModePaths[name] = path
	}
	for name, cfg := range rawModelsModes {
		if prev, ok := mergedModelsModePaths[name]; ok {
			return nil, nil, fmt.Errorf("duplicate models_mode %q in %q (already in %q)", name, path, prev)
		}
		mergedModelsModes[name] = cfg
		mergedModelsModePaths[name] = path
	}
	for name, cfg := range rawBalanceModes {
		if prev, ok := mergedBalanceModePaths[name]; ok {
			return nil, nil, fmt.Errorf("duplicate balance_mode %q in %q (already in %q)", name, path, prev)
		}
		mergedBalanceModes[name] = cfg
		mergedBalanceModePaths[name] = path
	}
	resolvedModes, err := resolveUsageModeRegistry(mergedModePaths, mergedModes)
	if err != nil {
		return nil, nil, err
	}
	resolvedFinishReasonModes, err := resolveFinishReasonModeRegistry(mergedFinishReasonModePaths, mergedFinishReasonModes)
	if err != nil {
		return nil, nil, err
	}
	resolvedModelsModes, err := resolveModelsModeRegistry(mergedModelsModePaths, mergedModelsModes)
	if err != nil {
		return nil, nil, err
	}
	resolvedBalanceModes, err := resolveBalanceModeRegistry(mergedBalanceModePaths, mergedBalanceModes)
	if err != nil {
		return nil, nil, err
	}
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
			routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderBody(s)
			if err != nil {
				return nil, nil, err
			}
			if err := validateProviderBaseURL(path, providerName, routing); err != nil {
				return nil, nil, err
			}
			if err := validateProviderMatchAPIs(path, providerName, routing); err != nil {
				return nil, nil, err
			}
			if err := validateProviderHeaders(path, providerName, headers); err != nil {
				return nil, nil, err
			}
			resolvedUsage, err := validateProviderUsage(path, providerName, usage, resolvedModes)
			if err != nil {
				return nil, nil, err
			}
			resolvedFinish, err := validateProviderFinishReason(path, providerName, finish, resolvedFinishReasonModes)
			if err != nil {
				return nil, nil, err
			}
			resolvedBalance, err := validateProviderBalance(path, providerName, balance, resolvedBalanceModes)
			if err != nil {
				return nil, nil, err
			}
			resolvedModels, err := validateProviderModels(path, providerName, models, resolvedModelsModes)
			if err != nil {
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
				Usage:    resolvedUsage,
				Finish:   resolvedFinish,
				Balance:  resolvedBalance,
				Models:   resolvedModels,
			}
			loaded = append(loaded, providerName)
		default:
			// ignore unknown top-level directives for forward compatibility
		}
	}

	sort.Strings(loaded)
	return next, loaded, nil
}
