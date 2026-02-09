package trafficdump

import (
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestid"
)

func TestMaskIfNeeded_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		key  string
		val  string
		on   bool
		want string
	}{
		{name: "off_no_redact", key: "Authorization", val: "Bearer xxx", on: false, want: "Bearer xxx"},
		{name: "authorization", key: "Authorization", val: "Bearer xxx", on: true, want: "[REDACTED]"},
		{name: "api_key", key: "api-key", val: "abc", on: true, want: "[REDACTED]"},
		{name: "x_api_key", key: "x-api-key", val: "abc", on: true, want: "[REDACTED]"},
		{name: "cookie", key: "Cookie", val: "a=b", on: true, want: "[REDACTED]"},
		{name: "token_like", key: "X-Foo-Token", val: "abc", on: true, want: "[REDACTED]"},
		{name: "non_sensitive", key: "Content-Type", val: "application/json", on: true, want: "application/json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskIfNeeded(tc.key, tc.val, tc.on)
			if got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestMaskURLIfNeeded_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		in         string
		on         bool
		wantSubs   []string
		avoidSubs  []string
		wantSameAs bool
	}{
		{
			name:       "off",
			in:         "https://example.com/path?key=abc",
			on:         false,
			wantSameAs: true,
		},
		{
			name:     "gemini_key_redacted",
			in:       "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse&key=AIzaSy123",
			on:       true,
			wantSubs: []string{"key=%5BREDACTED%5D", "alt=sse"},
		},
		{
			name:     "token_like_keys_redacted",
			in:       "https://example.com/x?access_token=abc&foo=bar",
			on:       true,
			wantSubs: []string{"access_token=%5BREDACTED%5D", "foo=bar"},
		},
		{
			name:     "api_key_variants_redacted",
			in:       "https://example.com/x?apikey=abc&api_key=def&foo=bar",
			on:       true,
			wantSubs: []string{"apikey=%5BREDACTED%5D", "api_key=%5BREDACTED%5D", "foo=bar"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskURLIfNeeded(tc.in, tc.on)
			if tc.wantSameAs && got != tc.in {
				t.Fatalf("got=%q want=%q", got, tc.in)
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(got, sub) {
					t.Fatalf("expected %q in %q", sub, got)
				}
			}
			for _, sub := range tc.avoidSubs {
				if strings.Contains(got, sub) {
					t.Fatalf("unexpected %q in %q", sub, got)
				}
			}
		})
	}
}

