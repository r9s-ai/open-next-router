package onrserver

import (
	"context"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr/internal/meterry"
	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func enqueueBillingEvent(cfg *config.Config, sink *meterry.Client, c *gin.Context, res *proxy.Result) {
	if cfg == nil || sink == nil || !sink.Enabled() || c == nil || res == nil {
		return
	}
	if cfg.Meterry.OnlyBillableSuccess != nil && *cfg.Meterry.OnlyBillableSuccess && (res.Status < 200 || res.Status >= 300) {
		return
	}
	rid := strings.TrimSpace(c.GetString("X-Onr-Request-Id"))
	if rid == "" {
		rid = strings.TrimSpace(c.GetString("X-Request-Id"))
	}
	subjectID := strings.TrimSpace(c.GetString("onr.auth_subject_id"))
	if subjectID == "" {
		subjectID = strings.TrimSpace(cfg.Meterry.FallbackSubjectID)
	}
	if subjectID == "" {
		return
	}
	var appname string
	if c.Request != nil {
		appname = c.GetHeader("appname")
	}
	event := meterry.NewEvent(
		rid,
		res.Provider,
		res.API,
		res.Model,
		res.Stream,
		res.Status,
		res.UsageStage,
		res.Usage,
		cfg.Meterry.SubjectType,
		subjectID,
		appname,
		pricingHints(res.Cost),
	)
	if err := sink.Enqueue(event); err != nil {
		// Billing is deliberately best-effort for the request path. The sink owns
		// retryable delivery once the event is durably queued.
		return
	}
}

func enforceBillingBalance(cfg *config.Config, sink *meterry.Client, c *gin.Context, requestIDHeaderKey string) bool {
	if cfg == nil || sink == nil || !cfg.Meterry.BalanceEnforcement.Enabled || !sink.BalanceEnabled() || c == nil {
		return true
	}
	subjectID := strings.TrimSpace(c.GetString("onr.auth_subject_id"))
	if subjectID == "" {
		return true
	}
	subjectType := strings.TrimSpace(cfg.Meterry.SubjectType)
	allowed, err := sink.CheckBalance(requestContext(c), subjectType, subjectID)
	if err != nil {
		if strings.EqualFold(strings.TrimSpace(cfg.Meterry.BalanceEnforcement.FailureMode), "open") {
			log.Printf("[ONR] WARN | meterry | balance lookup failed, allowing request | subject=%s/%s error=%v", subjectType, subjectID, err)
			return true
		}
		writeOpenAIErrorWithStatus(c, requestIDHeaderKey, 503, "billing_error", "billing_unavailable", "billing balance service is unavailable")
		return false
	}
	if !allowed {
		writeOpenAIErrorWithStatus(c, requestIDHeaderKey, 402, "billing_error", "insufficient_balance", "account balance is insufficient")
		return false
	}
	return true
}

func requestContext(c *gin.Context) context.Context {
	if c != nil && c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}

func pricingHints(cost map[string]any) map[string]any {
	if len(cost) == 0 {
		return nil
	}
	unit := numberOr(cost["cost_rate_unit"], 1000000)
	currency := "USD"
	if v, ok := cost["cost_unit"].(string); ok && strings.TrimSpace(v) != "" {
		currency = strings.ToUpper(strings.TrimSpace(v))
	}
	out := map[string]any{}
	for metric, key := range map[string]string{
		"input_tokens":       "price_input",
		"output_tokens":      "price_output",
		"cache_read_tokens":  "price_cache_read",
		"cache_write_tokens": "price_cache_write",
	} {
		if price, ok := cost[key]; ok && numberPositive(price) {
			out[metric] = map[string]any{"unit_price": price, "pricing_unit": unit, "currency": currency}
		}
	}
	if v, ok := out["input_tokens"]; ok {
		out["prompt_tokens"] = v
	}
	if v, ok := out["output_tokens"]; ok {
		out["completion_tokens"] = v
	}
	if v, ok := out["cache_read_tokens"]; ok {
		out["cached_tokens"] = v
	}
	return out
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

func numberOr(v any, fallback int) any {
	if numberPositive(v) {
		return v
	}
	return fallback
}
