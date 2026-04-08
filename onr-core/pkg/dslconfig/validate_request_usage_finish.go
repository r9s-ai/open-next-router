package dslconfig

import (
	"fmt"
	"strings"
)

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
	case "openai_chat_to_anthropic_messages":
		return nil
	case "openai_chat_to_gemini_generate_content":
		return nil
	case "anthropic_to_openai_chat":
		return nil
	case "gemini_to_openai_chat":
		return nil
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported req_map mode %q", providerName, path, scope, t.ReqMapMode),
			scope,
			"req_map",
		)
	}
}

func validateProviderUsage(path, providerName string, usage ProviderUsage, registry usageModeRegistry) (ProviderUsage, error) {
	resolvedDefaults, err := validateUsageExtractConfig(path, providerName, "defaults.metrics", usage.Defaults, registry)
	if err != nil {
		return ProviderUsage{}, err
	}
	resolved := ProviderUsage{Defaults: resolvedDefaults}
	if len(usage.Matches) == 0 {
		return resolved, nil
	}
	resolved.Matches = make([]MatchUsage, 0, len(usage.Matches))
	for i, m := range usage.Matches {
		scope := fmt.Sprintf("match[%d].metrics", i)
		extract, err := validateUsageExtractConfig(path, providerName, scope, m.Extract, registry)
		if err != nil {
			return ProviderUsage{}, err
		}
		m.Extract = extract
		resolved.Matches = append(resolved.Matches, m)
	}
	return resolved, nil
}

func validateProviderFinishReason(path, providerName string, finish ProviderFinishReason, registry finishReasonModeRegistry) (ProviderFinishReason, error) {
	resolvedDefaults, err := validateFinishReasonExtractConfig(path, providerName, "defaults.metrics", finish.Defaults, registry)
	if err != nil {
		return ProviderFinishReason{}, err
	}
	resolved := ProviderFinishReason{Defaults: resolvedDefaults}
	if len(finish.Matches) == 0 {
		return resolved, nil
	}
	resolved.Matches = make([]MatchFinishReason, 0, len(finish.Matches))
	for i, m := range finish.Matches {
		scope := fmt.Sprintf("match[%d].metrics", i)
		extract, err := validateFinishReasonExtractConfig(path, providerName, scope, m.Extract, registry)
		if err != nil {
			return ProviderFinishReason{}, err
		}
		m.Extract = extract
		resolved.Matches = append(resolved.Matches, m)
	}
	return resolved, nil
}

func validateFinishReasonExtractConfig(path, providerName, scope string, cfg FinishReasonExtractConfig, registry finishReasonModeRegistry) (FinishReasonExtractConfig, error) {
	resolved, err := resolveFinishReasonExtractConfig(path, providerName, scope, cfg, registry, nil)
	if err != nil {
		return FinishReasonExtractConfig{}, err
	}
	if err := validateResolvedFinishReasonExtractConfig(path, providerName, scope, resolved); err != nil {
		return FinishReasonExtractConfig{}, err
	}
	return normalizeFinishReasonExtractConfig(resolved), nil
}

func validateResolvedFinishReasonExtractConfig(path, providerName, scope string, cfg FinishReasonExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	paths := cfg.finishReasonPathConfigs()

	if mode == "" && len(paths) == 0 {
		return nil
	}
	switch mode {
	case "":
		// ok
	case usageModeCustom:
		if len(paths) == 0 {
			return fmt.Errorf("provider %q in %q: %s finish_reason_extract custom requires finish_reason_path", providerName, path, scope)
		}
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported finish_reason_extract mode %q", providerName, path, scope, cfg.Mode),
			scope,
			"finish_reason_extract",
		)
	}
	for i, pathRule := range paths {
		p := strings.TrimSpace(pathRule.Path)
		if p == "" {
			return fmt.Errorf("provider %q in %q: %s finish_reason_path[%d] is empty", providerName, path, scope, i)
		}
		if !strings.HasPrefix(p, "$.") {
			return fmt.Errorf("provider %q in %q: %s finish_reason_path[%d] must start with $. ", providerName, path, scope, i)
		}
		if strings.TrimSpace(pathRule.Event) == "" {
			if pathRule.EventOptional {
				return fmt.Errorf("provider %q in %q: %s finish_reason_path[%d] event_optional requires event", providerName, path, scope, i)
			}
		} else if strings.ContainsAny(strings.TrimSpace(pathRule.Event), " \t\r\n") {
			return fmt.Errorf("provider %q in %q: %s finish_reason_path[%d] event must not contain whitespace", providerName, path, scope, i)
		}
	}
	return nil
}

func validateUsageExtractConfig(path, providerName, scope string, cfg UsageExtractConfig, registry usageModeRegistry) (UsageExtractConfig, error) {
	resolved, err := resolveUsageExtractConfig(path, providerName, scope, cfg, registry, nil)
	if err != nil {
		return UsageExtractConfig{}, err
	}
	if err := validateResolvedUsageExtractConfig(path, providerName, scope, resolved); err != nil {
		return UsageExtractConfig{}, err
	}
	return resolved, nil
}

