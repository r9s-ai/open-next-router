package proxy

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

func TestClient_LogUsageFactsDebug(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	logger, err := logx.NewSystemLoggerWithOptions(logx.SystemLoggerOptions{
		Writer: &out,
		Level:  "debug",
		Color:  boolPtr(false),
	})
	if err != nil {
		t.Fatalf("NewSystemLoggerWithOptions: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("X-Onr-Request-Id", "rid-usage-facts-1")

	client := &Client{SystemLogger: logger}
	client.logUsageFactsDebug(c, "anthropic", "claude.messages", false, "claude-haiku-4-5", "upstream", &dslconfig.Usage{
		DebugFacts: []dslconfig.UsageFact{
			{
				Dimension: "cache_write",
				Unit:      "token",
				Quantity:  6802,
				Attributes: map[string]string{
					"ttl": "5m",
				},
			},
		},
	})

	got := out.String()
	if !strings.Contains(got, "usage facts extracted") {
		t.Fatalf("expected debug log message, got=%q", got)
	}
	if !strings.Contains(got, "request_id=rid-usage-facts-1") {
		t.Fatalf("expected request_id in debug log, got=%q", got)
	}
	if !strings.Contains(got, "provider=anthropic") {
		t.Fatalf("expected provider in debug log, got=%q", got)
	}
	if !strings.Contains(got, "usage_facts=") {
		t.Fatalf("expected usage_facts payload in debug log, got=%q", got)
	}
	if !strings.Contains(got, "\\\"dimension\\\":\\\"cache_write\\\"") {
		t.Fatalf("expected usage_facts json in debug log, got=%q", got)
	}
}

func boolPtr(v bool) *bool { return &v }
