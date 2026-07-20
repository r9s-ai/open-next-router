package dslconfig

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func (cfg UsageExtractConfig) DeclaredFacts() []UsageFact {
	cfg = prepareUsageExtractConfig(cfg)
	return cloneUsageFactsForIntrospection(cfg.facts, false)
}

func (cfg UsageExtractConfig) DeclaredUsageRoots() []UsageRoot {
	return cloneUsageRootsForIntrospection(cfg.usageRoots)
}

func (cfg UsageExtractConfig) BuiltinFacts() []UsageFact {
	_ = cfg
	return nil
}

func (cfg UsageExtractConfig) CompiledFacts(meta *dslmeta.Meta) []UsageFact {
	compiled := compileUsageExtractConfig(meta, cfg)
	return cloneUsageFactsForIntrospection(compiled.facts, len(compiled.usageRoots) > 0)
}

func (cfg UsageExtractConfig) CompiledPlan(meta *dslmeta.Meta) UsageExecutionPlan {
	compiled := compileUsageExtractConfig(meta, cfg)
	return UsageExecutionPlan{
		Mode:            compiled.Mode,
		UsageRoots:      cloneUsageRootsForIntrospection(compiled.usageRoots),
		Facts:           cloneUsageFactsForIntrospection(compiled.facts, len(compiled.usageRoots) > 0),
		TotalTokensExpr: usageExprString(compiled.TotalTokensExpr),
	}
}

func cloneUsageRootsForIntrospection(roots []usageRootConfig) []UsageRoot {
	if len(roots) == 0 {
		return nil
	}
	out := make([]UsageRoot, 0, len(roots))
	for _, root := range roots {
		item := UsageRoot(root)
		if len(root.ExcludeFields) > 0 {
			item.ExcludeFields = append([]string(nil), root.ExcludeFields...)
		}
		out = append(out, item)
	}
	return out
}

func (p ProviderUsage) CompiledPlans() ProviderUsageExecutionPlan {
	out := ProviderUsageExecutionPlan{
		Defaults: p.Defaults.CompiledPlan(nil),
	}
	if len(p.Matches) == 0 {
		return out
	}
	out.Matches = make([]MatchUsageExecutionPlan, 0, len(p.Matches))
	for _, m := range p.Matches {
		merged := mergeUsageConfig(p.Defaults, m.Extract)
		var meta *dslmeta.Meta
		api := strings.TrimSpace(m.API)
		if api != "" {
			meta = &dslmeta.Meta{API: api}
			if m.Stream != nil {
				meta.IsStream = *m.Stream
			}
		}
		out.Matches = append(out.Matches, MatchUsageExecutionPlan{
			API:    m.API,
			Stream: m.Stream,
			Plan:   merged.CompiledPlan(meta),
		})
	}
	return out
}

func cloneUsageFactsForIntrospection(facts []usageFactConfig, usageRootConfigured bool) []UsageFact {
	if len(facts) == 0 {
		return nil
	}
	out := make([]UsageFact, 0, len(facts))
	for _, fact := range facts {
		item := UsageFact{
			Dimension:     fact.Dimension,
			Unit:          fact.Unit,
			Source:        effectiveUsageFactSource(fact.Source, usageRootConfigured),
			Fallback:      fact.Fallback,
			Event:         fact.Event,
			EventOptional: fact.EventOptional,
			Path:          fact.Path,
			CountPath:     fact.CountPath,
			SumPath:       fact.SumPath,
			LenPath:       fact.LenPath,
			Type:          fact.Type,
			Status:        fact.Status,
			WhenPath:      fact.WhenPath,
			WhenEq:        fact.WhenEq,
			Scale:         fact.Scale,
		}
		if fact.Expr != nil {
			item.Expr = usageExprString(fact.Expr)
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

func usageExprString(expr *UsageExpr) string {
	if expr == nil {
		return ""
	}
	return expr.String()
}
