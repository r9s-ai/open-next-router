package dslconfig

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmetadata"
)

func ExportProviderMetadata(p ProviderFile) dslmetadata.ProviderConfig {
	cfg := dslmetadata.ProviderConfig{
		Metadata: exportProviderMetadata(p.Metadata),
		Routes:   exportProviderRoutes(p.Routing),
		Upstream: exportProviderUpstream(p.Routing),
		Request:  exportProviderRequest(p.Request),
		Auth:     exportProviderAuth(p.Headers),
		Balance:  exportProviderBalance(p.Balance),
		Models:   exportProviderModels(p.Models),
	}
	if usageFacts := exportProviderUsageFacts(p.Usage); usageFacts != nil {
		cfg.UsageFacts = usageFacts
	}
	return dslmetadata.NormalizeProviderConfig(cfg)
}

func exportProviderUpstream(routing ProviderRouting) *dslmetadata.ProviderUpstream {
	transport := strings.ToLower(strings.TrimSpace(routing.Transport))
	if transport == "" {
		return nil
	}
	return &dslmetadata.ProviderUpstream{
		Transport: transport,
	}
}

func exportProviderMetadata(metadata ProviderMetadata) *dslmetadata.ProviderMetadata {
	providerFamily := strings.ToLower(strings.TrimSpace(metadata.ProviderFamily))
	signalProfile := strings.ToLower(strings.TrimSpace(metadata.SignalProfile))
	if providerFamily == "" && signalProfile == "" {
		return nil
	}
	return &dslmetadata.ProviderMetadata{
		ProviderFamily: providerFamily,
		SignalProfile:  signalProfile,
	}
}

func exportProviderRoutes(routing ProviderRouting) []dslmetadata.ProviderRoute {
	if len(routing.Matches) == 0 {
		return nil
	}
	out := make([]dslmetadata.ProviderRoute, 0, len(routing.Matches))
	for _, match := range routing.Matches {
		api := strings.ToLower(strings.TrimSpace(match.API))
		if api == "" {
			continue
		}
		path, ok := exportRouteTemplate(match.SetPath, match.QueryPairs)
		if !ok {
			continue
		}
		var streamPtr *bool
		if match.Stream != nil {
			v := *match.Stream
			streamPtr = &v
		}
		out = append(out, dslmetadata.ProviderRoute{
			API:    api,
			Stream: streamPtr,
			Path:   path,
		})
	}
	return out
}

func exportProviderRequest(request ProviderRequestTransform) *dslmetadata.ProviderRequest {
	out := &dslmetadata.ProviderRequest{
		Defaults: exportRequestTransform(request.Defaults),
	}
	for _, match := range request.Matches {
		api := strings.ToLower(strings.TrimSpace(match.API))
		var streamPtr *bool
		if match.Stream != nil {
			v := *match.Stream
			streamPtr = &v
		}
		out.Matches = append(out.Matches, dslmetadata.RequestTransformMatch{
			API:       api,
			Stream:    streamPtr,
			Transform: exportRequestTransform(match.Transform),
		})
	}
	normalized := dslmetadata.NormalizeProviderConfig(dslmetadata.ProviderConfig{Request: out})
	return normalized.Request
}

func exportRequestTransform(t RequestTransform) dslmetadata.RequestTransform {
	return dslmetadata.RequestTransform{
		ModelMap: dslmetadata.ModelMap{
			Map:         cloneStringMapForMetadata(t.ModelMap.Map),
			DefaultExpr: strings.TrimSpace(t.ModelMap.DefaultExpr),
		},
		JSONOps:            exportJSONOps(t.JSONOps),
		AfterReqMapJSONOps: exportJSONOps(t.AfterReqMapJSONOps),
		ReqMapMode:         strings.TrimSpace(t.ReqMapMode),
	}
}

func exportJSONOps(in []JSONOp) []dslmetadata.JSONOp {
	if len(in) == 0 {
		return nil
	}
	out := make([]dslmetadata.JSONOp, 0, len(in))
	for _, op := range in {
		out = append(out, dslmetadata.JSONOp{
			Op:            strings.TrimSpace(op.Op),
			Path:          strings.TrimSpace(op.Path),
			FromPath:      strings.TrimSpace(op.FromPath),
			ToPath:        strings.TrimSpace(op.ToPath),
			ValueExpr:     strings.TrimSpace(op.ValueExpr),
			HeaderName:    strings.TrimSpace(op.HeaderName),
			FieldName:     strings.TrimSpace(op.FieldName),
			Patterns:      trimStringSliceForMetadata(op.Patterns),
			Separator:     strings.TrimSpace(op.Separator),
			Event:         strings.TrimSpace(op.Event),
			EventOptional: op.EventOptional,
			MaxCount:      op.MaxCount,
		})
	}
	return out
}

