package dslconfig

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// StreamMetricsAggregator aggregates best-effort metrics from SSE "data:" JSON payloads.
//
// Semantics (aligned with next-router requirements):
//   - usage:
//   - For Anthropic SSE, token fields may appear in different events; we keep the latest positive
//     value per field and build a final usage snapshot.
//   - For other modes (openai/gemini/custom), we merge per-field as well: a 0 value in a later
//     event is treated as "missing" and MUST NOT overwrite a previously known positive value.
//   - finish_reason: take the first non-empty finish_reason.
//
// This is a pure pkg utility (no HTTP/Gin dependency) and can be shared by ONR and next-router.
type StreamMetricsAggregator struct {
	meta      *dslmeta.Meta
	usageCfg  UsageExtractConfig
	finishCfg FinishReasonExtractConfig
	extract   streamUsageExtractFunc

	// merged usage snapshot across SSE events
	lastUsage        *Usage
	lastCachedTokens int

	// finish reason (first non-empty)
	finishReason string
}

type streamUsageExtractFunc func(respRoot map[string]any, respBody []byte) (*Usage, int, error)

func NewStreamMetricsAggregator(meta *dslmeta.Meta, usageCfg UsageExtractConfig, finishCfg FinishReasonExtractConfig) *StreamMetricsAggregator {
	usageCfg = prepareUsageExtractConfig(usageCfg)
	return &StreamMetricsAggregator{
		meta:      meta,
		usageCfg:  usageCfg,
		finishCfg: finishCfg,
		extract:   newStreamUsageExtractFunc(meta, usageCfg),
	}
}

func (a *StreamMetricsAggregator) OnSSEDataJSON(payload []byte) error {
	if a == nil {
		return nil
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil
	}

	var obj any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return nil
	}

	// finish_reason: first non-empty
	if strings.TrimSpace(a.finishReason) == "" && (strings.TrimSpace(a.finishCfg.Mode) != "" || a.finishCfg.hasFinishReasonPath()) {
		if v, err := extractFinishReasonFromRoot(a.meta, a.finishCfg, root); err == nil {
			if s := strings.TrimSpace(v); s != "" {
				a.finishReason = s
			}
		}
	}

	mode := strings.ToLower(strings.TrimSpace(a.usageCfg.Mode))
	if mode == "" {
		return nil
	}

	usageRoot := root
	if mode == usageModeAnthropic {
		usageRoot = normalizeAnthropicStreamUsageRoot(root)
	}

	u, cachedTokens, err := a.extract(usageRoot, payload)
	if err != nil {
		// ignore individual event errors
		return nil
	}
	if u == nil || isAllZeroUsage(u) {
		return nil
	}
	// Merge semantics: do not allow 0 to overwrite a previously known value.
	if a.lastUsage == nil {
		a.lastUsage = u
	} else {
		mergeUsagePreferNonZero(a.lastUsage, u)
	}
	if cachedTokens > 0 {
		a.lastCachedTokens = cachedTokens
	}
	// For anthropic/custom split-stream usage, recompute total from the merged fields unless
	// custom mode explicitly provides TotalTokensExpr.
	if shouldRecomputeMergedTotal(strings.TrimSpace(a.usageCfg.Mode), a.usageCfg) && a.lastUsage != nil {
		normalizeUsageFields(a.lastUsage)
		a.lastUsage.TotalTokens = a.lastUsage.InputTokens + a.lastUsage.OutputTokens
	}
	return nil
}

