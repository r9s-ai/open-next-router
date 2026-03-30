package onrserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
)

func TestSetProxyResultContext_InputTokensPassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("status 200 keeps input tokens", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		setProxyResultContext(c, &proxy.Result{
			Status: http.StatusOK,
			Usage: map[string]any{
				"input_tokens":  12,
				"output_tokens": 34,
			},
		})

		v, ok := c.Get("onr.usage_input_tokens")
		if !ok || v != 12 {
			t.Fatalf("expected input_tokens=12, got ok=%v value=%v", ok, v)
		}
		v, ok = c.Get("onr.usage_output_tokens")
		if !ok || v != 34 {
			t.Fatalf("expected output_tokens=34, got ok=%v value=%v", ok, v)
		}
	})

	t.Run("non-200 keeps input tokens when result already has usage", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		setProxyResultContext(c, &proxy.Result{
			Status: http.StatusBadGateway,
			Usage: map[string]any{
				"input_tokens":  12,
				"output_tokens": 34,
			},
		})

		v, ok := c.Get("onr.usage_input_tokens")
		if !ok || v != 12 {
			t.Fatalf("expected input_tokens=12, got ok=%v value=%v", ok, v)
		}
		v, ok = c.Get("onr.usage_output_tokens")
		if !ok || v != 34 {
			t.Fatalf("expected output_tokens=34, got ok=%v value=%v", ok, v)
		}
	})
}

func TestSetProxyResultContext_StreamPerfFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setProxyResultContext(c, &proxy.Result{
		Status:    http.StatusOK,
		TTFTMs:    321,
		TPS:       12.5,
		LatencyMs: 1000,
	})

	if v, ok := c.Get("onr.ttft_ms"); !ok || v != int64(321) {
		t.Fatalf("expected ttft_ms=321, got ok=%v value=%v", ok, v)
	}
	if v, ok := c.Get("onr.tps"); !ok || v != 12.5 {
		t.Fatalf("expected tps=12.5, got ok=%v value=%v", ok, v)
	}
}

func TestSetProxyResultContext_UsageExtraFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setProxyResultContext(c, &proxy.Result{
		Status: http.StatusOK,
		Usage: map[string]any{
			"input_tokens":              12,
			"cache_write_ttl_5m_tokens": 6802,
			"cache_write_ttl_1h_tokens": 0,
		},
	})

	if v, ok := c.Get("onr.usage_extra.cache_write_ttl_5m_tokens"); !ok || v != 6802 {
		t.Fatalf("expected cache_write_ttl_5m_tokens=6802, got ok=%v value=%v", ok, v)
	}
	if v, ok := c.Get("onr.usage_extra.cache_write_ttl_1h_tokens"); !ok || v != 0 {
		t.Fatalf("expected cache_write_ttl_1h_tokens=0, got ok=%v value=%v", ok, v)
	}
}

func TestSetProxyResultContext_CostFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setProxyResultContext(c, &proxy.Result{
		Status: http.StatusOK,
		Cost: map[string]any{
			"cost_total":            0.12,
			"cost_input":            0.03,
			"cost_output":           0.04,
			"cost_cache_read":       0.01,
			"cost_cache_write":      0.02,
			"billable_input_tokens": 123,
			"cost_multiplier":       1.5,
			"cost_model":            "gpt-4o-mini",
			"cost_channel":          "openai/key1",
			"cost_unit":             "usd",
		},
	})

	for _, tc := range []struct {
		ctxKey string
		want   any
	}{
		{ctxKey: "onr.cost_total", want: 0.12},
		{ctxKey: "onr.cost_input", want: 0.03},
		{ctxKey: "onr.cost_output", want: 0.04},
		{ctxKey: "onr.cost_cache_read", want: 0.01},
		{ctxKey: "onr.cost_cache_write", want: 0.02},
		{ctxKey: "onr.billable_input_tokens", want: 123},
		{ctxKey: "onr.cost_multiplier", want: 1.5},
		{ctxKey: "onr.cost_model", want: "gpt-4o-mini"},
		{ctxKey: "onr.cost_channel", want: "openai/key1"},
		{ctxKey: "onr.cost_unit", want: "usd"},
	} {
		if v, ok := c.Get(tc.ctxKey); !ok || v != tc.want {
			t.Fatalf("expected %s=%v, got ok=%v value=%v", tc.ctxKey, tc.want, ok, v)
		}
	}
}
