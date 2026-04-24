package dslconfig

import (
	"fmt"
	"regexp"
	"strings"
)

func validateProviderBalance(path, providerName string, balance ProviderBalance, registry balanceModeRegistry) (ProviderBalance, error) {
	resolvedDefaults, err := validateBalanceQueryConfig(path, providerName, "defaults.balance", balance.Defaults, registry)
	if err != nil {
		return ProviderBalance{}, err
	}
	resolved := ProviderBalance{Defaults: resolvedDefaults}
	for i, m := range balance.Matches {
		scope := fmt.Sprintf("match[%d].balance", i)
		query, err := validateBalanceQueryConfig(path, providerName, scope, m.Query, registry)
		if err != nil {
			return ProviderBalance{}, err
		}
		m.Query = query
		resolved.Matches = append(resolved.Matches, m)
	}
	return resolved, nil
}

func validateProviderModels(path, providerName string, models ProviderModels, registry modelsModeRegistry) (ProviderModels, error) {
	resolved, err := validateModelsQueryConfig(path, providerName, "defaults.models", models.Defaults, registry)
	if err != nil {
		return ProviderModels{}, err
	}
	return ProviderModels{Defaults: resolved}, nil
}

func validateBalanceQueryConfig(path, providerName, scope string, cfg BalanceQueryConfig, registry balanceModeRegistry) (BalanceQueryConfig, error) {
	resolved, err := resolveBalanceQueryConfig(path, providerName, scope, cfg, registry, nil)
	if err != nil {
		return BalanceQueryConfig{}, err
	}
	if err := validateResolvedBalanceQueryConfig(path, providerName, scope, resolved); err != nil {
		return BalanceQueryConfig{}, err
	}
	return resolved, nil
}

func validateResolvedBalanceQueryConfig(path, providerName, scope string, cfg BalanceQueryConfig) error {
	cfgp := inferImplicitCustomBalanceQueryConfig(&cfg)
	cfg = *cfgp
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil
	}
	if err := validateBalanceMode(path, providerName, scope, cfg.Mode, mode); err != nil {
		return err
	}
	if err := validateBalanceMethod(path, providerName, scope, cfg.Method); err != nil {
		return err
	}
	if err := validateBalancePaths(path, providerName, scope, cfg); err != nil {
		return err
	}
	if err := validateBalanceExpr(path, providerName, scope, "balance_expr", cfg.BalanceExpr); err != nil {
		return err
	}
	if err := validateBalanceExpr(path, providerName, scope, "used_expr", cfg.UsedExpr); err != nil {
		return err
	}
	if err := validateCustomBalanceConfig(path, providerName, scope, mode, cfg); err != nil {
		return err
	}
	if err := validateBalanceUnit(path, providerName, scope, cfg.Unit); err != nil {
		return err
	}
	if err := validateBalanceURLPath(path, providerName, scope, "subscription_path", cfg.SubscriptionPath); err != nil {
		return err
	}
	if err := validateBalanceURLPath(path, providerName, scope, "usage_path", cfg.UsagePath); err != nil {
		return err
	}
	return validateBalanceHeaders(path, providerName, scope, cfg.Headers)
}

func validateBalanceMode(path, providerName, scope, raw, mode string) error {
	switch mode {
	case balanceModeOpenAI, balanceModeCustom:
		return nil
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported balance_mode %q", providerName, path, scope, raw),
			scope,
			"balance_mode",
		)
	}
}

func validateBalanceMethod(path, providerName, scope, methodRaw string) error {
	method := strings.ToUpper(strings.TrimSpace(methodRaw))
	if method == "" || method == "GET" || method == "POST" {
		return nil
	}
	return validationIssue(
		fmt.Errorf("provider %q in %q: %s method must be GET or POST", providerName, path, scope),
		scope,
		"method",
	)
}

func validateBalancePaths(path, providerName, scope string, cfg BalanceQueryConfig) error {
	for _, pair := range []struct {
		name string
		val  string
	}{
		{"balance_path", cfg.BalancePath},
		{"used_path", cfg.UsedPath},
	} {
		v := strings.TrimSpace(pair.val)
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "$.") {
			return fmt.Errorf("provider %q in %q: %s %s must start with '$.'", providerName, path, scope, pair.name)
		}
	}
	return nil
}

