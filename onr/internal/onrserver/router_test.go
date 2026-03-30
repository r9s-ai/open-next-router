package onrserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/pkg/config"
)

func TestNewRouter_RegistersImageAndAudioRoutes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := NewRouter(&config.Config{}, &state{}, nil, nil, nil, false, "X-Onr-Request-Id", nil)

	cases := []string{
		"/v1/images/generations",
		"/v1/images/edits",
		"/v1/audio/speech",
		"/v1/audio/transcriptions",
		"/v1/audio/translations",
	}

	for _, path := range cases {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Fatalf("expected route %q to be registered, got 404", path)
		}
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected route %q to stop at auth middleware with 401, got %d", path, w.Code)
		}
	}
}
