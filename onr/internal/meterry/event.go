package meterry

import (
	"fmt"
	"strings"
	"time"
)

// Event is the normalized, non-sensitive usage payload sent to Meterry.
type Event struct {
	Source          string         `json:"source"`
	ExternalEventID string         `json:"external_event_id"`
	IdempotencyKey  string         `json:"idempotency_key"`
	OccurredAt      int64          `json:"occurred_at"`
	SubjectType     string         `json:"subject_type,omitempty"`
	SubjectID       string         `json:"subject_id,omitempty"`
	RawJSON         map[string]any `json:"raw_json"`
	Billing         map[string]any `json:"x-billing,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

func NewEvent(requestID string, provider string, api string, model string, stream bool, status int, usageStage string, usage map[string]any, subjectType string, subjectID string, appname string, pricingHints map[string]any) Event {
	rid := strings.TrimSpace(requestID)
	provider = strings.TrimSpace(provider)
	api = strings.TrimSpace(api)
	model = strings.TrimSpace(model)
	subjectType = strings.TrimSpace(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	usage = normalizeUsageAliases(usage)
	raw := map[string]any{
		"provider":    provider,
		"api":         api,
		"model":       model,
		"stream":      stream,
		"status":      status,
		"usage_stage": strings.TrimSpace(usageStage),
		"usage":       cloneMap(usage),
		"meta": map[string]any{
			"subject_type": subjectType,
			"subject_id":   subjectID,
			"request_id":   rid,
		},
	}
	if appname = strings.TrimSpace(appname); appname != "" {
		raw["meta"].(map[string]any)["appname"] = appname
	}
	e := Event{
		Source:          "open-next-router",
		ExternalEventID: rid,
		IdempotencyKey:  fmt.Sprintf("onr:%s", rid),
		OccurredAt:      time.Now().Unix(),
		SubjectType:     subjectType,
		SubjectID:       subjectID,
		RawJSON:         raw,
	}
	if len(pricingHints) > 0 {
		e.Billing = map[string]any{
			"items":         billingItems(usage),
			"pricing_hints": cloneMap(pricingHints),
		}
	} else if items := billingItems(usage); len(items) > 0 {
		e.Billing = map[string]any{"items": items}
	}
	return e
}

func billingItems(usage map[string]any) []map[string]any {
	if len(usage) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, 4)
	for metric, key := range map[string]string{
		"prompt_tokens":     "input_tokens",
		"completion_tokens": "output_tokens",
		"cached_tokens":     "cache_read_tokens",
	} {
		if value, ok := usage[key]; ok && numberPositive(value) {
			items = append(items, map[string]any{"metric": metric, "quantity": value, "unit": "token"})
		}
	}
	return items
}

func numberPositive(v any) bool {
	switch n := v.(type) {
	case int:
		return n > 0
	case int64:
		return n > 0
	case float64:
		return n > 0
	default:
		return false
	}
}

func normalizeUsageAliases(in map[string]any) map[string]any {
	out := cloneMap(in)
	if len(out) == 0 {
		return out
	}
	if _, ok := out["prompt_tokens"]; !ok {
		if v, ok := out["input_tokens"]; ok {
			out["prompt_tokens"] = v
		}
	}
	if _, ok := out["completion_tokens"]; !ok {
		if v, ok := out["output_tokens"]; ok {
			out["completion_tokens"] = v
		}
	}
	if _, ok := out["cached_tokens"]; !ok {
		if v, ok := out["cache_read_tokens"]; ok {
			out["cached_tokens"] = v
		}
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
