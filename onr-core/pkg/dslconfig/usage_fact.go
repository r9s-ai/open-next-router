package dslconfig

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

type usageFactConfig struct {
	Dimension string
	Unit      string

	Path      string
	CountPath string
	SumPath   string
	Expr      *UsageExpr

	Type   string
	Status string

	Attrs    map[string]string
	Fallback bool
}

// UsageFactRule describes one usage_fact extraction rule in DSL/runtime.
type UsageFactRule = usageFactConfig

type usageFactKey struct {
	Dimension string
	Unit      string
}

type UsageDimension struct {
	Dimension string
	Unit      string
}

type UsageDimensionRegistry struct {
	allowed map[usageFactKey]struct{}
}

type usageFactEval struct {
	cfg      usageFactConfig
	quantity int
	matched  bool
}

var defaultUsageDimensionRegistry = NewUsageDimensionRegistry(
	UsageDimension{Dimension: "input", Unit: "token"},
	UsageDimension{Dimension: "output", Unit: "token"},
	UsageDimension{Dimension: "cache_read", Unit: "token"},
	UsageDimension{Dimension: "cache_write", Unit: "token"},
	UsageDimension{Dimension: "server_tool.web_search", Unit: "call"},
	UsageDimension{Dimension: "server_tool.file_search", Unit: "call"},
)

func NewUsageDimensionRegistry(keys ...UsageDimension) UsageDimensionRegistry {
	reg := UsageDimensionRegistry{allowed: make(map[usageFactKey]struct{}, len(keys))}
	for _, key := range keys {
		reg.allowed[normalizeUsageFactKey(key.Dimension, key.Unit)] = struct{}{}
	}
	return reg
}

func (r UsageDimensionRegistry) Allows(dim, unit string) bool {
	if len(r.allowed) == 0 {
		return false
	}
	_, ok := r.allowed[normalizeUsageFactKey(dim, unit)]
	return ok
}

func normalizeUsageFactKey(dim, unit string) usageFactKey {
	return usageFactKey{
		Dimension: strings.ToLower(strings.TrimSpace(dim)),
		Unit:      strings.ToLower(strings.TrimSpace(unit)),
	}
}

func usageFactKeyAllowed(dim, unit string) bool {
	return defaultUsageDimensionRegistry.Allows(dim, unit)
}

func usageFactKeyString(k usageFactKey) string {
	return k.Dimension + " " + k.Unit
}

func usageFactValueFromLegacy(cfg UsageExtractConfig, key usageFactKey) (usageFactConfig, bool) {
	if strings.ToLower(strings.TrimSpace(cfg.Mode)) != usageModeCustom {
		return usageFactConfig{}, false
	}
	switch key {
	case usageFactKey{Dimension: "input", Unit: "token"}:
		return usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
			Path:      cfg.InputTokensPath,
			Expr:      cfg.InputTokensExpr,
		}, true
	case usageFactKey{Dimension: "output", Unit: "token"}:
		return usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
			Path:      cfg.OutputTokensPath,
			Expr:      cfg.OutputTokensExpr,
		}, true
	case usageFactKey{Dimension: "cache_read", Unit: "token"}:
		return usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
			Path:      cfg.CacheReadTokensPath,
			Expr:      cfg.CacheReadTokensExpr,
		}, true
	case usageFactKey{Dimension: "cache_write", Unit: "token"}:
		return usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
			Path:      cfg.CacheWriteTokensPath,
			Expr:      cfg.CacheWriteTokensExpr,
		}, true
	default:
		return usageFactConfig{}, false
	}
}

func usageFactQuantities(root map[string]any, facts []usageFactConfig) map[usageFactKey]int {
	grouped := groupUsageFactConfigs(facts)
	out := make(map[usageFactKey]int, len(grouped))
	for key, group := range grouped {
		resolved := evaluateUsageFactGroup(root, group)
		total := 0
		for _, r := range resolved {
			if r.matched {
				total += r.quantity
			}
		}
		out[key] = total
	}
	return out
}

func evaluateUsageFactConfigs(root map[string]any, facts []usageFactConfig) []usageFactEval {
	grouped := groupUsageFactConfigs(facts)
	out := make([]usageFactEval, 0, len(facts))
	for _, group := range grouped {
		out = append(out, evaluateUsageFactGroup(root, group)...)
	}
	return out
}

