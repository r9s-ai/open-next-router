package dslconfig

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
	onropenai "github.com/r9s-ai/open-next-router/onr-core/pkg/providerusage/openai"
)

type usageFactConfig struct {
	Dimension string
	Unit      string
	Source    string

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
	quantity float64
	matched  bool
}

type usageFactSet struct {
	facts      []usageFactConfig
	factGroups map[usageFactKey][]usageFactConfig
}

var defaultUsageDimensionRegistry = NewUsageDimensionRegistry(
	UsageDimension{Dimension: "input", Unit: "token"},
	UsageDimension{Dimension: "output", Unit: "token"},
	UsageDimension{Dimension: "cache_read", Unit: "token"},
	UsageDimension{Dimension: "cache_write", Unit: "token"},
	UsageDimension{Dimension: "server_tool.web_search", Unit: "call"},
	UsageDimension{Dimension: "server_tool.file_search", Unit: "call"},
	UsageDimension{Dimension: "image.generate", Unit: "image"},
	UsageDimension{Dimension: "image.edit", Unit: "image"},
	UsageDimension{Dimension: "image.variation", Unit: "image"},
	UsageDimension{Dimension: "audio.tts", Unit: "second"},
	UsageDimension{Dimension: "audio.stt", Unit: "second"},
	UsageDimension{Dimension: "audio.translate", Unit: "second"},
)