func validateResolvedUsageExtractConfig(path, providerName, scope string, cfg UsageExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		if len(cfg.facts) > 0 {
			return fmt.Errorf("provider %q in %q: %s usage_fact requires usage_extract mode", providerName, path, scope)
		}
		return nil
	}
	switch mode {
	case usageModeCustom:
		// ok
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported usage_extract mode %q", providerName, path, scope, cfg.Mode),
			scope,
			"usage_extract",
		)
	}
	if mode == usageModeCustom {
		if !hasAnyUsageExtractionRule(cfg) {
			return fmt.Errorf("provider %q in %q: %s custom usage_extract requires at least one usage_fact or *_tokens_path/*_expr rule", providerName, path, scope)
		}
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

	for i, fact := range cfg.facts {
		if err := validateUsageFactConfig(path, providerName, scope, i, fact); err != nil {
			return err
		}
	}
	return nil
}

func hasAnyUsageExtractionRule(cfg UsageExtractConfig) bool {
	return cfg.InputTokensExpr != nil ||
		strings.TrimSpace(cfg.InputTokensPath) != "" ||
		cfg.OutputTokensExpr != nil ||
		strings.TrimSpace(cfg.OutputTokensPath) != "" ||
		cfg.CacheReadTokensExpr != nil ||
		strings.TrimSpace(cfg.CacheReadTokensPath) != "" ||
		cfg.CacheWriteTokensExpr != nil ||
		strings.TrimSpace(cfg.CacheWriteTokensPath) != "" ||
		cfg.TotalTokensExpr != nil ||
		len(cfg.facts) > 0
}

func hasUsageFactForKey(cfg UsageExtractConfig, dim, unit string) bool {
	key := normalizeUsageFactKey(dim, unit)
	for _, fact := range cfg.facts {
		if normalizeUsageFactKey(fact.Dimension, fact.Unit) == key {
			return true
		}
	}
	return false
}

func validateUsageFactConfig(path, providerName, scope string, idx int, fact usageFactConfig) error {
	key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
	if !usageFactKeyAllowed(key.Dimension, key.Unit) {
		return fmt.Errorf("provider %q in %q: %s usage_fact[%d] unsupported dimension/unit %q %q", providerName, path, scope, idx, fact.Dimension, fact.Unit)
	}
	if source := strings.ToLower(strings.TrimSpace(fact.Source)); source != "" && source != "response" && source != "request" && source != "derived" {
		return fmt.Errorf("provider %q in %q: %s usage_fact[%d] unsupported source %q", providerName, path, scope, idx, fact.Source)
	}

	primitiveCount := 0
	if strings.TrimSpace(fact.Path) != "" {
		primitiveCount++
		if !strings.HasPrefix(strings.TrimSpace(fact.Path), "$.") {
			return fmt.Errorf("provider %q in %q: %s usage_fact[%d] path must start with $. ", providerName, path, scope, idx)
		}
	}
	if strings.TrimSpace(fact.CountPath) != "" {
		primitiveCount++
		if !strings.HasPrefix(strings.TrimSpace(fact.CountPath), "$.") {
			return fmt.Errorf("provider %q in %q: %s usage_fact[%d] count_path must start with $. ", providerName, path, scope, idx)
		}
	}
	if strings.TrimSpace(fact.SumPath) != "" {
		primitiveCount++
		if !strings.HasPrefix(strings.TrimSpace(fact.SumPath), "$.") {
			return fmt.Errorf("provider %q in %q: %s usage_fact[%d] sum_path must start with $. ", providerName, path, scope, idx)
		}
	}
	if fact.Expr != nil {
		primitiveCount++
	}
	if primitiveCount != 1 {
		return fmt.Errorf("provider %q in %q: %s usage_fact[%d] requires exactly one of path, count_path, sum_path or expr", providerName, path, scope, idx)
	}

	if strings.TrimSpace(fact.Type) != "" || strings.TrimSpace(fact.Status) != "" {
		if strings.TrimSpace(fact.CountPath) == "" {
			return fmt.Errorf("provider %q in %q: %s usage_fact[%d] type/status requires count_path", providerName, path, scope, idx)
		}
	}
	if strings.TrimSpace(fact.Event) == "" {
		if fact.EventOptional {
			return fmt.Errorf("provider %q in %q: %s usage_fact[%d] event_optional requires event", providerName, path, scope, idx)
		}
	} else if strings.ContainsAny(strings.TrimSpace(fact.Event), " \t\r\n") {
		return fmt.Errorf("provider %q in %q: %s usage_fact[%d] event must not contain whitespace", providerName, path, scope, idx)
	}
	if len(fact.Attrs) > 0 {
		for k, v := range fact.Attrs {
			if strings.TrimSpace(k) == "" {
				return fmt.Errorf("provider %q in %q: %s usage_fact[%d] attr name is empty", providerName, path, scope, idx)
			}
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("provider %q in %q: %s usage_fact[%d] attr.%s is empty", providerName, path, scope, idx, k)
			}
		}
	}
	return nil
}