func groupUsageFactConfigs(facts []usageFactConfig) map[usageFactKey][]usageFactConfig {
	out := make(map[usageFactKey][]usageFactConfig)
	for _, fact := range facts {
		key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
		out[key] = append(out[key], fact)
	}
	return out
}

func evaluateUsageFactGroup(root map[string]any, facts []usageFactConfig) []usageFactEval {
	out := make([]usageFactEval, 0, len(facts))
	var specificMatched bool
	// Ordering rule for the same dimension+unit:
	// - non-fallback rules run first, preserving declaration order
	// - fallback rules only run when no non-fallback rule matched, also preserving declaration order
	for _, fact := range facts {
		if fact.Fallback {
			continue
		}
		q, matched := evaluateUsageFact(root, fact)
		if matched {
			specificMatched = true
		}
		out = append(out, usageFactEval{cfg: fact, quantity: q, matched: matched})
	}
	if specificMatched {
		return out
	}
	for _, fact := range facts {
		if !fact.Fallback {
			continue
		}
		q, matched := evaluateUsageFact(root, fact)
		out = append(out, usageFactEval{cfg: fact, quantity: q, matched: matched})
	}
	return out
}

func evaluateUsageFact(root map[string]any, fact usageFactConfig) (quantity int, matched bool) {
	if len(root) == 0 {
		return 0, false
	}
	switch {
	case fact.Expr != nil:
		return fact.Expr.Eval(root), true
	case fact.CountPath != "":
		return evaluateUsageFactCountPath(root, fact.CountPath, fact.Type, fact.Status)
	case fact.SumPath != "":
		_, matched = jsonutil.GetValuesByPath(root, fact.SumPath)
		return jsonutil.GetIntByPath(root, fact.SumPath), matched
	case fact.Path != "":
		_, matched = jsonutil.GetValuesByPath(root, fact.Path)
		return jsonutil.GetIntByPath(root, fact.Path), matched
	default:
		return 0, false
	}
}

func evaluateUsageFactCountPath(root map[string]any, path, typ, status string) (quantity int, matched bool) {
	vals, ok := jsonutil.GetValuesByPath(root, path)
	if !ok {
		return 0, false
	}
	matched = true
	if len(vals) == 0 {
		return 0, true
	}
	if typ == "" && status == "" {
		return len(vals), true
	}
	count := 0
	for _, v := range vals {
		if matchesUsageFactFilter(v, typ, status) {
			count++
		}
	}
	return count, true
}

func matchesUsageFactFilter(v any, typ, status string) bool {
	if typ == "" && status == "" {
		return true
	}
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	if typ != "" && !strings.EqualFold(strings.TrimSpace(jsonutil.CoerceString(m["type"])), strings.TrimSpace(typ)) {
		return false
	}
	if status != "" && !strings.EqualFold(strings.TrimSpace(jsonutil.CoerceString(m["status"])), strings.TrimSpace(status)) {
		return false
	}
	return true
}

func projectUsageFromFacts(facts []usageFactEval) (*Usage, int, error) {
	usage := &Usage{}
	var cachedTokens int
	var cacheWriteTokens int
	for _, fact := range facts {
		if !fact.matched {
			continue
		}
		key := normalizeUsageFactKey(fact.cfg.Dimension, fact.cfg.Unit)
		switch key {
		case usageFactKey{Dimension: "input", Unit: "token"}:
			usage.InputTokens += fact.quantity
			usage.PromptTokens += fact.quantity
		case usageFactKey{Dimension: "output", Unit: "token"}:
			usage.OutputTokens += fact.quantity
			usage.CompletionTokens += fact.quantity
		case usageFactKey{Dimension: "cache_read", Unit: "token"}:
			cachedTokens += fact.quantity
		case usageFactKey{Dimension: "cache_write", Unit: "token"}:
			cacheWriteTokens += fact.quantity
		}
	}

	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	if cachedTokens > 0 || cacheWriteTokens > 0 {
		usage.InputTokenDetails = &ResponseTokenDetails{
			CachedTokens:     cachedTokens,
			CacheWriteTokens: cacheWriteTokens,
		}
	}
	usage.FlatFields = buildUsageFlatFields(facts)
	usage.DebugFacts = buildUsageDebugFacts(facts)
	return usage, cachedTokens, nil
}

