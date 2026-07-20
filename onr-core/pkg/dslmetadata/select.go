package dslmetadata

import "strings"

func SelectRoute(routes []ProviderRoute, api string, stream bool) (ProviderRoute, bool) {
	api = strings.ToLower(strings.TrimSpace(api))
	if api == "" {
		return ProviderRoute{}, false
	}
	for _, route := range routes {
		if strings.ToLower(strings.TrimSpace(route.API)) != api {
			continue
		}
		if route.Stream != nil && *route.Stream != stream {
			continue
		}
		path := strings.TrimSpace(route.Path)
		if path == "" {
			return ProviderRoute{}, false
		}
		out := route
		out.API = strings.ToLower(strings.TrimSpace(out.API))
		out.Path = path
		if route.Stream != nil {
			v := *route.Stream
			out.Stream = &v
		}
		return out, true
	}
	return ProviderRoute{}, false
}

func SelectRequestTransform(cfg *ProviderRequest, api string, stream bool) (RequestTransform, bool) {
	api = strings.ToLower(strings.TrimSpace(api))
	if cfg == nil || api == "" {
		return RequestTransform{}, false
	}
	defaults := normalizeRequestTransform(cfg.Defaults)
	for _, match := range cfg.Matches {
		matchAPI := strings.ToLower(strings.TrimSpace(match.API))
		if matchAPI != "" && matchAPI != api {
			continue
		}
		if match.Stream != nil && *match.Stream != stream {
			continue
		}
		transform := MergeRequestTransform(defaults, match.Transform)
		if !requestTransformHasRules(transform) {
			return RequestTransform{}, false
		}
		return transform, true
	}
	if !requestTransformHasRules(defaults) {
		return RequestTransform{}, false
	}
	return defaults, true
}

func SelectUsageFacts(cfg *ProviderUsageFacts, api string, stream bool) ([]UsageFact, bool) {
	api = strings.ToLower(strings.TrimSpace(api))
	if cfg == nil || api == "" {
		return nil, false
	}
	defaults := normalizeUsageFactList(cfg.Defaults)
	for _, match := range cfg.Matches {
		matchAPI := strings.ToLower(strings.TrimSpace(match.API))
		if matchAPI != "" && matchAPI != api {
			continue
		}
		if match.Stream != nil && *match.Stream != stream {
			continue
		}
		facts := append(cloneUsageFacts(defaults), normalizeUsageFactList(match.Facts)...)
		if len(facts) == 0 {
			return nil, false
		}
		return facts, true
	}
	if len(defaults) == 0 {
		return nil, false
	}
	return defaults, true
}

func MergeRequestTransform(defaults, override RequestTransform) RequestTransform {
	base := normalizeRequestTransform(defaults)
	over := normalizeRequestTransform(override)
	out := base
	if len(base.ModelMap.Map) > 0 {
		out.ModelMap.Map = cloneStringMap(base.ModelMap.Map)
	}
	if len(base.JSONOps) > 0 {
		out.JSONOps = append([]JSONOp(nil), base.JSONOps...)
	}
	if len(base.ValidationRules) > 0 {
		out.ValidationRules = append([]RequestValidationRule(nil), base.ValidationRules...)
	}
	if len(base.AfterReqMapJSONOps) > 0 {
		out.AfterReqMapJSONOps = append([]JSONOp(nil), base.AfterReqMapJSONOps...)
	}
	if len(over.ModelMap.Map) > 0 {
		if out.ModelMap.Map == nil {
			out.ModelMap.Map = map[string]string{}
		}
		for k, v := range over.ModelMap.Map {
			out.ModelMap.Map[k] = v
		}
	}
	if strings.TrimSpace(over.ModelMap.DefaultExpr) != "" {
		out.ModelMap.DefaultExpr = strings.TrimSpace(over.ModelMap.DefaultExpr)
	}
	if len(over.JSONOps) > 0 {
		out.JSONOps = append(out.JSONOps, over.JSONOps...)
	}
	if len(over.ValidationRules) > 0 {
		out.ValidationRules = append(out.ValidationRules, over.ValidationRules...)
	}
	if len(over.AfterReqMapJSONOps) > 0 {
		out.AfterReqMapJSONOps = append(out.AfterReqMapJSONOps, over.AfterReqMapJSONOps...)
	}
	if strings.TrimSpace(over.ReqMapMode) != "" {
		out.ReqMapMode = strings.TrimSpace(over.ReqMapMode)
	}
	return normalizeRequestTransform(out)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneUsageFacts(in []UsageFact) []UsageFact {
	if len(in) == 0 {
		return nil
	}
	out := make([]UsageFact, 0, len(in))
	for _, fact := range in {
		item := fact
		if len(fact.Attributes) > 0 {
			item.Attributes = cloneStringMap(fact.Attributes)
		}
		out = append(out, item)
	}
	return out
}
