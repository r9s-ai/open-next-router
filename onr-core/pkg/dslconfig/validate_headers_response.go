package dslconfig

import (
	"fmt"
	"strings"
)

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
		opHeaderSet    = "header_set"
		opHeaderDel    = "header_del"
		opHeaderPass   = "header_pass"
		opHeaderFilter = "header_filter_values"
	)
	for i, op := range headers {
		opScope := fmt.Sprintf("%s[%d]", scope, i)
		if op.Op != opHeaderSet && op.Op != opHeaderDel && op.Op != opHeaderPass && op.Op != opHeaderFilter {
			return fmt.Errorf("provider %q in %q: %s unsupported header op %q", providerName, path, opScope, op.Op)
		}
		if strings.TrimSpace(op.NameExpr) == "" {
			return fmt.Errorf("provider %q in %q: %s name is empty", providerName, path, opScope)
		}
		if op.Op == opHeaderSet && strings.TrimSpace(op.ValueExpr) == "" {
			return fmt.Errorf("provider %q in %q: %s value is empty", providerName, path, opScope)
		}
		if op.Op == opHeaderFilter {
			if len(op.Patterns) == 0 {
				return fmt.Errorf("provider %q in %q: %s requires at least one pattern", providerName, path, opScope)
			}
			for j, pattern := range op.Patterns {
				if strings.TrimSpace(pattern) == "" {
					return fmt.Errorf("provider %q in %q: %s pattern[%d] is empty", providerName, path, opScope, j)
				}
			}
			if strings.TrimSpace(op.Separator) == "" {
				return fmt.Errorf("provider %q in %q: %s separator must be non-empty", providerName, path, opScope)
			}
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
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported oauth_mode %q", providerName, path, scope, cfg.Mode),
			scope,
			"oauth_mode",
		)
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method != "" && method != "GET" && method != "POST" {
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s oauth_method must be GET or POST", providerName, path, scope),
			scope,
			"oauth_method",
		)
	}
	ct := strings.ToLower(strings.TrimSpace(cfg.ContentType))
	if ct != "" && ct != oauthContentTypeForm && ct != oauthContentTypeJSON {
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s oauth_content_type must be form or json", providerName, path, scope),
			scope,
			"oauth_content_type",
		)
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
		case "json_set", "json_set_if_absent", "json_del":
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