// OnSSETail parses a text/event-stream buffer and feeds each "data:" JSON payload into the aggregator.
func (a *StreamMetricsAggregator) OnSSETail(sse []byte) {
	if a == nil {
		return
	}
	lines := bytes.Split(sse, []byte{'\n'})
	var curData [][]byte
	flush := func() {
		if len(curData) == 0 {
			return
		}
		payload := bytes.TrimSpace(bytes.Join(curData, []byte{'\n'}))
		curData = curData[:0]
		_ = a.OnSSEDataJSON(payload)
	}
	for _, raw := range lines {
		line := bytes.TrimRight(raw, "\r")
		if len(bytes.TrimSpace(line)) == 0 {
			flush()
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			curData = append(curData, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
		}
	}
	flush()
}

// Result returns aggregated metrics.
//
// - usageOk: true when non-zero upstream usage is available.
// - cachedTokens: only meaningful when usageOk is true.
func (a *StreamMetricsAggregator) Result() (usage *Usage, cachedTokens int, finishReason string, usageOk bool) {
	if a == nil {
		return nil, 0, "", false
	}
	finishReason = strings.TrimSpace(a.finishReason)

	mode := strings.ToLower(strings.TrimSpace(a.usageCfg.Mode))
	if a.lastUsage == nil || isAllZeroUsage(a.lastUsage) {
		return nil, 0, finishReason, false
	}
	normalizeUsageFields(a.lastUsage)
	if shouldRecomputeMergedTotal(mode, a.usageCfg) {
		a.lastUsage.TotalTokens = a.lastUsage.InputTokens + a.lastUsage.OutputTokens
	}
	return a.lastUsage, a.lastCachedTokens, finishReason, true
}

func isAllZeroUsage(u *Usage) bool {
	if u == nil {
		return true
	}
	if u.InputTokens != 0 || u.OutputTokens != 0 || u.TotalTokens != 0 {
		return false
	}
	if u.InputTokenDetails != nil && (u.InputTokenDetails.CachedTokens != 0 || u.InputTokenDetails.CacheWriteTokens != 0) {
		return false
	}
	if hasNonZeroUsageFlatFields(u.FlatFields) {
		return false
	}
	return true
}

func mergeUsagePreferNonZero(dst *Usage, src *Usage) {
	if dst == nil || src == nil {
		return
	}
	if src.InputTokens > 0 {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens > 0 {
		dst.OutputTokens = src.OutputTokens
	}
	if src.PromptTokens > 0 {
		dst.PromptTokens = src.PromptTokens
	}
	if src.CompletionTokens > 0 {
		dst.CompletionTokens = src.CompletionTokens
	}

	// Only accept total tokens from an event when it is likely a full snapshot:
	// - total-only snapshot (no split fields)
	// - or both sides are present in that event.
	if src.TotalTokens > 0 {
		hasSplit := src.InputTokens > 0 || src.OutputTokens > 0 || src.PromptTokens > 0 || src.CompletionTokens > 0
		hasBothSides := (src.InputTokens > 0 && src.OutputTokens > 0) || (src.PromptTokens > 0 && src.CompletionTokens > 0)
		if !hasSplit || hasBothSides {
			dst.TotalTokens = src.TotalTokens
		}
	}

	if src.InputTokenDetails != nil {
		if dst.InputTokenDetails == nil {
			dst.InputTokenDetails = &ResponseTokenDetails{}
		}
		if src.InputTokenDetails.CachedTokens > 0 {
			dst.InputTokenDetails.CachedTokens = src.InputTokenDetails.CachedTokens
		}
		if src.InputTokenDetails.CacheWriteTokens > 0 {
			dst.InputTokenDetails.CacheWriteTokens = src.InputTokenDetails.CacheWriteTokens
		}
	}
	mergeUsageFlatFieldsPreferNonZero(dst, src)
	mergeUsageDebugFactsPreferNonZero(dst, src)

	normalizeUsageFields(dst)
}

func shouldRecomputeMergedTotal(mode string, cfg UsageExtractConfig) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == usageModeAnthropic {
		return true
	}
	return mode == usageModeCustom && cfg.TotalTokensExpr == nil
}

func newStreamUsageExtractFunc(meta *dslmeta.Meta, cfg UsageExtractConfig) streamUsageExtractFunc {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case usageModeOpenAI, usageModeAnthropic, usageModeGemini:
		if canUseBuiltinStreamUsageFastPath(meta, cfg) {
			return func(respRoot map[string]any, _ []byte) (*Usage, int, error) {
				return extractBuiltinStreamUsageFast(mode, respRoot)
			}
		}
	case usageModeCustom:
		if len(cfg.facts) == 0 {
			return func(respRoot map[string]any, _ []byte) (*Usage, int, error) {
				return extractLegacyCustomStreamUsageFast(cfg, respRoot)
			}
		}
	}
	return func(respRoot map[string]any, respBody []byte) (*Usage, int, error) {
		return extractUsageFromResponseRoot(meta, cfg, respRoot, respBody)
	}
}