func extractCustomUsage(root map[string]any, cfg UsageExtractConfig) (*Usage, int, error) {
	explicitKeys := map[usageFactKey]struct{}{}
	for _, fact := range cfg.facts {
		explicitKeys[normalizeUsageFactKey(fact.Dimension, fact.Unit)] = struct{}{}
	}

	evals := make([]usageFactEval, 0, len(cfg.facts)+4)
	evals = append(evals, evaluateUsageFactConfigs(root, cfg.facts)...)

	legacyCandidates := []usageFactKey{
		{Dimension: "input", Unit: "token"},
		{Dimension: "output", Unit: "token"},
		{Dimension: "cache_read", Unit: "token"},
		{Dimension: "cache_write", Unit: "token"},
	}
	legacyFacts := make([]usageFactConfig, 0, len(legacyCandidates))
	for _, key := range legacyCandidates {
		if _, ok := explicitKeys[key]; ok {
			continue
		}
		fact, ok := usageFactValueFromLegacy(cfg, key)
		if !ok {
			continue
		}
		if strings.TrimSpace(fact.Path) == "" && fact.Expr == nil && strings.TrimSpace(fact.CountPath) == "" && strings.TrimSpace(fact.SumPath) == "" {
			continue
		}
		legacyFacts = append(legacyFacts, fact)
	}
	evals = append(evals, evaluateUsageFactConfigs(root, legacyFacts)...)

	usage, cachedTokens, err := projectUsageFromFacts(evals)
	if err != nil {
		return nil, 0, err
	}
	if cfg.TotalTokensExpr != nil {
		total := cfg.TotalTokensExpr.Eval(root)
		usage.TotalTokens = total
	}
	return usage, cachedTokens, nil
}

func extractBuiltinUsage(root map[string]any, cfg UsageExtractConfig) (*Usage, int, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	explicitKeys := map[usageFactKey]struct{}{}
	for _, fact := range cfg.facts {
		explicitKeys[normalizeUsageFactKey(fact.Dimension, fact.Unit)] = struct{}{}
	}

	baseFacts := builtinUsageFactConfigs(mode)
	facts := make([]usageFactConfig, 0, len(baseFacts)+len(cfg.facts))
	for _, fact := range baseFacts {
		if _, ok := explicitKeys[normalizeUsageFactKey(fact.Dimension, fact.Unit)]; ok {
			continue
		}
		facts = append(facts, fact)
	}
	facts = append(facts, cfg.facts...)

	evals := evaluateUsageFactConfigs(root, facts)
	usage, cachedTokens, err := projectUsageFromFacts(evals)
	if err != nil {
		return nil, 0, err
	}
	if totalTokens := builtinTotalTokens(root, mode); totalTokens > 0 {
		usage.TotalTokens = totalTokens
	}
	return usage, cachedTokens, nil
}

func builtinUsageFactConfigs(mode string) []usageFactConfig {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case usageModeOpenAI:
		return []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.usage.prompt_tokens"},
			{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens", Fallback: true},
			{Dimension: "output", Unit: "token", Path: "$.usage.completion_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Fallback: true},
			{Dimension: "cache_read", Unit: "token", Path: "$.usage.prompt_tokens_details.cached_tokens"},
			{Dimension: "cache_read", Unit: "token", Path: "$.usage.input_tokens_details.cached_tokens", Fallback: true},
			{Dimension: "cache_read", Unit: "token", Path: "$.usage.cached_tokens", Fallback: true},
		}
	case usageModeAnthropic:
		return []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"},
			{Dimension: "cache_read", Unit: "token", Path: "$.usage.cache_read_input_tokens"},
			{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_5m_input_tokens", Attrs: map[string]string{"ttl": "5m"}},
			{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_1h_input_tokens", Attrs: map[string]string{"ttl": "1h"}},
			{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation_input_tokens", Fallback: true},
		}
	case usageModeGemini:
		return []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.usageMetadata.promptTokenCount"},
			{Dimension: "input", Unit: "token", Path: "$.usage_metadata.prompt_token_count", Fallback: true},
			{Dimension: "output", Unit: "token", Path: "$.usageMetadata.candidatesTokenCount"},
			{Dimension: "output", Unit: "token", Path: "$.usageMetadata.thoughtsTokenCount"},
			{Dimension: "output", Unit: "token", Path: "$.usage_metadata.candidates_token_count"},
			{Dimension: "output", Unit: "token", Path: "$.usage_metadata.thoughts_token_count"},
		}
	default:
		return nil
	}
}

func builtinTotalTokens(root map[string]any, mode string) int {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case usageModeGemini:
		return jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usageMetadata.totalTokenCount"),
			jsonutil.GetIntByPath(root, "$.usage_metadata.total_token_count"),
		)
	default:
		return 0
	}
}

