package dslconfig

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type usageModeRegistry map[string]UsageExtractConfig

func parseGlobalUsageModes(path string, content string) (usageModeRegistry, error) {
	s := newScanner(path, content)
	modes := usageModeRegistry{}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return modes, nil
		case tokIdent:
			switch tok.text {
			case "usage_mode":
				name, cfg, err := parseUsageModeBlock(s)
				if err != nil {
					return nil, err
				}
				if _, exists := modes[name]; exists {
					return nil, fmt.Errorf("duplicate usage_mode %q in %q", name, path)
				}
				modes[name] = prepareUsageExtractConfig(cfg)
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return nil, err
				}
			}
		}
	}
}

func parseUsageModeBlock(s *scanner) (string, UsageExtractConfig, error) {
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
	default:
		return "", UsageExtractConfig{}, s.errAt(nameTok, "usage_mode expects mode name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	name = normalizeUsageModeName(name)
	if name == "" {
		return "", UsageExtractConfig{}, s.errAt(nameTok, "usage_mode name is empty")
	}
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return "", UsageExtractConfig{}, s.errAt(lb, "expected '{' after usage_mode name")
	}
	var cfg UsageExtractConfig
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return "", UsageExtractConfig{}, s.errAt(tok, "unexpected EOF in usage_mode block")
		case tokRBrace:
			return name, prepareUsageExtractConfig(cfg), nil
		case tokIdent:
			switch tok.text {
			case "usage_extract":
				if err := parseUsageExtractStmt(s, &cfg); err != nil {
					return "", UsageExtractConfig{}, err
				}
			case "usage_fact":
				if err := parseUsageFactStmt(s, &cfg); err != nil {
					return "", UsageExtractConfig{}, err
				}
			case "input_tokens_expr", "output_tokens_expr", "cache_read_tokens_expr", "cache_write_tokens_expr", "total_tokens_expr":
				if err := parseUsageExtractAssignStmt(s, &cfg, tok.text); err != nil {
					return "", UsageExtractConfig{}, err
				}
			case "input_tokens_path", "output_tokens_path", "cache_read_tokens_path", "cache_write_tokens_path":
				if err := parseUsageExtractFieldStmt(s, &cfg, tok.text); err != nil {
					return "", UsageExtractConfig{}, err
				}
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return "", UsageExtractConfig{}, err
				}
			}
		}
	}
}

func normalizeUsageModeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func resolveUsageModeRegistry(pathByMode map[string]string, raw usageModeRegistry) (usageModeRegistry, error) {
	merged := usageModeRegistry{}
	paths := map[string]string{}
	for name, cfg := range raw {
		merged[name] = cfg
	}
	for name, path := range pathByMode {
		paths[name] = path
	}
	for name, cfg := range merged {
		if strings.TrimSpace(cfg.Mode) == "" && hasAnyUsageExtractionRule(cfg) {
			cfg.Mode = usageModeCustom
			merged[name] = cfg
		}
	}
	resolved := usageModeRegistry{}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cfg, err := resolveUsageExtractConfig(paths[name], "", fmt.Sprintf("usage_mode %q", name), merged[name], merged, nil)
		if err != nil {
			return nil, err
		}
		if err := validateResolvedUsageExtractConfig(paths[name], "", fmt.Sprintf("usage_mode %q", name), cfg); err != nil {
			return nil, err
		}
		resolved[name] = cfg
	}
	return resolved, nil
}

func resolveUsageExtractConfig(path, providerName, scope string, cfg UsageExtractConfig, registry usageModeRegistry, stack []string) (UsageExtractConfig, error) {
	mode := normalizeUsageMode(cfg.Mode)
	switch mode {
	case "":
		if hasAnyUsageExtractionRule(cfg) {
			cfg.Mode = usageModeCustom
		}
		return prepareUsageExtractConfig(cfg), nil
	case usageModeCustom:
		return prepareUsageExtractConfig(cfg), nil
	}
	if baseCfg, ok := registry[mode]; ok {
		for _, item := range stack {
			if item == mode {
				return UsageExtractConfig{}, fmt.Errorf("%s in %q: recursive usage_mode reference %q", scope, path, mode)
			}
		}
		base, err := resolveUsageExtractConfig(path, providerName, fmt.Sprintf("usage_mode %q", mode), baseCfg, registry, append(stack, mode))
		if err != nil {
			return UsageExtractConfig{}, err
		}
		if usageBuiltinPreset(base) == mode {
			base.Mode = mode
		}
		override := cfg
		override.Mode = ""
		return mergeUsageConfig(base, override), nil
	}
	return UsageExtractConfig{}, validationIssue(
		fmt.Errorf("provider %q in %q: %s unsupported usage_extract mode %q", providerName, path, scope, cfg.Mode),
		scope,
		"usage_extract",
	)
}

