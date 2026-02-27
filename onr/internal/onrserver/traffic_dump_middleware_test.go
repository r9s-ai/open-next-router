package onrserver

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/pkg/config"
)

func TestTrafficDumpMiddleware_LogsWarningWhenStartFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()
	badDir := filepath.Join(tmp, "dumps-as-file")
	if err := os.WriteFile(badDir, []byte("x"), 0o600); err != nil {
		t.Fatalf("write bad dir file: %v", err)
	}

	cfg := &config.Config{}
	cfg.TrafficDump.Enabled = true
	cfg.TrafficDump.Dir = badDir
	cfg.TrafficDump.FilePath = "{{.request_id}}.log"
	cfg.TrafficDump.MaxBytes = 1024
	cfg.TrafficDump.MaskSecrets = true

	var out bytes.Buffer
	oldOut := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&out)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	}()

	r := gin.New()
	r.Use(trafficDumpMiddleware(cfg, "X-Onr-Request-Id"))
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d", http.StatusOK, w.Code)
	}
	got := out.String()
	if !strings.Contains(got, "traffic dump start failed") {
		t.Fatalf("expected warning log for traffic dump start failure, got=%q", got)
	}
}
