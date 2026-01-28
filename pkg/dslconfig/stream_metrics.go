package dslconfig

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
)

// StreamMetricsAggregator aggregates best-effort metrics from SSE "data:" JSON payloads.
//
// Semantics (aligned with next-router requirements):
//   - usage: take the last known upstream usage (non-zero). For Anthropic SSE, token fields may appear
//     in different events; we keep the latest positive value per field and build a final usage snapshot.
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
		} else {
			// ignore individual event errors
		}
	}

	mode := strings.ToLower(strings.TrimSpace(a.usageCfg.Mode))
	if mode == "" {
		return nil
	}

	if mode == "anthropic" {
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
		if v := coerceInt(um["input_tokens"]); v > 0 {
			a.anthropicSnap.InputTokens = v
		}
		if v := coerceInt(um["output_tokens"]); v > 0 {
			a.anthropicSnap.OutputTokens = v
		}
		if v := coerceInt(um["cache_read_input_tokens"]); v > 0 {
			a.anthropicSnap.CacheReadTokens = v
		}
		if v := coerceInt(um["cache_creation_input_tokens"]); v > 0 {
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
	// "take last" semantics
	a.lastUsage = u
	a.lastCachedTokens = cachedTokens
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
	if mode == "anthropic" {
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
