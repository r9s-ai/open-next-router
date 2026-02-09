package oauthclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_GetToken_CacheAndPersist(t *testing.T) {
	t.Parallel()

	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		n := tokenCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-" + strconvI32(n),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	c := New(srv.Client(), true, filepath.Join(dir, "oauth"))
	in := AcquireInput{
		CacheKey:      "k1",
		TokenURL:      srv.URL + "/oauth/token",
		Method:        http.MethodPost,
		ContentType:   "form",
		Form:          map[string]string{"grant_type": "refresh_token", "refresh_token": "rk"},
		TokenPath:     "$.access_token",
		ExpiresInPath: "$.expires_in",
		TokenTypePath: "$.token_type",
		Timeout:       3 * time.Second,
		RefreshSkew:   1 * time.Second,
		FallbackTTL:   30 * time.Minute,
	}

	tok1, err := c.GetToken(context.Background(), in)
	if err != nil {
		t.Fatalf("first get token err=%v", err)
	}
	tok2, err := c.GetToken(context.Background(), in)
	if err != nil {
		t.Fatalf("second get token err=%v", err)
	}
	if tok1.AccessToken != tok2.AccessToken {
		t.Fatalf("cache should return same token: %q vs %q", tok1.AccessToken, tok2.AccessToken)
	}
	if got := tokenCalls.Load(); got != 1 {
		t.Fatalf("token endpoint calls=%d want=1", got)
	}

	c2 := New(srv.Client(), true, filepath.Join(dir, "oauth"))
	tok3, err := c2.GetToken(context.Background(), in)
	if err != nil {
		t.Fatalf("third get token err=%v", err)
	}
	if tok3.AccessToken != tok1.AccessToken {
		t.Fatalf("persisted token mismatch: %q vs %q", tok3.AccessToken, tok1.AccessToken)
	}
	if got := tokenCalls.Load(); got != 1 {
		t.Fatalf("persisted token should avoid endpoint call, got=%d", got)
	}
}

func TestClient_Invalidate(t *testing.T) {
	t.Parallel()

	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		n := tokenCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok-" + strconvI32(n),
			"expires_in":   3600,
		})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.Client(), false, "")
	in := AcquireInput{
		CacheKey:      "k2",
		TokenURL:      srv.URL + "/oauth/token",
		Method:        http.MethodPost,
		ContentType:   "form",
		Form:          map[string]string{"grant_type": "refresh_token", "refresh_token": "rk"},
		TokenPath:     "$.access_token",
		ExpiresInPath: "$.expires_in",
		Timeout:       3 * time.Second,
		RefreshSkew:   time.Second,
		FallbackTTL:   30 * time.Minute,
	}

	if _, err := c.GetToken(context.Background(), in); err != nil {
		t.Fatalf("first get err=%v", err)
	}
	c.Invalidate("k2")
	if _, err := c.GetToken(context.Background(), in); err != nil {
		t.Fatalf("second get err=%v", err)
	}
	if got := tokenCalls.Load(); got != 2 {
		t.Fatalf("calls=%d want=2", got)
	}
}

func strconvI32(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}
