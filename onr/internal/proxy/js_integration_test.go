package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

func jsClient(t *testing.T, url, source string) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.conf")
	source = strings.ReplaceAll(source, "UPSTREAM_URL", url)
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	reg := dslconfig.NewRegistry()
	if err := reg.ConfigureJS(jsext.RuntimeConfig{Root: "testdata/js"}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ReloadFromDir(dir); err != nil {
		t.Fatal(err)
	}
	return &Client{Registry: reg, HTTP: &http.Client{}, WriteTimeout: time.Second * 5}, path
}
func TestJSIntegrationSixStages(t *testing.T) {
	for _, stream := range []bool{false, true} {
		name := "json"
		if stream {
			name = "sse"
		}
		t.Run(name, func(t *testing.T) {
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["checked"] != true || body["mapped"] != "yes" {
					t.Errorf("after map not applied: %v", body)
				}
				messages, _ := body["messages"].([]any)
				if len(messages) != 2 {
					t.Errorf("prompt not added exactly once: %v", messages)
				}
				if r.Header.Get("Authorization") != "Bearer secret" || len(r.Header.Values("X-Policy")) != 2 {
					t.Errorf("headers: %v", r.Header)
				}
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":4}}\n\ndata: [DONE]\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":4}}`)
				}
			}))
			defer mock.Close()
			client, _ := jsClient(t, mock.URL, string(mustReadTestData(t, "js/provider.conf")))
			gc, rec := newGinJSONRequest(t, []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
			res, err := client.ProxyJSON(gc, "policy", ProviderKey{Value: "secret"}, "chat.completions", stream)
			if err != nil {
				t.Fatal(err)
			}
			if rec.Header().Get("X-Policy") != "onr-js" || !strings.Contains(rec.Body.String(), `"reviewed":true`) || strings.Contains(rec.Body.String(), `"usage"`) {
				t.Fatalf("unexpected downstream: %v %s", rec.Header(), rec.Body.String())
			}
			if res == nil || asInt(res.Usage["input_tokens"]) != 12 || asInt(res.Usage["output_tokens"]) != 4 {
				t.Fatalf("usage changed by JS: %+v", res)
			}
			if stream && strings.Count(rec.Body.String(), "[DONE]") != 1 {
				t.Fatal("terminal marker lost")
			}
		})
	}
}
func TestJSRejectDoesNotReachUpstream(t *testing.T) {
	var calls atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer mock.Close()
	client, _ := jsClient(t, mock.URL, string(mustReadTestData(t, "js/provider.conf")))
	gc, _ := newGinJSONRequest(t, []byte(`{"model":"m"}`))
	_, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", false)
	var rejection *jsext.Rejection
	if !errors.As(err, &rejection) || rejection.Status != 400 || calls.Load() != 0 {
		t.Fatalf("%v calls=%d", err, calls.Load())
	}
}
func TestJSErrorPreservesUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "sse"}[stream], func(t *testing.T) {
			source := string(mustReadTestData(t, "js/provider.conf"))
			if stream {
				source = strings.Replace(source, `if (body.usage) { return null; }`, `if (body.usage) { throw new Error("policy failed"); }`, 1)
			} else {
				source = strings.Replace(source, `delete body.usage;`, `throw new Error("policy failed");`, 1)
			}
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, "data: {\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":4}}\n\n")
				} else {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"usage":{"prompt_tokens":12,"completion_tokens":4}}`)
				}
			}))
			defer mock.Close()
			client, _ := jsClient(t, mock.URL, source)
			gc, _ := newGinJSONRequest(t, []byte(`{"model":"m","messages":[]}`))
			res, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", stream)
			if !jsext.Terminal(err) || res == nil || asInt(res.Usage["input_tokens"]) != 12 {
				t.Fatalf("facts lost: res=%+v err=%v", res, err)
			}
		})
	}
}
func TestJSLongStreamPinsSnapshot(t *testing.T) {
	first := make(chan struct{})
	release := make(chan struct{})
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"a\":1}\n\n")
		w.(http.Flusher).Flush()
		close(first)
		<-release
		_, _ = io.WriteString(w, "data: {\"a\":2}\n\ndata: [DONE]\n\n")
	}))
	defer mock.Close()
	source := string(mustReadTestData(t, "js/provider.conf"))
	client, path := jsClient(t, mock.URL, source)
	gc, rec := newGinJSONRequest(t, []byte(`{"model":"m","messages":[]}`))
	done := make(chan error, 1)
	go func() { _, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", true); done <- err }()
	<-first
	next := strings.ReplaceAll(strings.ReplaceAll(source, "UPSTREAM_URL", mock.URL), "body.reviewed = true;", "body.reviewed = false;")
	if err := os.WriteFile(path, []byte(next), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Registry.ReloadFromDir(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream stuck")
	}
	if strings.Count(rec.Body.String(), `"reviewed":true`) != 2 || strings.Contains(rec.Body.String(), `"reviewed":false`) {
		t.Fatal(rec.Body.String())
	}
}
func TestJSCanceledAttempt(t *testing.T) {
	client, _ := jsClient(t, "https://example.invalid", string(mustReadTestData(t, "js/provider.conf")))
	gc, _ := newGinJSONRequest(t, []byte(`{"model":"m","messages":[]}`))
	ctx, cancel := context.WithCancel(gc.Request.Context())
	cancel()
	gc.Request = gc.Request.WithContext(ctx)
	if _, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", false); err == nil {
		t.Fatal("canceled attempt accepted")
	}
}

