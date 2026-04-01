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
	paths := cfg.finishReasonPathConfigs()

	if mode == "" && len(paths) == 0 {
		return nil
	}
	switch mode {
	case "", "openai", "anthropic", "gemini":
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
	}
	return nil
}

func validateUsageExtractConfig(path, providerName, scope string, cfg UsageExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		if len(cfg.facts) > 0 {
			return fmt.Errorf("provider %q in %q: %s usage_fact requires usage_extract mode", providerName, path, scope)
		}
		return nil
	}
	switch mode {
	case usageModeOpenAI, usageModeAnthropic, usageModeGemini, usageModeCustom:
		// ok
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported usage_extract mode %q", providerName, path, scope, cfg.Mode),
			scope,
			"usage_extract",
		)
	}
	if mode == usageModeCustom {
		if cfg.InputTokensExpr == nil && strings.TrimSpace(cfg.InputTokensPath) == "" && !hasUsageFactForKey(cfg, "input", "token") {
			return fmt.Errorf("provider %q in %q: %s requires input_tokens_expr/input_tokens_path or usage_fact input token", providerName, path, scope)
		}
		if cfg.OutputTokensExpr == nil && strings.TrimSpace(cfg.OutputTokensPath) == "" && !hasUsageFactForKey(cfg, "output", "token") {
			return fmt.Errorf("provider %q in %q: %s requires output_tokens_expr/output_tokens_path or usage_fact output token", providerName, path, scope)
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