var builtinUsageFactSets = map[string]usageFactSet{
	usageModeOpenAI:    newUsageFactSet([]usageFactConfig{{Dimension: "input", Unit: "token", Path: "$.usage.prompt_tokens"}, {Dimension: "input", Unit: "token", Path: "$.usage.input_tokens", Fallback: true}, {Dimension: "output", Unit: "token", Path: "$.usage.completion_tokens"}, {Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Fallback: true}, {Dimension: "cache_read", Unit: "token", Path: "$.usage.prompt_tokens_details.cached_tokens"}, {Dimension: "cache_read", Unit: "token", Path: "$.usage.input_tokens_details.cached_tokens", Fallback: true}, {Dimension: "cache_read", Unit: "token", Path: "$.usage.cached_tokens", Fallback: true}}),
	usageModeAnthropic: newUsageFactSet([]usageFactConfig{{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens"}, {Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"}, {Dimension: "cache_read", Unit: "token", Path: "$.usage.cache_read_input_tokens"}, {Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_5m_input_tokens", Attrs: map[string]string{"ttl": "5m"}}, {Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_1h_input_tokens", Attrs: map[string]string{"ttl": "1h"}}, {Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation_input_tokens", Fallback: true}}),
	usageModeGemini:    newUsageFactSet([]usageFactConfig{{Dimension: "input", Unit: "token", Path: "$.usageMetadata.promptTokenCount"}, {Dimension: "input", Unit: "token", Path: "$.usage_metadata.prompt_token_count", Fallback: true}, {Dimension: "output", Unit: "token", Path: "$.usageMetadata.candidatesTokenCount"}, {Dimension: "output", Unit: "token", Path: "$.usageMetadata.thoughtsTokenCount"}, {Dimension: "output", Unit: "token", Path: "$.usage_metadata.candidates_token_count"}, {Dimension: "output", Unit: "token", Path: "$.usage_metadata.thoughts_token_count"}}),
}

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

func usageFactQuantities(reqRoot, respRoot, derivedRoot map[string]any, facts []usageFactConfig) map[usageFactKey]float64 {
	grouped := groupUsageFactConfigs(facts)
	out := make(map[usageFactKey]float64, len(grouped))
	for key, group := range grouped {
		resolved := evaluateUsageFactGroup(reqRoot, respRoot, derivedRoot, group)
		total := 0.0
		for _, r := range resolved {
			if r.matched {
				total += r.quantity
			}
		}
		out[key] = total
	}
	return out
}

func newUsageFactSet(facts []usageFactConfig) usageFactSet {
	return usageFactSet{
		facts:      facts,
		factGroups: groupUsageFactConfigs(facts),
	}
}

func evaluateUsageFactConfigs(reqRoot, respRoot, derivedRoot map[string]any, facts []usageFactConfig) []usageFactEval {
	return evaluateUsageFactConfigGroups(reqRoot, respRoot, derivedRoot, groupUsageFactConfigs(facts), len(facts))
}

func groupUsageFactConfigs(facts []usageFactConfig) map[usageFactKey][]usageFactConfig {
	out := make(map[usageFactKey][]usageFactConfig)
	for _, fact := range facts {
		key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
		out[key] = append(out[key], fact)
	}
	return out
}

func usageFactExplicitKeys(facts []usageFactConfig) map[usageFactKey]struct{} {
	out := make(map[usageFactKey]struct{}, len(facts))
	for _, fact := range facts {
		out[normalizeUsageFactKey(fact.Dimension, fact.Unit)] = struct{}{}
	}
	return out
}

func evaluateUsageFactConfigGroups(reqRoot, respRoot, derivedRoot map[string]any, grouped map[usageFactKey][]usageFactConfig, totalFacts int) []usageFactEval {
	out := make([]usageFactEval, 0, totalFacts)
	for _, group := range grouped {
		out = append(out, evaluateUsageFactGroup(reqRoot, respRoot, derivedRoot, group)...)
	}
	return out
}

func evaluateUsageFactGroup(reqRoot, respRoot, derivedRoot map[string]any, facts []usageFactConfig) []usageFactEval {
	out := make([]usageFactEval, 0, len(facts))
	var specificMatched bool
	// Ordering rule for the same dimension+unit:
	// - non-fallback rules run first, preserving declaration order
	// - fallback rules only run when no non-fallback rule matched, also preserving declaration order
	for _, fact := range facts {
		if fact.Fallback {
			continue
		}
		q, matched := evaluateUsageFact(reqRoot, respRoot, derivedRoot, fact)
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
		q, matched := evaluateUsageFact(reqRoot, respRoot, derivedRoot, fact)
		out = append(out, usageFactEval{cfg: fact, quantity: q, matched: matched})
	}
	return out
}

func evaluateUsageFact(reqRoot, respRoot, derivedRoot map[string]any, fact usageFactConfig) (quantity float64, matched bool) {
	root := usageFactSourceRoot(reqRoot, respRoot, derivedRoot, fact.Source)
	if len(root) == 0 {
		return 0, false
	}
	switch {
	case fact.Expr != nil:
		return float64(fact.Expr.Eval(root)), true
	case fact.CountPath != "":
		return evaluateUsageFactCountPath(root, fact.CountPath, fact.Type, fact.Status)
	case fact.SumPath != "":
		return jsonutil.GetFloatByPathWithMatch(root, fact.SumPath)
	case fact.Path != "":
		return jsonutil.GetFloatByPathWithMatch(root, fact.Path)
	default:
		return 0, false
	}
}

func usageFactSourceRoot(reqRoot, respRoot, derivedRoot map[string]any, source string) map[string]any {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "response":
		return respRoot
	case "request":
		return reqRoot
	case "derived":
		return derivedRoot
	default:
		return nil
	}
}

func evaluateUsageFactCountPath(root map[string]any, path, typ, status string) (quantity float64, matched bool) {
	count := 0
	if !jsonutil.VisitValuesByPath(root, path, func(v any) {
		if typ == "" && status == "" {
			count++
			return
		}
		if matchesUsageFactFilter(v, typ, status) {
			count++
		}
	}) {
		return 0, false
	}
	return float64(count), true
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
			v := int(math.Round(fact.quantity))
			usage.InputTokens += v
			usage.PromptTokens += v
		case usageFactKey{Dimension: "output", Unit: "token"}:
			v := int(math.Round(fact.quantity))
			usage.OutputTokens += v
			usage.CompletionTokens += v
		case usageFactKey{Dimension: "cache_read", Unit: "token"}:
			cachedTokens += int(math.Round(fact.quantity))
		case usageFactKey{Dimension: "cache_write", Unit: "token"}:
			cacheWriteTokens += int(math.Round(fact.quantity))
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

func extractCustomUsage(reqRoot, respRoot, derivedRoot map[string]any, cfg UsageExtractConfig) (*Usage, int, error) {
	explicitKeys := cfg.explicitFactKeys

	evals := make([]usageFactEval, 0, len(cfg.facts)+4)
	evals = append(evals, evaluateUsageFactConfigGroups(reqRoot, respRoot, derivedRoot, cfg.factGroups, len(cfg.facts))...)

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
	evals = append(evals, evaluateUsageFactConfigs(reqRoot, respRoot, derivedRoot, legacyFacts)...)

	usage, cachedTokens, err := projectUsageFromFacts(evals)
	if err != nil {
		return nil, 0, err
	}
	if cfg.TotalTokensExpr != nil {
		total := cfg.TotalTokensExpr.Eval(respRoot)
		usage.TotalTokens = total
	}
	return usage, cachedTokens, nil
}

func extractBuiltinUsage(meta *dslmeta.Meta, reqRoot, respRoot, derivedRoot map[string]any, respBody []byte, cfg UsageExtractConfig) (*Usage, int, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	explicitKeys := cfg.explicitFactKeys

	baseSet := builtinUsageFactSet(mode)
	evals := make([]usageFactEval, 0, len(baseSet.facts)+len(cfg.facts))
	for key, group := range baseSet.factGroups {
		if _, ok := explicitKeys[key]; ok {
			continue
		}
		evals = append(evals, evaluateUsageFactGroup(reqRoot, respRoot, derivedRoot, group)...)
	}
	evals = append(evals, evaluateUsageFactConfigGroups(reqRoot, respRoot, derivedRoot, cfg.factGroups, len(cfg.facts))...)
	evals = append(evals, builtinSupplementalUsageFactEvals(meta, derivedRoot, respBody, mode, explicitKeys)...)
	usage, cachedTokens, err := projectUsageFromFacts(evals)
	if err != nil {
		return nil, 0, err
	}
	if totalTokens := builtinTotalTokens(respRoot, mode); totalTokens > 0 {
		usage.TotalTokens = totalTokens
	}
	return usage, cachedTokens, nil
}

func builtinSupplementalUsageFactEvals(meta *dslmeta.Meta, derivedRoot map[string]any, respBody []byte, mode string, explicitKeys map[usageFactKey]struct{}) []usageFactEval {
	if strings.ToLower(strings.TrimSpace(mode)) != usageModeOpenAI || meta == nil {
		return nil
	}
	api := strings.ToLower(strings.TrimSpace(meta.API))
	switch api {
	case "images.generations":
		return builtinSupplementalImageUsageFactEvals(respBody, usageFactKey{Dimension: "image.generate", Unit: "image"}, explicitKeys)
	case "images.edits":
		return builtinSupplementalImageUsageFactEvals(respBody, usageFactKey{Dimension: "image.edit", Unit: "image"}, explicitKeys)
	case "audio.transcriptions":
		return builtinSupplementalAudioSecondsUsageFactEvals(respBody, usageFactKey{Dimension: "audio.stt", Unit: "second"}, explicitKeys)
	case "audio.translations":
		return builtinSupplementalAudioSecondsUsageFactEvals(respBody, usageFactKey{Dimension: "audio.translate", Unit: "second"}, explicitKeys)
	case "audio.speech":
		key := usageFactKey{Dimension: "audio.tts", Unit: "second"}
		if _, ok := explicitKeys[key]; ok || len(derivedRoot) == 0 {
			return nil
		}
		quantity, matched := evaluateUsageFact(nil, nil, derivedRoot, usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
			Source:    "derived",
			Path:      "$.audio_duration_seconds",
		})
		if !matched || quantity <= 0 {
			return nil
		}
		return []usageFactEval{{
			cfg: usageFactConfig{
				Dimension: key.Dimension,
				Unit:      key.Unit,
				Source:    "derived",
				Path:      "$.audio_duration_seconds",
			},
			quantity: quantity,
			matched:  true,
		}}
	case "responses":
		return builtinSupplementalWebSearchCallUsageFactEvals(respBody, usageFactKey{Dimension: "server_tool.web_search", Unit: "call"}, explicitKeys)
	default:
		return nil
	}
}

func builtinSupplementalImageUsageFactEvals(respBody []byte, key usageFactKey, explicitKeys map[usageFactKey]struct{}) []usageFactEval {
	if _, ok := explicitKeys[key]; ok {
		return nil
	}
	quantity, ok, err := onropenai.ImageCountFromResponseBody(respBody)
	if err != nil || !ok || quantity <= 0 {
		return nil
	}
	return []usageFactEval{{
		cfg: usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
		},
		quantity: quantity,
		matched:  true,
	}}
}

func builtinSupplementalAudioSecondsUsageFactEvals(respBody []byte, key usageFactKey, explicitKeys map[usageFactKey]struct{}) []usageFactEval {
	if _, ok := explicitKeys[key]; ok {
		return nil
	}
	quantity, ok, err := onropenai.AudioUsageSecondsFromResponseBody(respBody)
	if err != nil || !ok || quantity <= 0 {
		return nil
	}
	return []usageFactEval{{
		cfg: usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
		},
		quantity: quantity,
		matched:  true,
	}}
}

