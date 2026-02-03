package onrserver

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/internal/proxy"
)

func setProxyResultContext(c *gin.Context, res *proxy.Result) {
	if c == nil || res == nil {
		return
	}
	c.Set("onr.latency_ms", res.LatencyMs)
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
		return
	}
	if v, ok := res.Usage["input_tokens"]; ok {
		c.Set("onr.usage_input_tokens", v)
	}
	if v, ok := res.Usage["output_tokens"]; ok {
		c.Set("onr.usage_output_tokens", v)
	}
	if v, ok := res.Usage["total_tokens"]; ok {
		c.Set("onr.usage_total_tokens", v)
	}
	if v, ok := res.Usage["cache_read_tokens"]; ok {
		c.Set("onr.usage_cache_read_tokens", v)
	}
	if v, ok := res.Usage["cache_write_tokens"]; ok {
		c.Set("onr.usage_cache_write_tokens", v)
	}
}
