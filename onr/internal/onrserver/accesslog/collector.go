package accesslog

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/appnameinfer"
)

type contextFieldSpec struct {
	ctxKey string
	logKey string
}

type Collector struct {
	requestIDHeaderKey string
	appNameInfer       struct {
		enabled bool
		unknown string
	}
}

func NewCollector(requestIDHeaderKey string, appNameInferEnabled bool, appNameInferUnknown string) *Collector {
	c := &Collector{requestIDHeaderKey: requestIDHeaderKey}
	c.appNameInfer.enabled = appNameInferEnabled
	c.appNameInfer.unknown = strings.TrimSpace(appNameInferUnknown)
	return c
}

func (c *Collector) Collect(ctx *gin.Context, latency time.Duration) map[string]any {
	if ctx == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(accessLogContextFieldSpecs)+3)
	if v := strings.TrimSpace(ctx.GetString(c.requestIDHeaderKey)); v != "" {
		out["request_id"] = v
	}
	if v := c.resolveAppNameForLog(ctx); v != "" {
		out["appname"] = v
	}
	out["latency_ms"] = latency.Milliseconds()
	if v, ok := ctx.Get("onr.latency_ms"); ok {
		switch n := v.(type) {
		case int64:
			out["latency_ms"] = n
		case int:
			out["latency_ms"] = int64(n)
		default:
			out["latency_ms"] = v
		}
	}
	copyContextFieldsBySpec(ctx, out, accessLogContextFieldSpecs)
	copyUsageExtraFields(ctx, out)
	return out
}

func copyContextFieldsBySpec(ctx *gin.Context, dst map[string]any, specs []contextFieldSpec) {
	for _, s := range specs {
		if v, ok := ctx.Get(s.ctxKey); ok {
			dst[s.logKey] = v
		}
	}
}

func copyUsageExtraFields(ctx *gin.Context, dst map[string]any) {
	if ctx == nil || len(ctx.Keys) == 0 {
		return
	}
	for k, v := range ctx.Keys {
		if !strings.HasPrefix(k, "onr.usage_extra.") {
			continue
		}
		logKey := strings.TrimPrefix(k, "onr.usage_extra.")
		if strings.TrimSpace(logKey) == "" {
			continue
		}
		dst[logKey] = v
	}
}

func (c *Collector) resolveAppNameForLog(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	if v := strings.TrimSpace(ctx.GetHeader("appname")); v != "" {
		return v
	}
	if !c.appNameInfer.enabled {
		return ""
	}
	if v, ok := appnameinfer.Infer(ctx.GetHeader("User-Agent")); ok {
		return v
	}
	return c.appNameInfer.unknown
}

var accessLogContextFieldSpecs = []contextFieldSpec{
	{ctxKey: "onr.provider", logKey: "provider"},
	{ctxKey: "onr.provider_source", logKey: "provider_source"},
	{ctxKey: "onr.api", logKey: "api"},
	{ctxKey: "onr.stream", logKey: "stream"},
	{ctxKey: "onr.model", logKey: "model"},
	{ctxKey: "onr.usage_stage", logKey: "usage_stage"},
	{ctxKey: "onr.usage_input_tokens", logKey: "input_tokens"},
	{ctxKey: "onr.usage_output_tokens", logKey: "output_tokens"},
	{ctxKey: "onr.usage_total_tokens", logKey: "total_tokens"},
	{ctxKey: "onr.usage_cache_read_tokens", logKey: "cache_read_tokens"},
	{ctxKey: "onr.usage_cache_write_tokens", logKey: "cache_write_tokens"},
	{ctxKey: "onr.cost_total", logKey: "cost_total"},
	{ctxKey: "onr.cost_input", logKey: "cost_input"},
	{ctxKey: "onr.cost_output", logKey: "cost_output"},
	{ctxKey: "onr.cost_cache_read", logKey: "cost_cache_read"},
	{ctxKey: "onr.cost_cache_write", logKey: "cost_cache_write"},
	{ctxKey: "onr.billable_input_tokens", logKey: "billable_input_tokens"},
	{ctxKey: "onr.cost_multiplier", logKey: "cost_multiplier"},
	{ctxKey: "onr.cost_model", logKey: "cost_model"},
	{ctxKey: "onr.cost_channel", logKey: "cost_channel"},
	{ctxKey: "onr.cost_unit", logKey: "cost_unit"},
	{ctxKey: "onr.upstream_status", logKey: "upstream_status"},
	{ctxKey: "onr.finish_reason", logKey: "finish_reason"},
	{ctxKey: "onr.ttft_ms", logKey: "ttft_ms"},
	{ctxKey: "onr.tps", logKey: "tps"},
}
