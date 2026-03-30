package accesslog

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/appnameinfer"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

var accessLogFieldSpecs = logx.AccessLogContextFieldSpecs()

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
	out := make(map[string]any, len(accessLogFieldSpecs)+3)
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
	copyContextFieldsBySpec(ctx, out, accessLogFieldSpecs)
	copyUsageExtraFields(ctx, out)
	return out
}

func copyContextFieldsBySpec(ctx *gin.Context, dst map[string]any, specs []logx.AccessLogContextFieldSpec) {
	for _, s := range specs {
		if v, ok := ctx.Get(s.CtxKey); ok {
			dst[s.LogKey] = v
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
