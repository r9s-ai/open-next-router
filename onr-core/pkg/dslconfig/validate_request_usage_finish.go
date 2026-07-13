package dslconfig

import (
	"fmt"
	"strconv"
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
	if err := validateModelMapConfig(path, providerName, scope+".model_map", t.ModelMap); err != nil {
		return err
	}
	if err := validateRequestJSONOps(path, providerName, scope, t.JSONOps); err != nil {
		return err
	}
	if err := validateRequestJSONOps(path, providerName, scope+".after_req_map", t.AfterReqMapJSONOps); err != nil {
		return err
	}

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

func validateModelMapConfig(path, providerName, scope string, cfg ModelMapConfig) error {
	for from, expr := range cfg.Map {
		if err := ValidateStringExpr(expr); err != nil {
			return fmt.Errorf("provider %q in %q: %s[%q] invalid expression: %w", providerName, path, scope, from, err)
		}
	}
	if strings.TrimSpace(cfg.DefaultExpr) != "" {
		if err := ValidateStringExpr(cfg.DefaultExpr); err != nil {
			return fmt.Errorf("provider %q in %q: %s_default invalid expression: %w", providerName, path, scope, err)
		}
	}
	return nil
}

func validateRequestJSONOps(path, providerName, scope string, ops []JSONOp) error {
	for i, op := range ops {
		opScope := fmt.Sprintf("%s.json_op[%d]", scope, i)
		if err := validateRequestJSONOp(path, providerName, opScope, op); err != nil {
			return err
		}
	}
	return nil
}

// validateRequestJSONOp validates a single request JSON op. opScope already
// includes the op index for error messages.
func validateRequestJSONOp(path, providerName, opScope string, op JSONOp) error {
	invalidPath := func(kind string, err error) error {
		return fmt.Errorf("provider %q in %q: %s invalid %s: %w", providerName, path, opScope, kind, err)
	}
	requireErr := func(msg string) error {
		return fmt.Errorf("provider %q in %q: %s %s", providerName, path, opScope, msg)
	}
	switch strings.ToLower(strings.TrimSpace(op.Op)) {
	case jsonOpDelWithCond:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if strings.TrimSpace(op.FieldName) == "" {
			return requireErr("json_del_with_condition requires field name")
		}
		if len(op.Patterns) == 0 {
			return requireErr("json_del_with_condition requires at least one pattern")
		}
	case jsonOpSet, jsonOpReplace, jsonOpSetIfAbsent, jsonOpDel, jsonOpWrapInputText:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if op.Op == jsonOpSet || op.Op == jsonOpReplace || op.Op == jsonOpSetIfAbsent {
			if err := validateJSONValueExpr(op.ValueExpr); err != nil {
				return invalidPath("value expression", err)
			}
		}
	case jsonOpDelIfMissing:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if _, err := parseObjectPath(op.FromPath); err != nil {
			return invalidPath("required path", err)
		}
	case jsonOpSetHeaderVals:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if strings.TrimSpace(op.HeaderName) == "" {
			return requireErr("json_set_header_values requires header name")
		}
	case jsonOpFilterValues:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if len(op.Patterns) == 0 {
			return requireErr("json_filter_values requires at least one pattern")
		}
	case jsonOpRename:
		if _, err := parseObjectPath(op.FromPath); err != nil {
			return invalidPath("from path", err)
		}
		if _, err := parseObjectPath(op.ToPath); err != nil {
			return invalidPath("to path", err)
		}
	case jsonOpMapValue:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if err := validateJSONValueExpr(op.ValueExpr); err != nil {
			return invalidPath("value expression", err)
		}
	case jsonOpClamp:
		if _, err := parseObjectPath(op.Path); err != nil {
			return invalidPath("json path", err)
		}
		if op.ClampRange == nil || op.ClampRange.Max < op.ClampRange.Min {
			return requireErr("json_clamp requires max >= min")
		}
	default:
		return fmt.Errorf("provider %q in %q: %s unsupported json op %q", providerName, path, opScope, op.Op)
	}
	return nil
}

func validateJSONValueExpr(expr string) error {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return fmt.Errorf("expression is empty")
	}
	switch raw {
	case "true", "false", "null":
		return nil
	}
	if isQuotedStringExpr(raw) {
		return nil
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return nil
	}
	if _, ok := parseFloatLiteral(raw); ok {
		return nil
	}
	return ValidateStringExpr(raw)
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
		for i, root := range cfg.usageRoots {
			if err := validateUsageRootConfig(path, providerName, scope, i, root); err != nil {
				return err
			}
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
		if strings.EqualFold(strings.TrimSpace(fact.Source), "usage") && len(cfg.usageRoots) == 0 {
			return fmt.Errorf("provider %q in %q: %s usage_fact[%d] source=usage requires usage_root", providerName, path, scope, i)
		}
	}
	for i, root := range cfg.usageRoots {
		if err := validateUsageRootConfig(path, providerName, scope, i, root); err != nil {
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

func validateUsageFactConfig(path, providerName, scope string, idx int, fact usageFactConfig) error {
	key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
	if !usageFactKeyAllowed(key.Dimension, key.Unit) {
		return fmt.Errorf("provider %q in %q: %s usage_fact[%d] unsupported dimension/unit %q %q", providerName, path, scope, idx, fact.Dimension, fact.Unit)
	}
	if source := strings.ToLower(strings.TrimSpace(fact.Source)); source != "" && source != "usage" && source != "response" && source != "request" && source != "derived" {
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
	} else if err := validateUsageEventList(strings.TrimSpace(fact.Event)); err != nil {
		return fmt.Errorf("provider %q in %q: %s usage_fact[%d] %s", providerName, path, scope, idx, err.Error())
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

func validateUsageRootConfig(path, providerName, scope string, idx int, root usageRootConfig) error {
	if strings.TrimSpace(root.Path) == "" {
		return fmt.Errorf("provider %q in %q: %s usage_root[%d] requires path", providerName, path, scope, idx)
	}
	if !strings.HasPrefix(strings.TrimSpace(root.Path), "$.") {
		return fmt.Errorf("provider %q in %q: %s usage_root[%d] path must start with $. ", providerName, path, scope, idx)
	}
	for _, field := range root.ExcludeFields {
		if err := validateUsageRootExcludeField(field); err != nil {
			return fmt.Errorf("provider %q in %q: %s usage_root[%d] exclude %q invalid: %w", providerName, path, scope, idx, field, err)
		}
	}
	if strings.TrimSpace(root.Event) == "" {
		if root.EventOptional {
			return fmt.Errorf("provider %q in %q: %s usage_root[%d] event_optional requires event", providerName, path, scope, idx)
		}
		return nil
	}
	if err := validateUsageEventList(strings.TrimSpace(root.Event)); err != nil {
		return fmt.Errorf("provider %q in %q: %s usage_root[%d] %s", providerName, path, scope, idx, err.Error())
	}
	return nil
}

func validateUsageRootExcludeField(field string) error {
	field = strings.TrimSpace(field)
	if field == "" {
		return fmt.Errorf("field name is empty")
	}
	if strings.ContainsAny(field, " \t\r\n.$[]") {
		return fmt.Errorf("field name must be a top-level key, not a JSONPath")
	}
	return nil
}

func validateUsageEventList(event string) error {
	if strings.TrimSpace(event) == "" {
		return nil
	}
	for _, part := range strings.Split(event, "|") {
		item := strings.TrimSpace(part)
		if item == "" {
			return fmt.Errorf("event contains empty name")
		}
		if strings.ContainsAny(item, " \t\r\n") {
			return fmt.Errorf("event must not contain whitespace")
		}
	}
	return nil
}
