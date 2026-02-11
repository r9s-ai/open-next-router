package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
)

func TestShouldEstimateUsage(t *testing.T) {
	if !shouldEstimateUsage(http.StatusOK) {
		t.Fatalf("expected status 200 to enable usage estimation")
	}
	if shouldEstimateUsage(http.StatusBadGateway) {
		t.Fatalf("expected non-200 status to skip usage estimation")
	}
}

func TestHandleNonStreamResponse_SkipUsageEstimationOnNon200(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`)
	meta := &dslmeta.Meta{API: "chat.completions"}
	pf := dslconfig.ProviderFile{
		Finish: dslconfig.ProviderFinishReason{
			Defaults: dslconfig.FinishReasonExtractConfig{Mode: "openai"},
		},
	}
	client := &Client{
		UsageEst: &usageestimate.Config{
			Enabled:                   true,
			EstimateWhenMissingOrZero: true,
			MaxRequestBytes:           1 << 20,
			MaxResponseBytes:          1 << 20,
			APIs:                      []string{"chat.completions"},
		},
	}

	t.Run("status 200 keeps estimation", func(t *testing.T) {
		w := httptest.NewRecorder()
		gc, _ := gin.CreateTestContext(w)
		gc.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"cmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":"world"},"finish_reason":"stop"}]}`,
			)),
		}

		out, err := client.handleNonStreamResponse(
			gc,
			"openai",
			ProviderKey{Name: "test"},
			"chat.completions",
			false,
			time.Now(),
			pf,
			meta,
			"gpt-4o-mini",
			reqBody,
			dslconfig.ResponseDirective{},
			resp,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out == nil {
			t.Fatalf("expected non-nil result")
		}
		if out.Usage == nil {
			t.Fatalf("expected usage for status 200")
		}
		if strings.TrimSpace(out.UsageStage) == "" {
			t.Fatalf("expected usage_stage for status 200")
		}
		if out.FinishReason != "stop" {
			t.Fatalf("expected finish_reason=stop, got: %q", out.FinishReason)
		}
	})

	t.Run("non-200 skips estimation", func(t *testing.T) {
		w := httptest.NewRecorder()
		gc, _ := gin.CreateTestContext(w)
		gc.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))

		resp := &http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop"}],"error":{"message":"upstream failed"}}`)),
		}

		out, err := client.handleNonStreamResponse(
			gc,
			"openai",
			ProviderKey{Name: "test"},
			"chat.completions",
			false,
			time.Now(),
			pf,
			meta,
			"gpt-4o-mini",
			reqBody,
			dslconfig.ResponseDirective{},
			resp,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out == nil {
			t.Fatalf("expected non-nil result")
		}
		if out.Usage != nil {
			t.Fatalf("expected usage to be nil for non-200 status, got: %#v", out.Usage)
		}
		if strings.TrimSpace(out.UsageStage) != "" {
			t.Fatalf("expected empty usage_stage for non-200 status, got: %q", out.UsageStage)
		}
		if strings.TrimSpace(out.FinishReason) != "" {
			t.Fatalf("expected empty finish_reason for non-200 status, got: %q", out.FinishReason)
		}
	})
}