func appendUsageFactErrorPrefix(err error, fact usageFactConfig) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("usage_fact %s %s: %w", strings.TrimSpace(fact.Dimension), strings.TrimSpace(fact.Unit), err)
}

func buildUsageFlatFields(facts []usageFactEval) map[string]any {
	totals := map[string]int{}
	matched := map[string]struct{}{}
	for _, fact := range facts {
		if !fact.matched {
			continue
		}
		key, ok := usageFactFlatFieldKey(fact.cfg)
		if !ok {
			continue
		}
		totals[key] += fact.quantity
		matched[key] = struct{}{}
	}
	if len(matched) == 0 {
		return nil
	}
	out := make(map[string]any, len(matched))
	for key := range matched {
		out[key] = totals[key]
	}
	return out
}

func buildUsageDebugFacts(facts []usageFactEval) []UsageFact {
	out := make([]UsageFact, 0, len(facts))
	for _, fact := range facts {
		if !fact.matched {
			continue
		}
		item := UsageFact{
			Dimension: fact.cfg.Dimension,
			Unit:      fact.cfg.Unit,
			Quantity:  fact.quantity,
			Fallback:  fact.cfg.Fallback,
			Path:      fact.cfg.Path,
			CountPath: fact.cfg.CountPath,
			SumPath:   fact.cfg.SumPath,
			Type:      fact.cfg.Type,
			Status:    fact.cfg.Status,
		}
		if len(fact.cfg.Attrs) > 0 {
			attrs := make(map[string]string, len(fact.cfg.Attrs))
			for k, v := range fact.cfg.Attrs {
				attrs[k] = v
			}
			item.Attributes = attrs
		}
		if fact.cfg.Expr != nil {
			item.Expr = "<expr>"
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func usageFactFlatFieldKey(fact usageFactConfig) (string, bool) {
	key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
	if isLegacyUsageFactKey(key) && len(fact.Attrs) == 0 {
		return "", false
	}

	parts := []string{sanitizeUsageFactNamePart(key.Dimension)}
	attrKeys := make([]string, 0, len(fact.Attrs))
	for k := range fact.Attrs {
		attrKeys = append(attrKeys, k)
	}
	sort.Strings(attrKeys)
	for _, k := range attrKeys {
		v := fact.Attrs[k]
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		parts = append(parts, sanitizeUsageFactNamePart(k), sanitizeUsageFactNamePart(v))
	}
	if unit := usageFactFlatFieldUnitSuffix(key.Unit); unit != "" {
		parts = append(parts, unit)
	}
	name := strings.Join(compactUsageFactNameParts(parts), "_")
	if name == "" {
		return "", false
	}
	return name, true
}

func isLegacyUsageFactKey(key usageFactKey) bool {
	switch key {
	case usageFactKey{Dimension: "input", Unit: "token"}:
		return true
	case usageFactKey{Dimension: "output", Unit: "token"}:
		return true
	case usageFactKey{Dimension: "cache_read", Unit: "token"}:
		return true
	case usageFactKey{Dimension: "cache_write", Unit: "token"}:
		return true
	default:
		return false
	}
}

func usageFactFlatFieldUnitSuffix(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "token":
		return "tokens"
	case "call":
		return "calls"
	default:
		return sanitizeUsageFactNamePart(unit)
	}
}

func sanitizeUsageFactNamePart(s string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func compactUsageFactNameParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