func findProviderNameOptional(path string, content string) (string, bool, error) {
	s := newScanner(path, content)
	for {
		tok := s.nextNonTrivia()
		if tok.kind == tokEOF {
			return "", false, nil
		}
		if tok.kind != tokIdent || tok.text != "provider" {
			continue
		}
		nameTok := s.nextNonTrivia()
		if nameTok.kind != tokString {
			return "", false, s.errAt(nameTok, "expected string literal after provider")
		}
		lb := s.nextNonTrivia()
		if lb.kind != tokLBrace {
			return "", false, s.errAt(lb, "expected '{' after provider name")
		}
		name := strings.TrimSpace(unquoteString(nameTok.text))
		if name == "" {
			return "", false, s.errAt(nameTok, "provider name is empty")
		}
		return name, true, nil
	}
}

func FindProviderNameOptional(path string, content string) (string, bool, error) {
	return findProviderNameOptional(path, content)
}

func validateAndBuildProviderFile(path string, content string, usageModes usageModeRegistry, finishReasonModes finishReasonModeRegistry, modelsModes modelsModeRegistry, balanceModes balanceModeRegistry) (ProviderFile, bool, error) {
	providerName, hasProvider, err := findProviderNameOptional(path, content)
	if err != nil {
		return ProviderFile{}, false, err
	}
	if !hasProvider {
		return ProviderFile{Path: path, Content: content}, false, nil
	}
	providerName = normalizeProviderName(providerName)
	expected := normalizeProviderName(strings.TrimSuffix(filepath.Base(path), ".conf"))
	if providerName != expected {
		return ProviderFile{}, false, fmt.Errorf(
			"provider file %q declares provider %q, expected %q",
			path, providerName, expected,
		)
	}
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig(path, content)
	if err != nil {
		return ProviderFile{}, false, err
	}
	if err := validateProviderBaseURL(path, providerName, routing); err != nil {
		return ProviderFile{}, false, err
	}
	if err := validateProviderMatchAPIs(path, providerName, routing); err != nil {
		return ProviderFile{}, false, err
	}
	if err := validateProviderRequestTransform(path, providerName, req); err != nil {
		return ProviderFile{}, false, err
	}
	if err := validateProviderHeaders(path, providerName, headers); err != nil {
		return ProviderFile{}, false, err
	}
	if err := validateProviderResponse(path, providerName, response); err != nil {
		return ProviderFile{}, false, err
	}
	resolvedUsage, err := validateProviderUsage(path, providerName, usage, usageModes)
	if err != nil {
		return ProviderFile{}, false, err
	}
	resolvedFinish, err := validateProviderFinishReason(path, providerName, finish, finishReasonModes)
	if err != nil {
		return ProviderFile{}, false, err
	}
	resolvedBalance, err := validateProviderBalance(path, providerName, balance, balanceModes)
	if err != nil {
		return ProviderFile{}, false, err
	}
	resolvedModels, err := validateProviderModels(path, providerName, models, modelsModes)
	if err != nil {
		return ProviderFile{}, false, err
	}
	return ProviderFile{
		Name:     providerName,
		Path:     path,
		Content:  content,
		Routing:  routing,
		Headers:  headers,
		Request:  req,
		Response: response,
		Error:    perr,
		Usage:    resolvedUsage,
		Finish:   resolvedFinish,
		Balance:  resolvedBalance,
		Models:   resolvedModels,
	}, true, nil
}

func usageModeFileFromError(msg string, modeFiles map[string][]string) string {
	for path := range modeFiles {
		if strings.Contains(msg, path) {
			return path
		}
	}
	return ""
}