func validateBalanceExpr(path, providerName, scope, field, expr string) error {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil
	}
	if _, err := ParseBalanceExpr(trimmed); err != nil {
		return fmt.Errorf("provider %q in %q: %s invalid %s expr: %w", providerName, path, scope, field, err)
	}
	return nil
}

func validateCustomBalanceConfig(path, providerName, scope, mode string, cfg BalanceQueryConfig) error {
	if mode != balanceModeCustom {
		return nil
	}
	if strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("provider %q in %q: %s path is required when balance_mode=custom", providerName, path, scope)
	}
	if strings.TrimSpace(cfg.BalanceExpr) == "" && strings.TrimSpace(cfg.BalancePath) == "" {
		return fmt.Errorf("provider %q in %q: %s requires balance_path or balance_expr", providerName, path, scope)
	}
	return nil
}

func validateBalanceUnit(path, providerName, scope, unitRaw string) error {
	unit := strings.TrimSpace(unitRaw)
	if unit == "" || unit == "USD" || unit == "CNY" {
		return nil
	}
	return validationIssue(
		fmt.Errorf("provider %q in %q: %s balance_unit must be USD or CNY", providerName, path, scope),
		scope,
		"balance_unit",
	)
}

func validateBalanceURLPath(path, providerName, scope, field, value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	if strings.HasPrefix(v, "/") || strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return nil
	}
	return fmt.Errorf("provider %q in %q: %s %s must start with / or http(s)://", providerName, path, scope, field)
}

func validateBalanceHeaders(path, providerName, scope string, headers []HeaderOp) error {
	return validateHeaderOps(path, providerName, scope+".headers", headers)
}

func validateModelsQueryConfig(path, providerName, scope string, cfg ModelsQueryConfig, registry modelsModeRegistry) (ModelsQueryConfig, error) {
	resolved, err := resolveModelsQueryConfig(path, providerName, scope, cfg, registry, nil)
	if err != nil {
		return ModelsQueryConfig{}, err
	}
	if err := validateResolvedModelsQueryConfig(path, providerName, scope, resolved); err != nil {
		return ModelsQueryConfig{}, err
	}
	return normalizeModelsQueryConfig(resolved), nil
}

func validateResolvedModelsQueryConfig(path, providerName, scope string, cfg ModelsQueryConfig) error {
	cfg = inferImplicitCustomModelsQueryConfig(cfg)
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil
	}
	switch mode {
	case modelsModeOpenAI, modelsModeGemini, modelsModeCustom:
		// ok
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported models_mode %q", providerName, path, scope, cfg.Mode),
			scope,
			"models_mode",
		)
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method != "" && method != "GET" && method != "POST" {
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s method must be GET or POST", providerName, path, scope),
			scope,
			"method",
		)
	}
	if mode == modelsModeCustom && strings.TrimSpace(cfg.Path) == "" {
		return fmt.Errorf("provider %q in %q: %s path is required when models_mode=custom", providerName, path, scope)
	}
	if err := validateBalanceURLPath(path, providerName, scope, "path", cfg.Path); err != nil {
		return err
	}

	idPaths := cfg.IDPaths
	if mode == modelsModeOpenAI && len(idPaths) == 0 {
		idPaths = []string{"$.data[*].id"}
	}
	if mode == modelsModeGemini && len(idPaths) == 0 {
		idPaths = []string{"$.models[*].name"}
	}
	if mode == modelsModeCustom && len(idPaths) == 0 {
		return fmt.Errorf("provider %q in %q: %s requires at least one id_path", providerName, path, scope)
	}
	for i, p := range idPaths {
		v := strings.TrimSpace(p)
		if !strings.HasPrefix(v, "$.") {
			return fmt.Errorf("provider %q in %q: %s id_path[%d] must start with '$.'", providerName, path, scope, i)
		}
	}
	if strings.TrimSpace(cfg.IDRegex) != "" {
		if _, err := regexp.Compile(strings.TrimSpace(cfg.IDRegex)); err != nil {
			return fmt.Errorf("provider %q in %q: %s invalid id_regex: %w", providerName, path, scope, err)
		}
	}
	if strings.TrimSpace(cfg.IDAllowRegex) != "" {
		if _, err := regexp.Compile(strings.TrimSpace(cfg.IDAllowRegex)); err != nil {
			return fmt.Errorf("provider %q in %q: %s invalid id_allow_regex: %w", providerName, path, scope, err)
		}
	}
	return validateHeaderOps(path, providerName, scope+".headers", cfg.Headers)
}
