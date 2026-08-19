package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestRecordUpstreamRequestIDUsesConfiguredOrderAndTrims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	header := http.Header{}
	header.Set("X-Request-ID", "  ")
	header.Set("OpenAI-Request-ID", "  configured-id  ")
	header.Set("X-Implicit-ID", "must-not-be-used")
	resp := &http.Response{Header: header}
	recordUpstreamRequestID(c, dslconfig.ProviderFile{Observability: dslconfig.ProviderObservability{
		UpstreamRequestID: &dslconfig.UpstreamRequestIDRule{Headers: []string{"x-request-id", "openai-request-id"}},
	}}, resp)
	if got := c.GetString("onr.upstream_request_id"); got != "configured-id" {
		t.Fatalf("upstream_request_id=%q want configured-id", got)
	}
}

func TestDoUpstreamRequestRecordsProviderRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "  ")
		w.Header().Set("OpenAI-Request-ID", "upstream-123")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	gc, _ := gin.CreateTestContext(httptest.NewRecorder())
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	c := &Client{HTTP: &http.Client{Timeout: 3 * time.Second}, WriteTimeout: 3 * time.Second}
	pf := dslconfig.ProviderFile{Observability: dslconfig.ProviderObservability{
		UpstreamRequestID: &dslconfig.UpstreamRequestIDRule{Headers: []string{"x-request-id", "openai-request-id"}},
	}}
	resp, cancel, err := c.doUpstreamRequest(gc, "openai", &pf, &dslmeta.Meta{
		API: "chat.completions", BaseURL: srv.URL, RequestURLPath: "/",
	}, []byte(`{}`), "")
	if err != nil {
		t.Fatalf("doUpstreamRequest error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close(); cancel() })
	if got := gc.GetString("onr.upstream_request_id"); got != "upstream-123" {
		t.Fatalf("upstream_request_id=%q want upstream-123", got)
	}
}

func TestRecordUpstreamRequestIDMissingDoesNotSetOrForward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	recordUpstreamRequestID(c, dslconfig.ProviderFile{Observability: dslconfig.ProviderObservability{
		UpstreamRequestID: &dslconfig.UpstreamRequestIDRule{Headers: []string{"x-request-id"}},
	}}, &http.Response{Header: http.Header{"X-Other-Id": []string{"implicit-id"}}})
	if _, ok := c.Get("onr.upstream_request_id"); ok {
		t.Fatal("upstream_request_id should be absent when configured headers are missing")
	}
	if got := w.Header().Get("x-request-id"); got != "" {
		t.Fatalf("response unexpectedly forwarded upstream request ID: %q", got)
	}
}

func TestUpstreamRequestIDIsStillPassedThroughToDownstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	header := http.Header{}
	header.Set("X-Request-ID", "upstream-123")

	// Extraction records the value but does not mutate the upstream response headers.
	recordUpstreamRequestID(c, dslconfig.ProviderFile{Observability: dslconfig.ProviderObservability{
		UpstreamRequestID: &dslconfig.UpstreamRequestIDRule{Headers: []string{"x-request-id"}},
	}}, &http.Response{Header: header})
	copyHeadersToClient(c, header, false)

	if got := c.GetString("onr.upstream_request_id"); got != "upstream-123" {
		t.Fatalf("upstream_request_id=%q want upstream-123", got)
	}
	if got := w.Header().Get("X-Request-ID"); got != "upstream-123" {
		t.Fatalf("downstream X-Request-ID=%q want upstream-123", got)
	}
}

func TestRecordUpstreamRequestIDTruncatesAt256Bytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	header := http.Header{}
	header.Set("X-Request-ID", strings.Repeat("x", 300))
	recordUpstreamRequestID(c, dslconfig.ProviderFile{Observability: dslconfig.ProviderObservability{
		UpstreamRequestID: &dslconfig.UpstreamRequestIDRule{Headers: []string{"x-request-id"}},
	}}, &http.Response{Header: header})
	if got := c.GetString("onr.upstream_request_id"); len(got) != 256 {
		t.Fatalf("upstream_request_id length=%d want 256", len(got))
	}
}
