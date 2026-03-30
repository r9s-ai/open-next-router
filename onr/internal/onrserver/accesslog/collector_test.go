package accesslog

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestCollector_CollectMapsContextFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/v1/chat/completions", nil)
	ctx.Request.Header.Set("appname", "my-client")

	ctx.Set("X-Onr-Request-Id", "rid-1")
	ctx.Set("onr.provider", "openai")
	ctx.Set("onr.model", "gpt-4o-mini")
	ctx.Set("onr.ttft_ms", int64(123))
	ctx.Set("onr.tps", 45.6)

	got := NewCollector("X-Onr-Request-Id", false, "").Collect(ctx, 2*time.Second)
	if got["request_id"] != "rid-1" {
		t.Fatalf("unexpected request_id: %#v", got["request_id"])
	}
	if got["appname"] != "my-client" {
		t.Fatalf("unexpected appname: %#v", got["appname"])
	}
	if got["provider"] != "openai" {
		t.Fatalf("unexpected provider: %#v", got["provider"])
	}
	if got["model"] != "gpt-4o-mini" {
		t.Fatalf("unexpected model: %#v", got["model"])
	}
	if got["ttft_ms"] != int64(123) {
		t.Fatalf("unexpected ttft_ms: %#v", got["ttft_ms"])
	}
	if got["tps"] != 45.6 {
		t.Fatalf("unexpected tps: %#v", got["tps"])
	}
	if got["latency_ms"] != int64(2000) {
		t.Fatalf("unexpected latency_ms: %#v", got["latency_ms"])
	}
}

func TestCollector_CollectAppnameInference(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("infer from user-agent", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest("GET", "/v1/models", nil)
		ctx.Request.Header.Set("User-Agent", "claude-code/1.0")

		got := NewCollector("X-Onr-Request-Id", true, "").Collect(ctx, time.Second)
		if got["appname"] != "claude-code" {
			t.Fatalf("unexpected appname: %#v", got["appname"])
		}
	})

	t.Run("fallback unknown", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest("GET", "/v1/models", nil)
		ctx.Request.Header.Set("User-Agent", "Mozilla/5.0")

		got := NewCollector("X-Onr-Request-Id", true, "unknown-client").Collect(ctx, time.Second)
		if got["appname"] != "unknown-client" {
			t.Fatalf("unexpected appname: %#v", got["appname"])
		}
	})
}

func TestCollector_CollectUsageExtraFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/v1/chat/completions", nil)
	ctx.Set("onr.usage_extra.cache_write_ttl_5m_tokens", 6802)
	ctx.Set("onr.usage_extra.cache_write_ttl_1h_tokens", 0)

	got := NewCollector("X-Onr-Request-Id", false, "").Collect(ctx, time.Second)
	if got["cache_write_ttl_5m_tokens"] != 6802 {
		t.Fatalf("unexpected cache_write_ttl_5m_tokens: %#v", got["cache_write_ttl_5m_tokens"])
	}
	if got["cache_write_ttl_1h_tokens"] != 0 {
		t.Fatalf("unexpected cache_write_ttl_1h_tokens: %#v", got["cache_write_ttl_1h_tokens"])
	}
}