func TestRedactImageBase64Fields_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no_change",
			in:   `{"foo":"bar"}`,
			want: `{"foo":"bar"}`,
		},
		{
			name: "b64_json_redacted",
			in:   `{"b64_json":"AAAA","foo":"bar"}`,
			want: `{"b64_json":"[[OMITTED]]","foo":"bar"}`,
		},
		{
			name: "image_and_mask_redacted",
			in:   `{"image":"AAAA","mask":"BBBB","other":"ok"}`,
			want: `{"image":"[[OMITTED]]","mask":"[[OMITTED]]","other":"ok"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(redactImageBase64Fields([]byte(tc.in)))
			if got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestLimitBytes_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        []byte
		max       int
		wantLen   int
		truncated bool
	}{
		{name: "max_zero", in: []byte("abc"), max: 0, wantLen: 0, truncated: false},
		{name: "max_negative", in: []byte("abc"), max: -1, wantLen: 0, truncated: false},
		{name: "short", in: []byte("abc"), max: 10, wantLen: 3, truncated: false},
		{name: "equal", in: []byte("abc"), max: 3, wantLen: 3, truncated: false},
		{name: "truncate", in: []byte("abcd"), max: 3, wantLen: 3, truncated: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, tr := LimitBytes(tc.in, tc.max)
			if len(out) != tc.wantLen || tr != tc.truncated {
				t.Fatalf("len(out)=%d tr=%v wantLen=%d wantTr=%v", len(out), tr, tc.wantLen, tc.truncated)
			}
		})
	}
}

func TestStartWithRequestID_WritesAndMasksHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()
	cfg := Config{
		Enabled:     true,
		Dir:         tmp,
		FilePath:    "{{.request_id}}.log",
		MaxBytes:    1024 * 1024,
		MaskSecrets: true,
	}

	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest("POST", "/v1/test?api_key=abc&foo=bar", strings.NewReader(`{"x":1}`))
	gc.Request.Header.Set("Authorization", "Bearer should-redact")
	gc.Request.Header.Set("Content-Type", "application/json")

	rec, err := StartWithRequestID(gc, cfg, "rid_test_1")
	if err != nil {
		t.Fatalf("StartWithRequestID error: %v", err)
	}
	t.Cleanup(func() { rec.Close() })

	AppendOriginRequest(gc, []byte(`{"x":1}`), false, false)
	rec.Close()

	path := filepath.Join(tmp, "rid_test_1.log")
	// #nosec G304 -- test reads a file path constructed from t.TempDir().
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump file: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "=== META ===") || !strings.Contains(s, "request_id=rid_test_1") {
		t.Fatalf("unexpected dump meta:\n%s", s)
	}
	if !strings.Contains(s, "Authorization: [REDACTED]") {
		t.Fatalf("expected Authorization redacted:\n%s", s)
	}
	// query redaction is best-effort: it should not leak api_key.
	if strings.Contains(s, "api_key=abc") {
		t.Fatalf("expected api_key redacted in path:\n%s", s)
	}
	if !strings.Contains(s, "foo=bar") {
		t.Fatalf("expected non-sensitive query preserved:\n%s", s)
	}
	if !strings.Contains(s, "=== ORIGIN REQUEST ===") {
		t.Fatalf("expected origin request section:\n%s", s)
	}
}

func TestRecorderAppendSections_BinaryAndStreamSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()
	cfg := Config{
		Enabled:     true,
		Dir:         tmp,
		FilePath:    "{{.request_id}}.log",
		MaxBytes:    1024 * 1024,
		MaskSecrets: true,
	}

	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest("POST", "/v1/test?token=abc&foo=bar", strings.NewReader(`{}`))
	gc.Request.Header.Set("Authorization", "Bearer should-redact")

	rec, err := StartWithRequestID(gc, cfg, "rid_test_2")
	if err != nil {
		t.Fatalf("StartWithRequestID error: %v", err)
	}
	t.Cleanup(func() { rec.Close() })

	if got := Enabled(cfg); !got {
		t.Fatalf("Enabled returned false")
	}
	if got := rec.MaxBytes(); got != cfg.MaxBytes {
		t.Fatalf("MaxBytes=%d want=%d", got, cfg.MaxBytes)
	}

	AppendUpstreamRequest(gc, "POST", "https://example.com/x?token=abc&foo=bar", map[string][]string{
		"Authorization": {"Bearer aaa"},
		"Content-Type":  {"application/octet-stream"},
	}, []byte{0x00, 0x01, 0x02}, true, false)

	AppendUpstreamResponse(gc, "200 OK", map[string][]string{
		"Content-Type": {"application/octet-stream"},
	}, []byte{0x10, 0x11, 0x12}, true, false)

	AppendProxyResponse(gc, []byte{0x20, 0x21, 0x22}, true, false, 200)
	AppendStreamSummary(gc, 123, "context canceled", true)

	rec.Close()

	path := filepath.Join(tmp, "rid_test_2.log")
	// #nosec G304 -- test reads a file path constructed from t.TempDir().
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump file: %v", err)
	}
	s := string(b)

	if !strings.Contains(s, "=== UPSTREAM REQUEST ===") ||
		!strings.Contains(s, "=== UPSTREAM RESPONSE ===") ||
		!strings.Contains(s, "=== PROXY RESPONSE ===") ||
		!strings.Contains(s, "=== STREAM ===") {
		t.Fatalf("missing expected sections:\n%s", s)
	}
	if strings.Contains(s, "token=abc") {
		t.Fatalf("expected token query redacted:\n%s", s)
	}
	if !strings.Contains(s, "foo=bar") {
		t.Fatalf("expected foo preserved:\n%s", s)
	}
	if !strings.Contains(s, "[base64]") {
		t.Fatalf("expected binary sections base64 encoded:\n%s", s)
	}
	if !strings.Contains(s, base64.StdEncoding.EncodeToString([]byte{0x10, 0x11, 0x12})) {
		t.Fatalf("expected upstream binary content included as base64:\n%s", s)
	}
	if !strings.Contains(s, "bytes_copied=123") || !strings.Contains(s, "ignored_client_disconnect=true") {
		t.Fatalf("expected stream summary fields:\n%s", s)
	}
}

func TestRequestID_PrefersHeaderWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(w)
	gc.Request = httptest.NewRequest("GET", "/v1/test", nil)
	gc.Request.Header.Set(requestid.HeaderKey, "rid_header_1")

	if got := RequestID(gc); got != "rid_header_1" {
		t.Fatalf("got=%q want=%q", got, "rid_header_1")
	}
}
