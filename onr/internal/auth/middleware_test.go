package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware_TokenKey_AccessKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	match := func(accessKey string) (string, bool) {
		if accessKey == "ak-1" {
			return "client1", true
		}
		return "", false
	}
	r := gin.New()
	r.Use(Middleware("master", match))
	r.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })

	k64 := base64.RawURLEncoding.EncodeToString([]byte("ak-1"))
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("Authorization", "Bearer onr:v1?k64="+k64+"&p=openai&m=gpt-4o-mini")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMiddleware_TokenKey_UK64(t *testing.T) {
	gin.SetMode(gin.TestMode)

	match := func(accessKey string) (string, bool) {
		if accessKey == "ak-1" {
			return "client1", true
		}
		return "", false
	}
	r := gin.New()
	r.Use(Middleware("master", match))
	r.GET("/ok", func(c *gin.Context) {
		if TokenUpstreamKey(c) != "sk-upstream" {
			c.String(http.StatusInternalServerError, "bad upstream")
			return
		}
		if TokenModeFromContext(c) != TokenModeBYOK {
			c.String(http.StatusInternalServerError, "bad mode")
			return
		}
		c.String(200, "ok")
	})

	k64 := base64.RawURLEncoding.EncodeToString([]byte("ak-1"))
	uk64 := base64.RawURLEncoding.EncodeToString([]byte("sk-upstream"))
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("Authorization", "Bearer onr:v1?k64="+k64+"&uk64="+uk64)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
