package dslconfig

import (
	"bytes"
	"encoding/json"
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

	// last usage (for non-anthropic)
	lastUsage        *Usage
	lastCachedTokens int

	// finish reason (first non-empty)
	finishReason string

	// anthropic snapshot (best-effort per-field merge across events)
	anthropicSnap usageSnapshot
}

type usageSnapshot struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
}

func NewStreamMetricsAggregator(meta *dslmeta.Meta, usageCfg UsageExtractConfig, finishCfg FinishReasonExtractConfig) *StreamMetricsAggregator {
	return &StreamMetricsAggregator{
		meta:      meta,
		usageCfg:  usageCfg,
		finishCfg: finishCfg,
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

	// finish_reason: first non-empty
	if strings.TrimSpace(a.finishReason) == "" && (strings.TrimSpace(a.finishCfg.Mode) != "" || strings.TrimSpace(a.finishCfg.FinishReasonPath) != "") {
		if v, err := ExtractFinishReason(a.meta, a.finishCfg, payload); err == nil {
			if s := strings.TrimSpace(v); s != "" {
				a.finishReason = s
			}
		}
	}

	mode := strings.ToLower(strings.TrimSpace(a.usageCfg.Mode))
	if mode == "" {
		return nil
	}

	if mode == usageModeAnthropic {
		// Anthropic SSE events may carry usage at:
		// - obj.usage (message_delta)
		// - obj.message.usage (message_start)
		var obj map[string]any
		if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
			return nil
		}
		var usage any
		if u, ok := obj["usage"]; ok {
			usage = u
		} else if msg, ok := obj["message"].(map[string]any); ok {
			usage = msg["usage"]
		}
		um, ok := usage.(map[string]any)
		if !ok || um == nil {
			return nil
		}
		if v := jsonutil.CoerceInt(um["input_tokens"]); v > 0 {
			a.anthropicSnap.InputTokens = v
		}
		if v := jsonutil.CoerceInt(um["output_tokens"]); v > 0 {
			a.anthropicSnap.OutputTokens = v
		}
		if v := jsonutil.CoerceInt(um["cache_read_input_tokens"]); v > 0 {
			a.anthropicSnap.CacheReadTokens = v
		}
		if v := jsonutil.CoerceInt(um["cache_creation_input_tokens"]); v > 0 {
			a.anthropicSnap.CacheWriteTokens = v
		}
		return nil
	}

	u, cachedTokens, err := ExtractUsage(a.meta, a.usageCfg, payload)
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
	// For custom mode, total_tokens is derived as input+output unless an explicit TotalTokensExpr is set.
	// In streaming where usage may be split across events, we must recompute total from the merged fields.
	if strings.EqualFold(strings.TrimSpace(a.usageCfg.Mode), usageModeCustom) && a.usageCfg.TotalTokensExpr == nil && a.lastUsage != nil {
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
	if mode == usageModeAnthropic {
		s := a.anthropicSnap
		if s.InputTokens == 0 && s.OutputTokens == 0 && s.CacheReadTokens == 0 && s.CacheWriteTokens == 0 {
			return nil, 0, finishReason, false
		}
		u := &Usage{
			InputTokens:      s.InputTokens,
			OutputTokens:     s.OutputTokens,
			PromptTokens:     s.InputTokens,
			CompletionTokens: s.OutputTokens,
			TotalTokens:      s.InputTokens + s.OutputTokens,
		}
		if s.CacheReadTokens > 0 || s.CacheWriteTokens > 0 {
			u.InputTokenDetails = &ResponseTokenDetails{
				CachedTokens:     s.CacheReadTokens,
				CacheWriteTokens: s.CacheWriteTokens,
			}
		}
		return u, s.CacheReadTokens, finishReason, !isAllZeroUsage(u)
	}

	if a.lastUsage == nil || isAllZeroUsage(a.lastUsage) {
		return nil, 0, finishReason, false
	}
	normalizeUsageFields(a.lastUsage)
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

	normalizeUsageFields(dst)
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
