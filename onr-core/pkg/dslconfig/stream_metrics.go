package dslconfig

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
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

	inputIncludesCache  bool
	maxFreshInputTokens int

	// merged usage snapshot across SSE events
	lastUsage        *Usage
	lastCachedTokens int

	// finish reason (first non-empty)
	finishReason string
}

type streamUsageExtractFunc func(event string, respRoot map[string]any, respBody []byte) (*Usage, int, error)

func NewStreamMetricsAggregator(meta *dslmeta.Meta, usageCfg UsageExtractConfig, finishCfg FinishReasonExtractConfig) *StreamMetricsAggregator {
	usageCfg = prepareUsageExtractConfig(usageCfg)
	return &StreamMetricsAggregator{
		meta:               meta,
		usageCfg:           usageCfg,
		finishCfg:          finishCfg,
		extract:            newStreamUsageExtractFunc(meta, usageCfg),
		inputIncludesCache: usageConfigInputIncludesCache(meta, usageCfg),
	}
}

func (a *StreamMetricsAggregator) OnSSEDataJSON(payload []byte) error {
	return a.OnSSEEventDataJSON("", payload)
}

func (a *StreamMetricsAggregator) OnSSEEventDataJSON(event string, payload []byte) error {
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

	u, cachedTokens, err := a.extract(event, root, payload)
	if err != nil {
		// ignore individual event errors
		return nil
	}
	if u == nil || isAllZeroUsage(u) {
		return nil
	}
	a.recordFreshInputTokens(u)
	// Merge semantics: do not allow 0 to overwrite a previously known value.
	if a.lastUsage == nil {
		a.lastUsage = u
	} else {
		mergeUsagePreferNonZero(a.lastUsage, u)
	}
	a.recomputeMergedInputTokens()
	if cachedTokens > 0 {
		a.lastCachedTokens = cachedTokens
	}
	// For anthropic/custom split-stream usage, recompute total from the merged fields unless
	// custom mode explicitly provides TotalTokensExpr.
	if shouldRecomputeMergedTotal(a.usageCfg) && a.lastUsage != nil {
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
	var curEvent string
	var curData [][]byte
	flush := func() {
		if len(curData) == 0 {
			curEvent = ""
			return
		}
		payload := bytes.TrimSpace(bytes.Join(curData, []byte{'\n'}))
		curData = curData[:0]
		_ = a.OnSSEEventDataJSON(curEvent, payload)
		curEvent = ""
	}
	for _, raw := range lines {
		line := bytes.TrimRight(raw, "\r")
		if len(bytes.TrimSpace(line)) == 0 {
			flush()
			continue
		}
		if bytes.HasPrefix(line, []byte("event:")) {
			curEvent = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
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

	if a.lastUsage == nil || isAllZeroUsage(a.lastUsage) {
		return nil, 0, finishReason, false
	}
	a.recomputeMergedInputTokens()
	normalizeUsageFields(a.lastUsage)
	if shouldRecomputeMergedTotal(a.usageCfg) {
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

func (a *StreamMetricsAggregator) recordFreshInputTokens(u *Usage) {
	if a == nil || !a.inputIncludesCache || u == nil {
		return
	}
	fresh := u.InputTokens
	if u.InputTokenDetails != nil {
		fresh -= u.InputTokenDetails.CachedTokens
		fresh -= u.InputTokenDetails.CacheWriteTokens
	}
	if fresh > a.maxFreshInputTokens {
		a.maxFreshInputTokens = fresh
	}
}

func (a *StreamMetricsAggregator) recomputeMergedInputTokens() {
	if a == nil || !a.inputIncludesCache || a.lastUsage == nil {
		return
	}
	total := a.maxFreshInputTokens
	if a.lastUsage.InputTokenDetails != nil {
		total += a.lastUsage.InputTokenDetails.CachedTokens
		total += a.lastUsage.InputTokenDetails.CacheWriteTokens
	}
	if total > 0 {
		a.lastUsage.InputTokens = total
		a.lastUsage.PromptTokens = total
	}
}

func usageConfigInputIncludesCache(meta *dslmeta.Meta, cfg UsageExtractConfig) bool {
	compiled := compileUsageExtractConfig(meta, cfg)
	if normalizeUsageMode(compiled.Mode) != usageModeCustom {
		return false
	}
	inputRules := map[string]struct{}{}
	for _, fact := range compiled.facts {
		if normalizeUsageFactKey(fact.Dimension, fact.Unit) != (usageFactKey{Dimension: "input", Unit: "token"}) {
			continue
		}
		inputRules[usageFactMergeSignature(fact)] = struct{}{}
	}
	if len(inputRules) == 0 {
		return false
	}
	for _, fact := range compiled.facts {
		key := normalizeUsageFactKey(fact.Dimension, fact.Unit)
		if key != (usageFactKey{Dimension: "cache_read", Unit: "token"}) &&
			key != (usageFactKey{Dimension: "cache_write", Unit: "token"}) {
			continue
		}
		if _, ok := inputRules[usageFactMergeSignature(fact)]; ok {
			return true
		}
	}
	return false
}

func usageFactMergeSignature(fact usageFactConfig) string {
	expr := ""
	if fact.Expr != nil {
		expr = fact.Expr.String()
	}
	return strings.Join([]string{
		strings.TrimSpace(fact.Source),
		strings.TrimSpace(fact.Event),
		strings.TrimSpace(fact.Path),
		strings.TrimSpace(fact.CountPath),
		strings.TrimSpace(fact.SumPath),
		expr,
		strings.TrimSpace(fact.Type),
		strings.TrimSpace(fact.Status),
	}, "|")
}

func shouldRecomputeMergedTotal(cfg UsageExtractConfig) bool {
	return strings.EqualFold(strings.TrimSpace(cfg.Mode), usageModeCustom) && cfg.TotalTokensExpr == nil
}

func newStreamUsageExtractFunc(meta *dslmeta.Meta, cfg UsageExtractConfig) streamUsageExtractFunc {
	compiled := compileUsageExtractConfig(meta, cfg)
	return func(event string, respRoot map[string]any, respBody []byte) (*Usage, int, error) {
		reqRoot := requestRootFromMeta(meta)
		derivedRoot := derivedRootFromMeta(meta)
		return extractUsageFromRootsWithEvent(meta, event, compiled, reqRoot, respRoot, derivedRoot, respBody)
	}
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
	if strings.TrimSpace(fact.Event) != "" {
		parts = append(parts, strings.TrimSpace(fact.Event))
	}
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
