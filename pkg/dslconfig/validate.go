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
	routing, headers, req, response, perr, usage, finish, balance, err := parseProviderConfig(p, content)
	if err != nil {
		return ProviderFile{}, err
	}
	if err := validateProviderBaseURL(p, providerName, routing); err != nil {
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
	}, nil
}

func validateProviderResponse(path, providerName string, resp ProviderResponse) error {
	if err := validateResponseDirective(path, providerName, "defaults.response", resp.Defaults); err != nil {
		return err
	}
	for i, m := range resp.Matches {
		scope := fmt.Sprintf("match[%d].response", i)
		if err := validateResponseDirective(path, providerName, scope, m.Response); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderHeaders(path, providerName string, headers ProviderHeaders) error {
	if err := validatePhaseHeaders(path, providerName, "defaults.auth", headers.Defaults); err != nil {
		return err
	}
	for i, m := range headers.Matches {
		scope := fmt.Sprintf("match[%d].auth", i)
		if err := validatePhaseHeaders(path, providerName, scope, m.Headers); err != nil {
			return err
		}
	}
	return nil
}

func validatePhaseHeaders(path, providerName, scope string, phase PhaseHeaders) error {
	if err := validateHeaderOps(path, providerName, scope+".headers", append(append([]HeaderOp(nil), phase.Auth...), phase.Request...)); err != nil {
		return err
	}
	return validateOAuthConfig(path, providerName, scope+".oauth", phase.OAuth)
}

func validateHeaderOps(path, providerName, scope string, headers []HeaderOp) error {
	const (
		opHeaderSet = "header_set"
		opHeaderDel = "header_del"
	)
	for i, op := range headers {
		opScope := fmt.Sprintf("%s[%d]", scope, i)
		if op.Op != opHeaderSet && op.Op != opHeaderDel {
			return fmt.Errorf("provider %q in %q: %s unsupported header op %q", providerName, path, opScope, op.Op)
		}
		if strings.TrimSpace(op.NameExpr) == "" {
			return fmt.Errorf("provider %q in %q: %s name is empty", providerName, path, opScope)
		}
		if op.Op == opHeaderSet && strings.TrimSpace(op.ValueExpr) == "" {
			return fmt.Errorf("provider %q in %q: %s value is empty", providerName, path, opScope)
		}
	}
	return nil
}

func validateOAuthConfig(path, providerName, scope string, cfg OAuthConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		if cfg.IsEmpty() {
			return nil
		}
		return fmt.Errorf("provider %q in %q: %s requires oauth_mode", providerName, path, scope)
	}
	if _, ok := oauthBuiltinTemplates[mode]; !ok {
		return fmt.Errorf("provider %q in %q: %s unsupported oauth_mode %q", providerName, path, scope, cfg.Mode)
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method != "" && method != "GET" && method != "POST" {
		return fmt.Errorf("provider %q in %q: %s oauth_method must be GET or POST", providerName, path, scope)
	}
	ct := strings.ToLower(strings.TrimSpace(cfg.ContentType))
	if ct != "" && ct != oauthContentTypeForm && ct != oauthContentTypeJSON {
		return fmt.Errorf("provider %q in %q: %s oauth_content_type must be form or json", providerName, path, scope)
	}
	if cfg.TimeoutMs != nil && *cfg.TimeoutMs <= 0 {
		return fmt.Errorf("provider %q in %q: %s oauth_timeout_ms must be > 0", providerName, path, scope)
	}
	if cfg.RefreshSkewSec != nil && *cfg.RefreshSkewSec < 0 {
		return fmt.Errorf("provider %q in %q: %s oauth_refresh_skew_sec must be >= 0", providerName, path, scope)
	}
	if cfg.FallbackTTLSeconds != nil && *cfg.FallbackTTLSeconds <= 0 {
		return fmt.Errorf("provider %q in %q: %s oauth_fallback_ttl_sec must be > 0", providerName, path, scope)
	}
	if mode != oauthModeCustom && len(cfg.Form) > 0 {
		// allow field overrides in builtin modes
		for i, f := range cfg.Form {
			if strings.TrimSpace(f.Key) == "" {
				return fmt.Errorf("provider %q in %q: %s oauth_form[%d] key is empty", providerName, path, scope, i)
			}
		}
	}
	if mode == oauthModeCustom {
		if strings.TrimSpace(cfg.TokenURLExpr) == "" {
			return fmt.Errorf("provider %q in %q: %s oauth_token_url is required in custom mode", providerName, path, scope)
		}
		if len(cfg.Form) == 0 {
			return fmt.Errorf("provider %q in %q: %s oauth_form is required in custom mode", providerName, path, scope)
		}
	}
	for _, pair := range []struct {
		name string
		val  string
	}{
		{name: "oauth_token_path", val: cfg.TokenPath},
		{name: "oauth_expires_in_path", val: cfg.ExpiresInPath},
		{name: "oauth_token_type_path", val: cfg.TokenTypePath},
	} {
		p := strings.TrimSpace(pair.val)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "$.") {
			return fmt.Errorf("provider %q in %q: %s %s must start with '$.'", providerName, path, scope, pair.name)
		}
	}
	return nil
}

func validateResponseDirective(path, providerName, scope string, d ResponseDirective) error {
	for i, r := range d.SSEJSONDelIf {
		rs := fmt.Sprintf("%s.sse_json_del_if[%d]", scope, i)
		if strings.TrimSpace(r.Equals) == "" {
			return fmt.Errorf("provider %q in %q: %s equals must be non-empty", providerName, path, rs)
		}
		if _, err := parseObjectPath(r.CondPath); err != nil {
			return fmt.Errorf("provider %q in %q: %s invalid cond path: %w", providerName, path, rs, err)
		}
		if _, err := parseObjectPath(r.DelPath); err != nil {
			return fmt.Errorf("provider %q in %q: %s invalid del path: %w", providerName, path, rs, err)
		}
	}
	for i, op := range d.JSONOps {
		opScope := fmt.Sprintf("%s.json_op[%d]", scope, i)
		switch strings.ToLower(strings.TrimSpace(op.Op)) {
		case "json_set", "json_del":
			if _, err := parseObjectPath(op.Path); err != nil {
				return fmt.Errorf("provider %q in %q: %s invalid json path: %w", providerName, path, opScope, err)
			}
		case "json_rename":
			if _, err := parseObjectPath(op.FromPath); err != nil {
				return fmt.Errorf("provider %q in %q: %s invalid from path: %w", providerName, path, opScope, err)
			}
			if _, err := parseObjectPath(op.ToPath); err != nil {
				return fmt.Errorf("provider %q in %q: %s invalid to path: %w", providerName, path, opScope, err)
			}
		default:
			return fmt.Errorf("provider %q in %q: %s unsupported json op %q", providerName, path, opScope, op.Op)
		}
	}
	return nil
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
	case "openai_chat_to_anthropic_messages":
		return nil
	case "openai_chat_to_gemini_generate_content":
		return nil
	case "anthropic_to_openai_chat":
		return nil
	case "gemini_to_openai_chat":
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

func validateProviderBalance(path, providerName string, balance ProviderBalance) error {
	if err := validateBalanceQueryConfig(path, providerName, "defaults.balance", balance.Defaults); err != nil {
		return err
	}
	for i, m := range balance.Matches {
		scope := fmt.Sprintf("match[%d].balance", i)
		if err := validateBalanceQueryConfig(path, providerName, scope, m.Query); err != nil {
			return err
		}
	}
	return nil
}

func validateBalanceQueryConfig(path, providerName, scope string, cfg BalanceQueryConfig) error {
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
	if err := validateBalanceExpr(path, providerName, scope, "balance", cfg.BalanceExpr); err != nil {
		return err
	}
	if err := validateBalanceExpr(path, providerName, scope, "used", cfg.UsedExpr); err != nil {
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
		return fmt.Errorf("provider %q in %q: %s unsupported balance_mode %q", providerName, path, scope, raw)
	}
}

func validateBalanceMethod(path, providerName, scope, methodRaw string) error {
	method := strings.ToUpper(strings.TrimSpace(methodRaw))
	if method == "" || method == "GET" || method == "POST" {
		return nil
	}
	return fmt.Errorf("provider %q in %q: %s method must be GET or POST", providerName, path, scope)
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
		return fmt.Errorf("provider %q in %q: %s requires balance_path or balance expr", providerName, path, scope)
	}
	return nil
}

func validateBalanceUnit(path, providerName, scope, unitRaw string) error {
	unit := strings.TrimSpace(unitRaw)
	if unit == "" || unit == "USD" || unit == "CNY" {
		return nil
	}
	return fmt.Errorf("provider %q in %q: %s balance_unit must be USD or CNY", providerName, path, scope)
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