func exportProviderUsageFacts(usage ProviderUsage) *dslmetadata.ProviderUsageFacts {
	plans := usage.CompiledPlans()
	out := &dslmetadata.ProviderUsageFacts{
		Defaults: exportUsageFacts(plans.Defaults.Facts),
	}
	for _, match := range plans.Matches {
		api := strings.ToLower(strings.TrimSpace(match.API))
		facts := exportUsageFacts(match.Plan.Facts)
		if len(facts) == 0 {
			continue
		}
		var streamPtr *bool
		if match.Stream != nil {
			v := *match.Stream
			streamPtr = &v
		}
		out.Matches = append(out.Matches, dslmetadata.UsageFactMatch{
			API:    api,
			Stream: streamPtr,
			Facts:  facts,
		})
	}
	if len(out.Defaults) == 0 && len(out.Matches) == 0 {
		return nil
	}
	return out
}

func exportUsageFacts(in []UsageFact) []dslmetadata.UsageFact {
	if len(in) == 0 {
		return nil
	}
	out := make([]dslmetadata.UsageFact, 0, len(in))
	for _, fact := range in {
		out = append(out, dslmetadata.UsageFact{
			Dimension:  strings.ToLower(strings.TrimSpace(fact.Dimension)),
			Unit:       strings.ToLower(strings.TrimSpace(fact.Unit)),
			Source:     strings.ToLower(strings.TrimSpace(fact.Source)),
			Path:       strings.TrimSpace(fact.Path),
			CountPath:  strings.TrimSpace(fact.CountPath),
			SumPath:    strings.TrimSpace(fact.SumPath),
			LenPath:    strings.TrimSpace(fact.LenPath),
			Expr:       strings.TrimSpace(fact.Expr),
			Type:       strings.TrimSpace(fact.Type),
			Status:     strings.TrimSpace(fact.Status),
			Event:      strings.TrimSpace(fact.Event),
			Fallback:   fact.Fallback,
			WhenPath:   strings.TrimSpace(fact.WhenPath),
			WhenEq:     strings.TrimSpace(fact.WhenEq),
			Scale:      fact.Scale,
			Attributes: normalizeMetadataAttributes(fact.Attributes),
		})
	}
	return out
}

func exportProviderBalance(balance ProviderBalance) *dslmetadata.ProviderBalance {
	cfg := balance.Defaults
	if strings.TrimSpace(cfg.Mode) == "" {
		return nil
	}
	out := &dslmetadata.ProviderBalance{
		Mode:             strings.TrimSpace(cfg.Mode),
		Method:           strings.TrimSpace(cfg.Method),
		Path:             strings.TrimSpace(cfg.Path),
		BalancePath:      strings.TrimSpace(cfg.BalancePath),
		BalanceExpr:      strings.TrimSpace(cfg.BalanceExpr),
		UsedPath:         strings.TrimSpace(cfg.UsedPath),
		UsedExpr:         strings.TrimSpace(cfg.UsedExpr),
		Unit:             strings.TrimSpace(cfg.Unit),
		SubscriptionPath: strings.TrimSpace(cfg.SubscriptionPath),
		UsagePath:        strings.TrimSpace(cfg.UsagePath),
		Headers:          exportHeaderOps(cfg.Headers),
	}
	return out
}

func exportProviderModels(models ProviderModels) *dslmetadata.ProviderModels {
	cfg, ok := models.Select(nil)
	if !ok || cfg == nil || strings.TrimSpace(cfg.Mode) == "" {
		return nil
	}
	out := &dslmetadata.ProviderModels{
		Mode:         strings.TrimSpace(cfg.Mode),
		Method:       strings.TrimSpace(cfg.Method),
		Path:         strings.TrimSpace(cfg.Path),
		IDPaths:      trimStringSliceForMetadata(cfg.IDPaths),
		IDRegex:      strings.TrimSpace(cfg.IDRegex),
		IDAllowRegex: strings.TrimSpace(cfg.IDAllowRegex),
		Headers:      exportHeaderOps(cfg.Headers),
	}
	return out
}

func exportProviderAuth(headers ProviderHeaders) *dslmetadata.ProviderAuth {
	if auth := exportAuthFromPhase(headers.Defaults); auth != nil {
		return auth
	}
	for _, match := range headers.Matches {
		if auth := exportAuthFromPhase(match.Headers); auth != nil {
			return auth
		}
	}
	return nil
}

