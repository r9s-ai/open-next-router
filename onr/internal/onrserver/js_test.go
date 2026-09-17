package onrserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func TestJSErrorHandlingAfterCommit(t *testing.T) {
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Data(200, "text/event-stream", []byte("data: first\n\n"))
	gc.Writer.Flush()
	writeProxyError(gc, "", &jsext.Error{Stage: jsext.SSEEvent, Cause: errors.New("script failed")})
	if rec.Body.String() != "data: first\n\n" {
		t.Fatalf("JSON appended to stream: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	gc, _ = gin.CreateTestContext(rec)
	writeProxyError(gc, "", &jsext.Rejection{Status: 403, Body: []byte(`{"error":"blocked"}`)})
	if rec.Code != 403 || rec.Body.String() != `{"error":"blocked"}` {
		t.Fatal(rec)
	}
}

func TestJSErrorBeforeCommitResetsUpstreamFraming(t *testing.T) {
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Header("Content-Type", "text/event-stream")
	gc.Header("Content-Encoding", "gzip")
	gc.Header("Content-Length", "999")
	writeProxyError(gc, "", &jsext.Error{Stage: jsext.ResponseHeaders, Cause: errors.New("failed")})
	if rec.Code != 500 || rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Content-Length") != "" || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid error framing: %v %s", rec.Header(), rec.Body.String())
	}
}

func TestJSAutoReloadAndRollback(t *testing.T) {
	for _, mode := range []string{"watch", "poll"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			scriptDir := filepath.Join(root, "scripts")
			if err := os.Mkdir(scriptDir, 0700); err != nil {
				t.Fatal(err)
			}
			providerPath := filepath.Join(root, "policy.conf")
			script := filepath.Join(scriptDir, "policy.js")
			if err := os.WriteFile(providerPath, []byte(`provider "policy" {defaults {upstream_config {base_url="https://example.com";} request {request_by_js_file policy.js;}}}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(script, []byte(`ctx.state.version=1;`), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{}
			cfg.Providers.Dir = root
			cfg.JS = jsext.RuntimeConfig{Root: scriptDir, Reload: mode}
			cfg.Providers.AutoReload.DebounceMs = 10
			reg := dslconfig.NewRegistry()
			if err := reg.ConfigureJS(cfg.JS); err != nil {
				t.Fatal(err)
			}
			if _, err := reg.ReloadFromPath(root); err != nil {
				t.Fatal(err)
			}
			color := false
			logger, err := logx.NewSystemLoggerWithOptions(logx.SystemLoggerOptions{Writer: io.Discard, Level: "debug", Color: &color})
			if err != nil {
				t.Fatal(err)
			}
			closer, err := installProvidersAutoReload(cfg, reg, &sync.Mutex{}, logger)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = closer.Close() }()
			waitVersion := func(want string) {
				t.Helper()
				deadline := time.Now().Add(4 * time.Second)
				for time.Now().Before(deadline) {
					pf, _ := reg.GetProvider("policy")
					if strings.Contains(pf.JS.Select("chat.completions", false).Handlers[jsext.Request].Body, want) {
						return
					}
					time.Sleep(20 * time.Millisecond)
				}
				t.Fatalf("%s reload did not reach %s", mode, want)
			}
			if err := os.WriteFile(script, []byte(`ctx.state.version=2;`), 0600); err != nil {
				t.Fatal(err)
			}
			waitVersion("version=2")
			if err := os.WriteFile(script, []byte(`invalid(`), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := reloadProvidersRuntime(cfg, reg, logger); err == nil {
				t.Fatal("invalid JS accepted")
			}
			waitVersion("version=2")
			if err := os.WriteFile(script, []byte(`ctx.state.version=3;`), 0600); err != nil {
				t.Fatal(err)
			}
			waitVersion("version=3")
		})
	}
}
