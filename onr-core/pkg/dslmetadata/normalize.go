package dslmetadata

import "strings"

func NormalizeProviderConfig(cfg ProviderConfig) ProviderConfig {
	out := ProviderConfig{
		Routes: normalizeRoutes(cfg.Routes),
	}
	if cfg.Metadata != nil {
		metadata := ProviderMetadata{
			ProviderFamily: strings.ToLower(strings.TrimSpace(cfg.Metadata.ProviderFamily)),
			SignalProfile:  strings.ToLower(strings.TrimSpace(cfg.Metadata.SignalProfile)),
		}
		if metadata.ProviderFamily != "" || metadata.SignalProfile != "" {
			out.Metadata = &metadata
		}
	}
	if cfg.Auth != nil {
		auth := ProviderAuth{
			Type:     strings.ToLower(strings.TrimSpace(cfg.Auth.Type)),
			Header:   strings.TrimSpace(cfg.Auth.Header),
			Mode:     strings.ToLower(strings.TrimSpace(cfg.Auth.Mode)),
			Scope:    strings.TrimSpace(cfg.Auth.Scope),
			TokenURL: strings.TrimSpace(cfg.Auth.TokenURL),
		}
		if auth.Type != "" || auth.Header != "" || auth.Mode != "" || auth.Scope != "" || auth.TokenURL != "" {
			out.Auth = &auth
		}
	}
	if cfg.Request != nil {
		req := normalizeProviderRequest(*cfg.Request)
		if providerRequestHasRules(req) {
			out.Request = &req
		}
	}
	if cfg.Models != nil {
		models := normalizeModels(*cfg.Models)
		if models.Mode != "" {
			out.Models = &models
		}
	}
	if cfg.Balance != nil {
		balance := normalizeBalance(*cfg.Balance)
		if balance.Mode != "" {
			out.Balance = &balance
		}
	}
	if cfg.UsageFacts != nil {
		usageFacts := normalizeUsageFacts(*cfg.UsageFacts)
		out.UsageFacts = &usageFacts
	}
	return out
}

func normalizeRoutes(in []ProviderRoute) []ProviderRoute {
	if len(in) == 0 {
		return nil
	}
	out := make([]ProviderRoute, 0, len(in))
	for _, route := range in {
		api := strings.ToLower(strings.TrimSpace(route.API))
		path := strings.TrimSpace(route.Path)
		if api == "" || path == "" {
			continue
		}
		var streamPtr *bool
		if route.Stream != nil {
			v := *route.Stream
			streamPtr = &v
		}
		out = append(out, ProviderRoute{
			API:    api,
			Stream: streamPtr,
			Path:   path,
		})
	}
	return out
}

func normalizeProviderRequest(in ProviderRequest) ProviderRequest {
	out := ProviderRequest{
		Defaults: normalizeRequestTransform(in.Defaults),
	}
	for _, match := range in.Matches {
		api := strings.ToLower(strings.TrimSpace(match.API))
		transform := normalizeRequestTransform(match.Transform)
		if !requestTransformHasRules(transform) {
			continue
		}
		var streamPtr *bool
		if match.Stream != nil {
			v := *match.Stream
			streamPtr = &v
		}
		out.Matches = append(out.Matches, RequestTransformMatch{
			API:       api,
			Stream:    streamPtr,
			Transform: transform,
		})
	}
	return out
}

func normalizeRequestTransform(in RequestTransform) RequestTransform {
	return RequestTransform{
		ModelMap: ModelMap{
			Map:         normalizeStringMap(in.ModelMap.Map),
			DefaultExpr: strings.TrimSpace(in.ModelMap.DefaultExpr),
		},
		JSONOps:            normalizeJSONOps(in.JSONOps),
		AfterReqMapJSONOps: normalizeJSONOps(in.AfterReqMapJSONOps),
		ReqMapMode:         strings.TrimSpace(in.ReqMapMode),
	}
}

