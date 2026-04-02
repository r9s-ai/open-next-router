package dslconfig

import "strings"

func (cfg UsageExtractConfig) DeclaredFacts() []UsageFact {
	cfg = prepareUsageExtractConfig(cfg)
	return cloneUsageFactsForIntrospection(cfg.facts)
}

func (cfg UsageExtractConfig) BuiltinFacts() []UsageFact {
	cfg = prepareUsageExtractConfig(cfg)
	mode := normalizeUsageMode(cfg.Mode)
	if mode == "" {
		return nil
	}
	set, ok := builtinUsageFactSets[mode]
	if !ok || len(set.facts) == 0 {
		return nil
	}
	return cloneUsageFactsForIntrospection(set.facts)
}

func cloneUsageFactsForIntrospection(facts []usageFactConfig) []UsageFact {
	if len(facts) == 0 {
		return nil
	}
	out := make([]UsageFact, 0, len(facts))
	for _, fact := range facts {
		item := UsageFact{
			Dimension: fact.Dimension,
			Unit:      fact.Unit,
			Source:    fact.Source,
			Fallback:  fact.Fallback,
			Path:      fact.Path,
			CountPath: fact.CountPath,
			SumPath:   fact.SumPath,
			Type:      fact.Type,
			Status:    fact.Status,
		}
		if len(fact.Attrs) > 0 {
			item.Attributes = make(map[string]string, len(fact.Attrs))
			for k, v := range fact.Attrs {
				item.Attributes[k] = v
			}
		}
		out = append(out, item)
	}
	return out
}

func normalizeUsageMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}