func canUseBuiltinStreamUsageFastPath(meta *dslmeta.Meta, cfg UsageExtractConfig) bool {
	if len(cfg.facts) > 0 || hasLegacyUsageOverrides(cfg) {
		return false
	}
	if meta == nil {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Mode), usageModeOpenAI) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(meta.API)) {
	case "images.generations", "images.edits", "audio.transcriptions", "audio.translations", "audio.speech", "responses":
		return false
	default:
		return true
	}
}

func hasLegacyUsageOverrides(cfg UsageExtractConfig) bool {
	return strings.TrimSpace(cfg.InputTokensPath) != "" ||
		strings.TrimSpace(cfg.OutputTokensPath) != "" ||
		strings.TrimSpace(cfg.CacheReadTokensPath) != "" ||
		strings.TrimSpace(cfg.CacheWriteTokensPath) != "" ||
		cfg.InputTokensExpr != nil ||
		cfg.OutputTokensExpr != nil ||
		cfg.CacheReadTokensExpr != nil ||
		cfg.CacheWriteTokensExpr != nil ||
		cfg.TotalTokensExpr != nil
}

func extractBuiltinStreamUsageFast(mode string, root map[string]any) (*Usage, int, error) {
	evals := builtinStreamUsageFactEvals(mode, root)
	usage, cachedTokens, err := projectUsageFromFacts(evals)
	if err != nil {
		return nil, 0, err
	}
	if totalTokens := builtinTotalTokens(root, mode); totalTokens > 0 {
		usage.TotalTokens = totalTokens
	}
	return usage, cachedTokens, nil
}

func builtinStreamUsageFactEvals(mode string, root map[string]any) []usageFactEval {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case usageModeOpenAI:
		return openAIStreamUsageFactEvals(root)
	case usageModeAnthropic:
		return anthropicStreamUsageFactEvals(root)
	case usageModeGemini:
		return geminiStreamUsageFactEvals(root)
	default:
		return nil
	}
}