func normalizeJSONOps(in []JSONOp) []JSONOp {
	if len(in) == 0 {
		return nil
	}
	out := make([]JSONOp, 0, len(in))
	for _, op := range in {
		item := JSONOp{
			Op:         strings.TrimSpace(op.Op),
			Path:       strings.TrimSpace(op.Path),
			FromPath:   strings.TrimSpace(op.FromPath),
			ToPath:     strings.TrimSpace(op.ToPath),
			ValueExpr:  strings.TrimSpace(op.ValueExpr),
			HeaderName: strings.TrimSpace(op.HeaderName),
			FieldName:  strings.TrimSpace(op.FieldName),
			Patterns:   trimStringSlice(op.Patterns),
			Separator:  strings.TrimSpace(op.Separator),
			Event:      strings.TrimSpace(op.Event),
			MaxCount:   op.MaxCount,
		}
		if item.Op == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizeModels(in ProviderModels) ProviderModels {
	out := in
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	out.Method = strings.TrimSpace(out.Method)
	out.Path = strings.TrimSpace(out.Path)
	out.IDPaths = trimStringSlice(out.IDPaths)
	out.IDRegex = strings.TrimSpace(out.IDRegex)
	out.IDAllowRegex = strings.TrimSpace(out.IDAllowRegex)
	out.Headers = normalizeHeaderOps(out.Headers)
	if out.Method == "" {
		out.Method = "GET"
	}
	switch out.Mode {
	case "openai":
		if out.Path == "" {
			out.Path = "/v1/models"
		}
		if len(out.IDPaths) == 0 {
			out.IDPaths = []string{"$.data[*].id"}
		}
	case "gemini":
		if out.Path == "" {
			out.Path = "/v1beta/models"
		}
		if len(out.IDPaths) == 0 {
			out.IDPaths = []string{"$.models[*].name"}
		}
		if out.IDRegex == "" {
			out.IDRegex = "^models/(.+)$"
		}
	}
	if out.Mode == "" && hasAnyModelsRule(out) {
		out.Mode = "custom"
	}
	return out
}

func normalizeBalance(in ProviderBalance) ProviderBalance {
	out := in
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	out.Method = strings.TrimSpace(out.Method)
	out.Path = strings.TrimSpace(out.Path)
	out.BalancePath = strings.TrimSpace(out.BalancePath)
	out.BalanceExpr = strings.TrimSpace(out.BalanceExpr)
	out.UsedPath = strings.TrimSpace(out.UsedPath)
	out.UsedExpr = strings.TrimSpace(out.UsedExpr)
	out.Unit = strings.TrimSpace(out.Unit)
	out.SubscriptionPath = strings.TrimSpace(out.SubscriptionPath)
	out.UsagePath = strings.TrimSpace(out.UsagePath)
	out.Headers = normalizeHeaderOps(out.Headers)
	if out.Method == "" {
		out.Method = "GET"
	}
	if out.Mode == "" && hasAnyBalanceRule(out) {
		out.Mode = "custom"
	}
	return out
}

func normalizeHeaderOps(in []HeaderOp) []HeaderOp {
	if len(in) == 0 {
		return nil
	}
	out := make([]HeaderOp, 0, len(in))
	for _, op := range in {
		item := HeaderOp{
			Op:        strings.TrimSpace(op.Op),
			NameExpr:  strings.TrimSpace(op.NameExpr),
			ValueExpr: strings.TrimSpace(op.ValueExpr),
			Patterns:  trimStringSlice(op.Patterns),
			Separator: strings.TrimSpace(op.Separator),
		}
		if item.Op == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizeUsageFacts(in ProviderUsageFacts) ProviderUsageFacts {
	return ProviderUsageFacts{
		Defaults: normalizeUsageFactList(in.Defaults),
		Matches:  normalizeUsageFactMatches(in.Matches),
	}
}

func normalizeUsageFactMatches(in []UsageFactMatch) []UsageFactMatch {
	if len(in) == 0 {
		return nil
	}
	out := make([]UsageFactMatch, 0, len(in))
	for _, match := range in {
		api := strings.ToLower(strings.TrimSpace(match.API))
		facts := normalizeUsageFactList(match.Facts)
		if len(facts) == 0 {
			continue
		}
		var streamPtr *bool
		if match.Stream != nil {
			v := *match.Stream
			streamPtr = &v
		}
		out = append(out, UsageFactMatch{
			API:    api,
			Stream: streamPtr,
			Facts:  facts,
		})
	}
	return out
}

func normalizeUsageFactList(in []UsageFact) []UsageFact {
	if len(in) == 0 {
		return nil
	}
	out := make([]UsageFact, 0, len(in))
	for _, fact := range in {
		dimension := strings.ToLower(strings.TrimSpace(fact.Dimension))
		unit := strings.ToLower(strings.TrimSpace(fact.Unit))
		if dimension == "" || unit == "" {
			continue
		}
		out = append(out, UsageFact{
			Dimension:  dimension,
			Unit:       unit,
			Source:     strings.ToLower(strings.TrimSpace(fact.Source)),
			Path:       strings.TrimSpace(fact.Path),
			CountPath:  strings.TrimSpace(fact.CountPath),
			SumPath:    strings.TrimSpace(fact.SumPath),
			Expr:       strings.TrimSpace(fact.Expr),
			Type:       strings.TrimSpace(fact.Type),
			Status:     strings.TrimSpace(fact.Status),
			Event:      strings.TrimSpace(fact.Event),
			Fallback:   fact.Fallback,
			Attributes: normalizeAttributes(fact.Attributes),
		})
	}
	return out
}

func normalizeStringMap(in map[string]string) map[string]string {
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
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeAttributes(in map[string]string) map[string]string {
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
	if len(out) == 0 {
		return nil
	}
	return out
}

func trimStringSlice(in []string) []string {
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

func providerRequestHasRules(cfg ProviderRequest) bool {
	if requestTransformHasRules(cfg.Defaults) {
		return true
	}
	for _, match := range cfg.Matches {
		if requestTransformHasRules(match.Transform) {
			return true
		}
	}
	return false
}

func requestTransformHasRules(t RequestTransform) bool {
	return len(t.ModelMap.Map) > 0 ||
		strings.TrimSpace(t.ModelMap.DefaultExpr) != "" ||
		len(t.JSONOps) > 0 ||
		len(t.AfterReqMapJSONOps) > 0 ||
		strings.TrimSpace(t.ReqMapMode) != ""
}

func hasAnyModelsRule(cfg ProviderModels) bool {
	return strings.TrimSpace(cfg.Path) != "" ||
		len(cfg.IDPaths) > 0 ||
		strings.TrimSpace(cfg.IDRegex) != "" ||
		strings.TrimSpace(cfg.IDAllowRegex) != "" ||
		len(cfg.Headers) > 0
}

func hasAnyBalanceRule(cfg ProviderBalance) bool {
	return strings.TrimSpace(cfg.Path) != "" ||
		strings.TrimSpace(cfg.BalancePath) != "" ||
		strings.TrimSpace(cfg.BalanceExpr) != "" ||
		strings.TrimSpace(cfg.UsedPath) != "" ||
		strings.TrimSpace(cfg.UsedExpr) != "" ||
		strings.TrimSpace(cfg.Unit) != "" ||
		strings.TrimSpace(cfg.SubscriptionPath) != "" ||
		strings.TrimSpace(cfg.UsagePath) != "" ||
		len(cfg.Headers) > 0
}
