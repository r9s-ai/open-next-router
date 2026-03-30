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

func TestDoUpstreamRequest_AppliesFilterHeaderValuesAfterSetHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	streamTrue := true

	gotHeader := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("X-Feature-Flags")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/test", strings.NewReader(`{"x":1}`))

	c := &Client{WriteTimeout: 10 * time.Second}
	pf := dslconfig.ProviderFile{
		Headers: dslconfig.ProviderHeaders{
			Defaults: dslconfig.PhaseHeaders{
				Request: []dslconfig.HeaderOp{
					{Op: "header_set", NameExpr: `"X-Feature-Flags"`, ValueExpr: `"exp-a; keep; debug"`},
				},
			},
			Matches: []dslconfig.MatchHeaders{
				{
					API:    "chat.completions",
					Stream: &streamTrue,
					Headers: dslconfig.PhaseHeaders{
						Request: []dslconfig.HeaderOp{
							{Op: "header_filter_values", NameExpr: `"X-Feature-Flags"`, Patterns: []string{"exp-*", "debug"}, Separator: ";"},
						},
					},
				},
			},
		},
	}
	meta := &dslmeta.Meta{
		API:            "chat.completions",
		IsStream:       true,
		BaseURL:        srv.URL,
		RequestURLPath: "/",
	}

	resp, cancel, err := c.doUpstreamRequest(gc, "openai", pf, meta, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("doUpstreamRequest error: %v", err)
	}
	t.Cleanup(func() {
		_ = resp.Body.Close()
		cancel()
	})

	select {
	case got := <-gotHeader:
		if got != "keep" {
			t.Fatalf("X-Feature-Flags=%q want=%q", got, "keep")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for upstream request")
	}
}

func TestDoUpstreamRequest_PassesHeaderFromOriginRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	streamTrue := true

	gotHeader := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader <- r.Header.Get("Anthropic-Beta")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"x":1}`))
	gc.Request.Header.Set("Anthropic-Beta", "context-1m-123213, asdb")

	c := &Client{WriteTimeout: 10 * time.Second}
	pf := dslconfig.ProviderFile{
		Headers: dslconfig.ProviderHeaders{
			Matches: []dslconfig.MatchHeaders{
				{
					API:    "claude.messages",
					Stream: &streamTrue,
					Headers: dslconfig.PhaseHeaders{
						Request: []dslconfig.HeaderOp{
							{Op: "header_pass", NameExpr: `"anthropic-beta"`},
						},
					},
				},
			},
		},
	}
	meta := &dslmeta.Meta{
		API:            "claude.messages",
		IsStream:       true,
		BaseURL:        srv.URL,
		RequestURLPath: "/",
	}

	resp, cancel, err := c.doUpstreamRequest(gc, "anthropic", pf, meta, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("doUpstreamRequest error: %v", err)
	}
	t.Cleanup(func() {
		_ = resp.Body.Close()
		cancel()
	})

	select {
	case got := <-gotHeader:
		if got != "context-1m-123213, asdb" {
			t.Fatalf("Anthropic-Beta=%q want=%q", got, "context-1m-123213, asdb")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for upstream request")
	}
}
