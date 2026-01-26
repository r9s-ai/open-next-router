package onrserver

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/edgefn/open-next-router/internal/logx"
	"github.com/edgefn/open-next-router/internal/requestid"
)

func requestLogger() gin.HandlerFunc {
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
		if v, ok := c.Get("onr.latency_ms"); ok {
			fields["latency_ms"] = v
		} else {
			fields["latency_ms"] = latency.Milliseconds()
		}

		log.Println(logx.FormatRequestLine(time.Now(), status, latency, c.ClientIP(), c.Request.Method, c.Request.URL.Path, fields))
	}
}