func builtinSupplementalWebSearchCallUsageFactEvals(respBody []byte, key usageFactKey, explicitKeys map[usageFactKey]struct{}) []usageFactEval {
	if _, ok := explicitKeys[key]; ok {
		return nil
	}
	quantity, ok, err := onropenai.CompletedWebSearchCallsFromResponseBody(respBody)
	if err != nil || !ok || quantity <= 0 {
		return nil
	}
	return []usageFactEval{{
		cfg: usageFactConfig{
			Dimension: key.Dimension,
			Unit:      key.Unit,
		},
		quantity: quantity,
		matched:  true,
	}}
}

func builtinUsageFactConfigs(mode string) []usageFactConfig {
	return builtinUsageFactSet(mode).facts
}

func builtinUsageFactSet(mode string) usageFactSet {
	if set, ok := builtinUsageFactSets[strings.ToLower(strings.TrimSpace(mode))]; ok {
		return set
	}
	return usageFactSet{}
}

func builtinTotalTokens(root map[string]any, mode string) int {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case usageModeGemini:
		return jsonutil.GetFirstIntByPaths(root, "$.usageMetadata.totalTokenCount", "$.usage_metadata.total_token_count")
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
	totals := map[string]float64{}
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
		out[key] = normalizeUsageFactFlatFieldValue(totals[key])
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
			Source:    normalizeUsageFactSource(fact.cfg.Source),
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

func normalizeUsageFactSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "response":
		return "response"
	case "request":
		return "request"
	default:
		return strings.ToLower(strings.TrimSpace(source))
	}
}

func normalizeUsageFactFlatFieldValue(v float64) any {
	if math.Mod(v, 1) == 0 {
		return int(v)
	}
	return v
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
	case "image":
		return "images"
	case "second":
		return "seconds"
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
