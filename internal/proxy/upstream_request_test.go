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
)

func TestDoUpstreamRequest_DoesNotCancelBodyImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/test", strings.NewReader(`{"x":1}`))

	c := &Client{WriteTimeout: 10 * time.Second}
	meta := &dslmeta.Meta{
		API:            "chat.completions",
		IsStream:       true,
		BaseURL:        srv.URL,
		RequestURLPath: "/",
	}

	resp, cancel, err := c.doUpstreamRequest(gc, "openai", dslconfig.ProviderFile{}, meta, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("doUpstreamRequest error: %v", err)
	}
	t.Cleanup(func() {
		_ = resp.Body.Close()
		cancel()
	})

	buf := make([]byte, 5)
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatalf("read body error: %v", err)
	}
	if got := string(buf); got != "hello" {
		t.Fatalf("unexpected body: %q", got)
	}
}
