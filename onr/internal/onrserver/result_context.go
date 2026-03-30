package onrserver

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
)

func setProxyResultContext(c *gin.Context, res *proxy.Result) {
	if c == nil || res == nil {
		return
	}
	c.Set("onr.latency_ms", res.LatencyMs)
	if res.TTFTMs > 0 {
		c.Set("onr.ttft_ms", res.TTFTMs)
	}
	if res.TPS > 0 {
		c.Set("onr.tps", res.TPS)
	}
	if res.Status > 0 {
		c.Set("onr.upstream_status", res.Status)
	}
	if strings.TrimSpace(res.FinishReason) != "" {
		c.Set("onr.finish_reason", strings.TrimSpace(res.FinishReason))
	}
	if strings.TrimSpace(res.UsageStage) != "" {
		c.Set("onr.usage_stage", res.UsageStage)
	}
	if res.Usage == nil {
		goto setCost
	}
	for k, v := range res.Usage {
		if ctxKey, ok := logx.StandardUsageContextKey(k); ok {
			c.Set(ctxKey, v)
			continue
		}
		if strings.TrimSpace(k) == "" {
			continue
		}
		c.Set("onr.usage_extra."+k, v)
	}

setCost:
	if res.Cost == nil {
		return
	}
	for k, v := range res.Cost {
		if ctxKey, ok := logx.CostContextKey(k); ok {
			c.Set(ctxKey, v)
		}
	}
}
