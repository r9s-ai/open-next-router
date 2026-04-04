package dslconfig

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func (cfg UsageExtractConfig) DeclaredFacts() []UsageFact {
	cfg = prepareUsageExtractConfig(cfg)
	return cloneUsageFactsForIntrospection(cfg.facts)
}

func (cfg UsageExtractConfig) BuiltinFacts() []UsageFact {
	_ = cfg
	return nil
}

func (cfg UsageExtractConfig) CompiledFacts(meta *dslmeta.Meta) []UsageFact {
	compiled := compileUsageExtractConfig(meta, cfg)
	return cloneUsageFactsForIntrospection(compiled.facts)
}

func (cfg UsageExtractConfig) CompiledPlan(meta *dslmeta.Meta) UsageExecutionPlan {
	compiled := compileUsageExtractConfig(meta, cfg)
	return UsageExecutionPlan{
		Mode:            compiled.Mode,
		Facts:           cloneUsageFactsForIntrospection(compiled.facts),
		TotalTokensExpr: usageExprString(compiled.TotalTokensExpr),
	}
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