func openAIStreamUsageFactEvals(root map[string]any) []usageFactEval {
	usage, _ := root["usage"].(map[string]any)
	if usage == nil {
		return nil
	}
	evals := make([]usageFactEval, 0, 3)
	appendFirstMatchedUsageFactEval(&evals, usage,
		usageFactConfig{Dimension: "input", Unit: "token", Path: "$.usage.prompt_tokens"},
		usageFactConfig{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens", Fallback: true},
		"prompt_tokens", "input_tokens",
	)
	appendFirstMatchedUsageFactEval(&evals, usage,
		usageFactConfig{Dimension: "output", Unit: "token", Path: "$.usage.completion_tokens"},
		usageFactConfig{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Fallback: true},
		"completion_tokens", "output_tokens",
	)
	promptDetails, _ := usage["prompt_tokens_details"].(map[string]any)
	inputDetails, _ := usage["input_tokens_details"].(map[string]any)
	appendFirstMatchedNestedUsageFactEval(&evals,
		usageFactConfig{Dimension: "cache_read", Unit: "token", Path: "$.usage.prompt_tokens_details.cached_tokens"},
		promptDetails, "cached_tokens",
		usageFactConfig{Dimension: "cache_read", Unit: "token", Path: "$.usage.input_tokens_details.cached_tokens", Fallback: true},
		inputDetails, "cached_tokens",
		usageFactConfig{Dimension: "cache_read", Unit: "token", Path: "$.usage.cached_tokens", Fallback: true},
		usage, "cached_tokens",
	)
	return evals
}

func anthropicStreamUsageFactEvals(root map[string]any) []usageFactEval {
	usage, _ := root["usage"].(map[string]any)
	if usage == nil {
		return nil
	}
	evals := make([]usageFactEval, 0, 5)
	appendMatchedUsageFactEval(&evals, usageFactConfig{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens"}, usage, "input_tokens")
	appendMatchedUsageFactEval(&evals, usageFactConfig{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"}, usage, "output_tokens")
	appendMatchedUsageFactEval(&evals, usageFactConfig{Dimension: "cache_read", Unit: "token", Path: "$.usage.cache_read_input_tokens"}, usage, "cache_read_input_tokens")
	cacheCreation, _ := usage["cache_creation"].(map[string]any)
	var cacheWriteMatched bool
	cacheWriteMatched = appendMatchedUsageFactEval(&evals, usageFactConfig{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_5m_input_tokens", Attrs: map[string]string{"ttl": "5m"}}, cacheCreation, "ephemeral_5m_input_tokens") || cacheWriteMatched
	cacheWriteMatched = appendMatchedUsageFactEval(&evals, usageFactConfig{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation.ephemeral_1h_input_tokens", Attrs: map[string]string{"ttl": "1h"}}, cacheCreation, "ephemeral_1h_input_tokens") || cacheWriteMatched
	if !cacheWriteMatched {
		appendMatchedUsageFactEval(&evals, usageFactConfig{Dimension: "cache_write", Unit: "token", Path: "$.usage.cache_creation_input_tokens", Fallback: true}, usage, "cache_creation_input_tokens")
	}
	return evals
}

func geminiStreamUsageFactEvals(root map[string]any) []usageFactEval {
	if _, ok := root["usageMetadata"].(map[string]any); !ok {
		return nil
	}
	set := builtinUsageFactSet(usageModeGemini)
	return evaluateUsageFactConfigGroups(nil, root, nil, set.factGroups, len(set.facts))
}

func appendFirstMatchedUsageFactEval(dst *[]usageFactEval, root map[string]any, primary, fallback usageFactConfig, primaryKey, fallbackKey string) {
	if appendMatchedUsageFactEval(dst, primary, root, primaryKey) {
		return
	}
	appendMatchedUsageFactEval(dst, fallback, root, fallbackKey)
}

func appendFirstMatchedNestedUsageFactEval(dst *[]usageFactEval, primary usageFactConfig, primaryRoot map[string]any, primaryKey string, fallback usageFactConfig, fallbackRoot map[string]any, fallbackKey string, last usageFactConfig, lastRoot map[string]any, lastKey string) {
	if appendMatchedUsageFactEval(dst, primary, primaryRoot, primaryKey) {
		return
	}
	if appendMatchedUsageFactEval(dst, fallback, fallbackRoot, fallbackKey) {
		return
	}
	appendMatchedUsageFactEval(dst, last, lastRoot, lastKey)
}

func appendMatchedUsageFactEval(dst *[]usageFactEval, cfg usageFactConfig, root map[string]any, key string) bool {
	if root == nil {
		return false
	}
	value, ok := root[key]
	if !ok {
		return false
	}
	*dst = append(*dst, usageFactEval{
		cfg:      cfg,
		quantity: jsonutil.CoerceFloat(value),
		matched:  true,
	})
	return true
}

func extractLegacyCustomStreamUsageFast(cfg UsageExtractConfig, root map[string]any) (*Usage, int, error) {
	legacyCandidates := []usageFactKey{
		{Dimension: "input", Unit: "token"},
		{Dimension: "output", Unit: "token"},
		{Dimension: "cache_read", Unit: "token"},
		{Dimension: "cache_write", Unit: "token"},
	}
	evals := make([]usageFactEval, 0, len(legacyCandidates))
	for _, key := range legacyCandidates {
		fact, ok := usageFactValueFromLegacy(cfg, key)
		if !ok {
			continue
		}
		if strings.TrimSpace(fact.Path) == "" && fact.Expr == nil {
			continue
		}
		quantity, matched := evaluateUsageFact(nil, root, nil, fact)
		evals = append(evals, usageFactEval{cfg: fact, quantity: quantity, matched: matched})
	}
	usage, cachedTokens, err := projectUsageFromFacts(evals)
	if err != nil {
		return nil, 0, err
	}
	if cfg.TotalTokensExpr != nil {
		usage.TotalTokens = cfg.TotalTokensExpr.Eval(root)
	} else {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return usage, cachedTokens, nil
}

func normalizeAnthropicStreamUsageRoot(root map[string]any) map[string]any {
	if _, ok := root["usage"]; ok {
		return root
	}
	msg, ok := root["message"].(map[string]any)
	if !ok || msg == nil {
		return root
	}
	usage, ok := msg["usage"]
	if !ok {
		return root
	}
	normalized := make(map[string]any, len(root)+1)
	for k, v := range root {
		normalized[k] = v
	}
	normalized["usage"] = usage
	return normalized
}

func normalizeUsageFields(u *Usage) {
	if u == nil {
		return
	}
	// Normalize legacy OpenAI fields.
	if u.InputTokens == 0 && u.PromptTokens > 0 {
		u.InputTokens = u.PromptTokens
	}
	if u.OutputTokens == 0 && u.CompletionTokens > 0 {
		u.OutputTokens = u.CompletionTokens
	}
	if u.PromptTokens == 0 && u.InputTokens > 0 {
		u.PromptTokens = u.InputTokens
	}
	if u.CompletionTokens == 0 && u.OutputTokens > 0 {
		u.CompletionTokens = u.OutputTokens
	}
	if u.TotalTokens == 0 && (u.InputTokens > 0 || u.OutputTokens > 0) {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
}

func mergeUsageFlatFieldsPreferNonZero(dst *Usage, src *Usage) {
	if dst == nil || src == nil || len(src.FlatFields) == 0 {
		return
	}
	if dst.FlatFields == nil {
		dst.FlatFields = map[string]any{}
	}
	for k, v := range src.FlatFields {
		if strings.TrimSpace(k) == "" {
			continue
		}
		if current, ok := dst.FlatFields[k]; ok {
			currentInt, currentIsNumber := usageFlatFieldInt(current)
			nextInt, nextIsNumber := usageFlatFieldInt(v)
			if currentIsNumber && nextIsNumber {
				if nextInt > 0 || currentInt == 0 {
					dst.FlatFields[k] = nextInt
				}
				continue
			}
		}
		dst.FlatFields[k] = v
	}
}

func mergeUsageDebugFactsPreferNonZero(dst *Usage, src *Usage) {
	if dst == nil || src == nil || len(src.DebugFacts) == 0 {
		return
	}
	if dst.DebugFacts == nil {
		dst.DebugFacts = make([]UsageFact, 0, len(src.DebugFacts))
	}
	indexByKey := make(map[string]int, len(dst.DebugFacts))
	for i, fact := range dst.DebugFacts {
		indexByKey[usageFactDebugMergeKey(fact)] = i
	}
	for _, fact := range src.DebugFacts {
		key := usageFactDebugMergeKey(fact)
		if idx, ok := indexByKey[key]; ok {
			if fact.Quantity > 0 || dst.DebugFacts[idx].Quantity == 0 {
				dst.DebugFacts[idx] = cloneUsageFactForMerge(fact)
			}
			continue
		}
		indexByKey[key] = len(dst.DebugFacts)
		dst.DebugFacts = append(dst.DebugFacts, cloneUsageFactForMerge(fact))
	}
}

func usageFactDebugMergeKey(fact UsageFact) string {
	key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
	parts := []string{key.Dimension, key.Unit}
	if len(fact.Attributes) == 0 {
		return strings.Join(parts, "|")
	}
	attrKeys := make([]string, 0, len(fact.Attributes))
	for k := range fact.Attributes {
		attrKeys = append(attrKeys, k)
	}
	// Stable merge key regardless of attribute declaration order.
	slices.Sort(attrKeys)
	for _, k := range attrKeys {
		parts = append(parts, strings.TrimSpace(k), strings.TrimSpace(fact.Attributes[k]))
	}
	return strings.Join(parts, "|")
}

func cloneUsageFactForMerge(fact UsageFact) UsageFact {
	out := fact
	if len(fact.Attributes) == 0 {
		return out
	}
	out.Attributes = make(map[string]string, len(fact.Attributes))
	for k, v := range fact.Attributes {
		out.Attributes[k] = v
	}
	return out
}

func hasNonZeroUsageFlatFields(fields map[string]any) bool {
	for _, v := range fields {
		if n, ok := usageFlatFieldInt(v); ok && n != 0 {
			return true
		}
	}
	return false
}

func usageFlatFieldInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