func exportAuthFromPhase(phase PhaseHeaders) *dslmetadata.ProviderAuth {
	if phase.AWSSigV4 {
		return &dslmetadata.ProviderAuth{
			Type:           "aws_sigv4",
			Service:        "bedrock",
			Credentials:    "static_ak_sk",
			RequiresRegion: true,
		}
	}
	for _, op := range phase.Auth {
		if strings.TrimSpace(op.Op) != "header_set" {
			continue
		}
		name := exportHeaderName(op.NameExpr)
		if name == "" {
			continue
		}
		value := strings.TrimSpace(op.ValueExpr)
		if value == "" {
			continue
		}
		auth := &dslmetadata.ProviderAuth{Header: name}
		lower := strings.ToLower(value)
		switch {
		case strings.Contains(value, "$channel.key"):
			auth.Type = "header_key"
			if strings.Contains(lower, "bearer") {
				auth.Type = "bearer"
			}
		case strings.Contains(value, "$oauth.access_token"):
			auth.Type = "oauth_bearer"
			auth.Mode = strings.ToLower(strings.TrimSpace(phase.OAuth.Mode))
			auth.Scope = exportAuthExprValue(phase.OAuth.ScopeExpr)
			auth.TokenURL = exportAuthExprValue(phase.OAuth.TokenURLExpr)
		default:
			continue
		}
		return auth
	}
	return nil
}

func exportHeaderOps(in []HeaderOp) []dslmetadata.HeaderOp {
	if len(in) == 0 {
		return nil
	}
	out := make([]dslmetadata.HeaderOp, 0, len(in))
	for _, op := range in {
		out = append(out, dslmetadata.HeaderOp{
			Op:        strings.TrimSpace(op.Op),
			NameExpr:  strings.TrimSpace(op.NameExpr),
			ValueExpr: strings.TrimSpace(op.ValueExpr),
			Patterns:  trimStringSliceForMetadata(op.Patterns),
			Separator: strings.TrimSpace(op.Separator),
		})
	}
	return out
}

func exportRouteTemplate(pathExpr string, queryPairs map[string]string) (string, bool) {
	path, ok := exportExprTemplate(pathExpr)
	if !ok || path == "" || !strings.HasPrefix(path, "/") {
		return "", false
	}
	if len(queryPairs) == 0 {
		return path, true
	}
	keys := make([]string, 0, len(queryPairs))
	for key := range queryPairs {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	sort.Strings(keys)

	query := url.Values{}
	for _, key := range keys {
		value, ok := exportExprTemplate(queryPairs[key])
		if !ok {
			return "", false
		}
		query.Set(key, value)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path, true
}

func exportExprTemplate(expr string) (string, bool) {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return "", false
	}
	switch raw {
	case "$request.model", "$request.model_mapped":
		return "{model}", true
	case "$task.id":
		return "{task.id}", true
	case "$task.upstream_id":
		return "{task.upstream_id}", true
	}
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")")
		parts := splitTopLevelArgs(inner)
		if len(parts) == 0 {
			return "", false
		}
		var builder strings.Builder
		for _, part := range parts {
			rendered, ok := exportExprTemplate(part)
			if !ok {
				return "", false
			}
			builder.WriteString(rendered)
		}
		return builder.String(), true
	}
	if strings.HasPrefix(raw, "template(") && strings.HasSuffix(raw, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "template("), ")")
		parts := splitTopLevelArgs(inner)
		if len(parts) != 1 {
			return "", false
		}
		tmpl, ok := exportExprTemplate(parts[0])
		if !ok {
			return "", false
		}
		return exportTemplatePlaceholders(tmpl), true
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		v, err := strconv.Unquote(raw)
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(v), true
	}
	if strings.Contains(raw, "$") {
		return "", false
	}
	return raw, true
}

func exportTemplatePlaceholders(tmpl string) string {
	replacements := map[string]string{
		"${request.model}":          "{model}",
		"${request.model_mapped}":   "{model}",
		"${credential.project_id}":  "{credential.project_id}",
		"${channel.location}":       "{channel.location}",
		"${oauth.access_token}":     "{oauth.access_token}",
		"${channel.key}":            "{channel.key}",
		"${channel.base_url}":       "{channel.base_url}",
		"${task.id}":                "{task.id}",
		"${task.upstream_id}":       "{task.upstream_id}",
		"${$request.model}":         "{model}",
		"${$request.model_mapped}":  "{model}",
		"${$credential.project_id}": "{credential.project_id}",
		"${$channel.location}":      "{channel.location}",
		"${$task.id}":               "{task.id}",
		"${$task.upstream_id}":      "{task.upstream_id}",
	}
	out := tmpl
	for old, replacement := range replacements {
		out = strings.ReplaceAll(out, old, replacement)
	}
	return out
}

func exportAuthExprValue(expr string) string {
	value := strings.TrimSpace(expr)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return strings.TrimSpace(unquoted)
		}
	}
	return value
}

func exportHeaderName(expr string) string {
	return exportAuthExprValue(expr)
}

func cloneStringMapForMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for rawKey, rawValue := range in {
		key := strings.TrimSpace(rawKey)
		value := strings.TrimSpace(rawValue)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func trimStringSliceForMetadata(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, value := range in {
		if v := strings.TrimSpace(value); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func normalizeMetadataAttributes(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for rawKey, rawValue := range in {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		value := strings.ToLower(strings.TrimSpace(rawValue))
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}