func TestJSOAuthRetryStartsFreshAttempt(t *testing.T) {
	var upstreamCalls, tokenCalls atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			n := tokenCalls.Add(1)
			_, _ = io.WriteString(w, `{"access_token":"token-`+strconv.Itoa(int(n))+`","expires_in":3600}`)
			return
		}
		attempt := upstreamCalls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		messages, _ := body["messages"].([]any)
		if len(messages) != 2 {
			t.Errorf("request rewrites accumulated: %v", messages)
		}
		if r.Header.Get("X-Attempt") != strconv.Itoa(int(attempt)) {
			t.Errorf("bad attempt: %s", r.Header.Get("X-Attempt"))
		}
		if attempt == 1 {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":"expired"}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)
	}))
	defer mock.Close()
	source := providerConfOAuthCustom(mock.URL)
	source = strings.Replace(source, "defaults {", `defaults {
 request {request_by_js_block {
  if(ctx.state.seen)throw new Error("state leaked across attempts");ctx.state.seen=true;
  const body=JSON.parse(ctx.request.body);body.messages.unshift({role:"system",content:"once"});ctx.request.body=JSON.stringify(body);
  ctx.request.headers["x-attempt"]=[String(ctx.meta.attempt)];
 }}
 log_by_js_block {ctx.log.info("attempt",{attempt:ctx.meta.attempt});}
 `, 1)
	client := newMockE2EClient(t, map[string]string{"openai.conf": source})
	var log bytes.Buffer
	color := false
	logger, err := logx.NewSystemLoggerWithOptions(logx.SystemLoggerOptions{Writer: &log, Level: "debug", Color: &color})
	if err != nil {
		t.Fatal(err)
	}
	client.SystemLogger = logger
	gc, _ := newGinJSONRequest(t, []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	res, err := client.ProxyJSON(gc, "openai", ProviderKey{Value: "refresh-secret"}, "chat.completions", false)
	if err != nil || res == nil || res.Status != 200 || upstreamCalls.Load() != 2 {
		t.Fatalf("res=%+v err=%v calls=%d", res, err, upstreamCalls.Load())
	}
	if strings.Count(log.String(), "JavaScript: attempt") != 2 {
		t.Fatalf("log not exactly once per attempt: %s", log.String())
	}
}

func TestJSDispatchUsesActualResponseFormat(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"error":"invalid input"}`)
	}))
	defer mock.Close()
	client, _ := jsClient(t, mock.URL, string(mustReadTestData(t, "js/provider.conf")))
	gc, rec := newGinJSONRequest(t, []byte(`{"model":"m","messages":[],"stream":true}`))
	_, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", true)
	if err != nil || !strings.Contains(rec.Body.String(), `"reviewed":true`) {
		t.Fatalf("JSON response was treated as SSE: %v %s", err, rec.Body.String())
	}
}

func TestJSAfterMapRunsBeforeBedrockSigning(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if r.Header.Get("X-Policy") != "checked" || !strings.Contains(authorization, "x-policy") {
			t.Errorf("JS header was not signed: %v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["anthropic_version"] != "bedrock-2023-05-31" {
			t.Errorf("mapping did not complete: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer mock.Close()
	source := providerConfAWSBedrock(mock.URL)
	source = strings.Replace(source, "after_req_map {", `after_req_map {request_by_js_block {ctx.request.headers["x-policy"]=["checked"];}`, 1)
	client := newMockE2EClient(t, map[string]string{"aws-bedrock.conf": source})
	gc, _ := newGinJSONRequest(t, []byte(`{"model":"claude-haiku-4-5","messages":[{"role":"user","content":"hi"}]}`))
	_, err := client.ProxyJSON(gc, "aws-bedrock", ProviderKey{AWSAccessKeyID: "test-id", AWSSecretAccessKey: "test-secret", AWSRegion: "us-east-1"}, "chat.completions", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJSPolicyFixtureRequiresHostAuthorization(t *testing.T) {
	var calls atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer mock.Close()
	source := `provider "policy" {defaults {upstream_config {base_url="UPSTREAM_URL";} request {after_req_map {request_by_js_file policy-check.js;}}} match api="chat.completions" {upstream {set_path "/v1/chat/completions";}}}`
	client, _ := jsClient(t, mock.URL, source)
	gc, _ := newGinJSONRequest(t, []byte(`{"model":"m","messages":[]}`))
	_, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", false)
	if !jsext.Terminal(err) || !strings.Contains(err.Error(), "not authorized") || calls.Load() != 0 {
		t.Fatalf("audit policy bypassed: %v calls=%d", err, calls.Load())
	}
}

func TestJSResponseLimitAppliesAfterDecompression(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = io.WriteString(gz, `{"padding":"`+strings.Repeat("x", 2048)+`"}`)
		_ = gz.Close()
	}))
	defer mock.Close()
	source := strings.Replace(string(mustReadTestData(t, "js/provider.conf")), "js_timeout 200ms;", "js_timeout 200ms; js_body_limit 512;", 1)
	client, _ := jsClient(t, mock.URL, source)
	gc, _ := newGinJSONRequest(t, []byte(`{"model":"m","messages":[]}`))
	_, err := client.ProxyJSON(gc, "policy", ProviderKey{}, "chat.completions", false)
	if !jsext.Terminal(err) || !strings.Contains(err.Error(), "js_body_limit") {
		t.Fatalf("decoded body limit bypassed: %v", err)
	}
}
