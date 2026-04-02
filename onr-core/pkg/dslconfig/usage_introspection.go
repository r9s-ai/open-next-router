package dslconfig

func (cfg UsageExtractConfig) DeclaredFacts() []UsageFact {
	cfg = prepareUsageExtractConfig(cfg)
	if len(cfg.facts) == 0 {
		return nil
	}

	out := make([]UsageFact, 0, len(cfg.facts))
	for _, fact := range cfg.facts {
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
