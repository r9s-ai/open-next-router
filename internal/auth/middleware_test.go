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
