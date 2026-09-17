package jsext

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHTTPResponsesLimitsAndCancellation(t *testing.T) {
	// Exercise real HTTP with an explicitly constructed test client. Production
	// clients always use publicDialContext, separately tested for private denial.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/large":
			_, _ = io.WriteString(w, strings.Repeat("x", 1025))
		case "/slow":
			<-r.Context().Done()
		case "/deny":
			w.WriteHeader(403)
			_, _ = io.WriteString(w, `{"allow":false}`)
		default:
			w.Header().Add("X-Multi", "a")
			w.Header().Add("X-Multi", "b")
			_, _ = io.WriteString(w, `{"allow":true}`)
		}
	}))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = rejectRedirect
	p := HTTPPolicy{MaxResponseBodyBytes: 1024}.clone()
	for _, path := range []string{"/ok", "/deny"} {
		result, err := executeHTTPRequest(context.Background(), p, httpRequest{URL: server.URL + path}, client)
		if err != nil {
			t.Fatal(err)
		}
		if path == "/deny" && result["status"] != 403 {
			t.Fatal(result)
		}
	}
	for _, path := range []string{"/redirect", "/large"} {
		if _, err := executeHTTPRequest(context.Background(), p, httpRequest{URL: server.URL + path}, client); err == nil {
			t.Fatal(path)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := executeHTTPRequest(ctx, p, httpRequest{URL: server.URL + "/slow"}, client); err == nil {
		t.Fatal("missing cancellation")
	}
	if time.Since(start) > time.Second {
		t.Fatal("HTTP cancellation was not propagated")
	}
}
func TestHTTPBudgetAndStagePermissions(t *testing.T) {
	s := testSession(t, map[Stage]string{Request: `return;`}, Limits{}, nil)
	s.policy = HTTPPolicy{Enabled: true, AllowedOrigins: []string{"https://example.com"}, MaxCallsPerHook: 1, MaxRequestBodyBytes: 2}.clone()
	s.hookContext = context.Background()
	s.stage = Request
	if _, err := s.httpRequest(httpRequest{URL: "https://example.com", Body: "long"}); err == nil || !strings.Contains(err.Error(), "body") {
		t.Fatal(err)
	}
	if _, err := s.httpRequest(httpRequest{URL: "https://example.com"}); err == nil || !strings.Contains(err.Error(), "call limit") {
		t.Fatal(err)
	}
	for _, stage := range []Stage{ResponseHeaders, SSEEvent, Log} {
		s.stage = stage
		if _, err := s.httpRequest(httpRequest{URL: "https://example.com"}); err == nil {
			t.Fatal(stage)
		}
	}
	for _, origin := range []string{"file:///etc/passwd", "https://example.com/path", "https://user:pass@example.com", "https://*.example.com"} {
		p := HTTPPolicy{AllowedOrigins: []string{origin}}

		if err := p.validate(); err == nil {
			t.Fatal(origin)
		}
	}
	a, _ := url.Parse("https://EXAMPLE.COM:443")
	b, _ := url.Parse("https://example.com")
	if canonicalOrigin(a) != canonicalOrigin(b) {
		t.Fatal("default port / DNS case mismatch")
	}
}
