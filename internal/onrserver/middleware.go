package onrserver

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/internal/logx"
	"github.com/r9s-ai/open-next-router/internal/requestid"
)

func requestLoggerWithColor(l *log.Logger, color bool) gin.HandlerFunc {
	if l == nil {
		l = log.New(os.Stdout, "", log.LstdFlags)
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)

		fields := map[string]any{}
		if v := c.GetString(requestid.HeaderKey); v != "" {
			fields["request_id"] = v
		}
		if v, ok := c.Get("onr.provider"); ok {
			fields["provider"] = v
		}
		if v, ok := c.Get("onr.provider_source"); ok {
			fields["provider_source"] = v
		}
		if v, ok := c.Get("onr.api"); ok {
			fields["api"] = v
		}
		if v, ok := c.Get("onr.stream"); ok {
			fields["stream"] = v
		}
		if v, ok := c.Get("onr.model"); ok {
			fields["model"] = v
		}
		if v, ok := c.Get("onr.usage_stage"); ok {
			fields["usage_stage"] = v
		}
		if v, ok := c.Get("onr.usage_input_tokens"); ok {
			fields["input_tokens"] = v
		}
		if v, ok := c.Get("onr.usage_output_tokens"); ok {
			fields["output_tokens"] = v
		}
		if v, ok := c.Get("onr.usage_total_tokens"); ok {
			fields["total_tokens"] = v
		}
		if v, ok := c.Get("onr.usage_cache_read_tokens"); ok {
			fields["cache_read_tokens"] = v
		}
		if v, ok := c.Get("onr.usage_cache_write_tokens"); ok {
			fields["cache_write_tokens"] = v
		}
		if v, ok := c.Get("onr.latency_ms"); ok {
			fields["latency_ms"] = v
		} else {
			fields["latency_ms"] = latency.Milliseconds()
		}
		if v, ok := c.Get("onr.upstream_status"); ok {
			fields["upstream_status"] = v
		}
		if v, ok := c.Get("onr.finish_reason"); ok {
			fields["finish_reason"] = v
		}

		l.Println(logx.FormatRequestLineWithColor(time.Now(), status, latency, c.ClientIP(), c.Request.Method, c.Request.URL.Path, fields, color))
	}
}
