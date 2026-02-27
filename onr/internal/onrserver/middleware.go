package onrserver

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestid"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/onr/internal/onrserver/accesslog"
)

func requestLoggerWithColor(l *log.Logger, color bool, requestIDHeaderKey string, appnameInferEnabled bool, appnameInferUnknown string, accessFormatter *logx.AccessLogFormatter) gin.HandlerFunc {
	requestIDHeaderKey = requestid.ResolveHeaderKey(requestIDHeaderKey)
	collector := accesslog.NewCollector(requestIDHeaderKey, appnameInferEnabled, appnameInferUnknown)
	if l == nil {
		l = log.New(os.Stdout, "", 0)
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)
		fields := collector.Collect(c, latency)

		ts := time.Now()
		if accessFormatter != nil {
			l.Println(accessFormatter.Format(ts, status, latency, c.ClientIP(), c.Request.Method, c.Request.URL.Path, fields, color))
			return
		}
		l.Println(logx.FormatRequestLineWithColor(ts, status, latency, c.ClientIP(), c.Request.Method, c.Request.URL.Path, fields, color))
	}
}
