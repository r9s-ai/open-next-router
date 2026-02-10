package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestE2EMock_ChatCompletions_AnthropicMessages_NonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadTestData(t, "mock_upstream/anthropic/messages_nonstream_ok.json")
	fixtureReq := mustReadTestData(t, "fixtures/chat_nonstream_request.json")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if strings.TrimSpace(asString(req["model"])) != "claude-haiku-4-5" {
			t.Fatalf("unexpected upstream model: %#v", req["model"])
		}
		if asInt(req["max_tokens"]) != 32 {
			t.Fatalf("unexpected upstream max_tokens: %#v", req["max_tokens"])
		}
		msgs, _ := req["messages"].([]any)
		if len(msgs) != 1 {
			t.Fatalf("unexpected upstream messages length: %d", len(msgs))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"anthropic.conf": providerConfAnthropic(mock.URL),
	})
	gc, rec := newGinJSONRequest(t, fixtureReq)
	res, err := c.ProxyJSON(gc, "anthropic", ProviderKey{Name: "anthropic-key", Value: "mock-key"}, "chat.completions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}

	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("unexpected status: %d", got)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode downstream body: %v", err)
	}
	choices, _ := out["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("unexpected choices length: %d", len(choices))
	}
	ch0, _ := choices[0].(map[string]any)
	msg, _ := ch0["message"].(map[string]any)
	if asString(msg["role"]) != "assistant" || asString(msg["content"]) != "Hi" {
		t.Fatalf("unexpected assistant message: %#v", msg)
	}
	if asString(ch0["finish_reason"]) != "stop" {
		t.Fatalf("unexpected finish_reason: %#v", ch0["finish_reason"])
	}
	usage, _ := out["usage"].(map[string]any)
	if asInt(usage["prompt_tokens"]) != 12 || asInt(usage["completion_tokens"]) != 4 || asInt(usage["total_tokens"]) != 16 {
		t.Fatalf("unexpected usage: %#v", usage)
	}

	assertGolden(t, "golden/anthropic_nonstream_openai_chat.json", normalizeForGolden(rec.Body.String()))
}

func TestE2EMock_ChatCompletions_AnthropicMessages_StreamToolUse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadTestData(t, "mock_upstream/anthropic/messages_stream_tool_use.sse")
	fixtureReq := mustReadTestData(t, "fixtures/chat_stream_tool_request.json")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if s, ok := req["stream"].(bool); !ok || !s {
			t.Fatalf("expected upstream stream=true, got %#v", req["stream"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"anthropic.conf": providerConfAnthropic(mock.URL),
	})
	gc, rec := newGinJSONRequest(t, fixtureReq)
	res, err := c.ProxyJSON(gc, "anthropic", ProviderKey{Name: "anthropic-key", Value: "mock-key"}, "chat.completions", true)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}

	body := rec.Body.String()
	if !containsAll(body,
		`"object":"chat.completion.chunk"`,
		`"tool_calls"`,
		`"name":"get_weather"`,
		`"arguments":"{\"city\":\"SF\"}"`,
		`"finish_reason":"tool_calls"`,
		"data: [DONE]",
	) {
		t.Fatalf("unexpected stream body:\n%s", body)
	}
	if strings.Count(body, "data: [DONE]") != 1 {
		t.Fatalf("expected one [DONE], got body:\n%s", body)
	}
	assertGolden(t, "golden/anthropic_stream_tool_use_openai_chat.sse", normalizeForGolden(body))
}

func TestE2EMock_ChatCompletions_Gemini_Stream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadTestData(t, "mock_upstream/gemini/stream_generate_content_ok.sse")
	fixtureReq := mustReadTestData(t, "fixtures/chat_nonstream_request.json")
	var gotPath string
	var gotAlt string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAlt = r.URL.Query().Get("alt")
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"gemini.conf": providerConfGemini(mock.URL),
	})
	gc, rec := newGinJSONRequest(t, fixtureReq)
	res, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "chat.completions", true)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if !strings.HasPrefix(gotPath, "/v1beta/models/") || !strings.HasSuffix(gotPath, ":streamGenerateContent") {
		t.Fatalf("unexpected gemini upstream path: %s", gotPath)
	}
	if gotAlt != "sse" {
		t.Fatalf("unexpected gemini alt query: %q", gotAlt)
	}

	body := rec.Body.String()
	if !containsAll(body,
		`"object":"chat.completion.chunk"`,
		`"role":"assistant"`,
		`"content":"Hi"`,
		`"finish_reason":"stop"`,
		`"choices":[]`,
		`"total_tokens":3`,
		"data: [DONE]",
	) {
		t.Fatalf("unexpected stream body:\n%s", body)
	}
	assertGolden(t, "golden/gemini_stream_openai_chat.sse", normalizeForGolden(body))
}

func TestE2EMock_ChatCompletions_AzureResponses_StreamCompletedEarly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadTestData(t, "mock_upstream/azure_response/responses_stream_completed_early.sse")
	fixtureReq := mustReadTestData(t, "fixtures/chat_nonstream_request.json")

	var gotPath string
	var gotAPIVersion string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIVersion = r.URL.Query().Get("api-version")
		if r.Method != http.MethodPost || r.URL.Path != "/openai/responses" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"azure-response.conf": providerConfAzureResponse(mock.URL),
	})
	gc, rec := newGinJSONRequest(t, fixtureReq)
	res, err := c.ProxyJSON(gc, "azure-response", ProviderKey{Name: "azure-key", Value: "mock-key"}, "chat.completions", true)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if gotPath != "/openai/responses" {
		t.Fatalf("unexpected azure upstream path: %s", gotPath)
	}
	if gotAPIVersion != "2025-04-01-preview" {
		t.Fatalf("unexpected azure api-version: %q", gotAPIVersion)
	}

	body := rec.Body.String()
	if strings.Count(body, "data: [DONE]") != 1 {
		t.Fatalf("DONE should appear once, got body:\n%s", body)
	}
	deltaPos := strings.Index(body, `"delta":{"content":"Hello"}`)
	donePos := strings.LastIndex(body, "data: [DONE]")
	if deltaPos < 0 || donePos < 0 || deltaPos > donePos {
		t.Fatalf("DONE should be emitted after delta, got body:\n%s", body)
	}
	assertGolden(t, "golden/azure_responses_stream_completed_early_openai_chat.sse", normalizeForGolden(body))
}

func newMockE2EClient(t *testing.T, confByFile map[string]string) *Client {
	t.Helper()
	reg := dslconfig.NewRegistry()

	dir := t.TempDir()
	for file, conf := range confByFile {
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
			t.Fatalf("write temp provider conf %s: %v", file, err)
		}
	}
	if _, err := reg.ReloadFromDir(dir); err != nil {
		t.Fatalf("reload registry: %v", err)
	}

	return &Client{
		HTTP:         &http.Client{Timeout: 5 * time.Second},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Registry:     reg,
	}
}

func providerConfAnthropic(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "anthropic" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_header_key "x-api-key";
    }
    request {
      set_header "anthropic-version" "2023-06-01";
    }
    response {
      resp_passthrough;
    }
  }

  match api = "chat.completions" stream = false {
    request {
      req_map openai_chat_to_anthropic_messages;
      json_del "$.stream_options";
    }
    upstream {
      set_path "/v1/messages";
    }
    response {
      resp_map anthropic_to_openai_chat;
    }
  }

  match api = "chat.completions" stream = true {
    request {
      req_map openai_chat_to_anthropic_messages;
      json_del "$.stream_options";
    }
    upstream {
      set_path "/v1/messages";
    }
    response {
      sse_parse anthropic_to_openai_chunks;
    }
  }
}
`, baseURL)
}

func providerConfGemini(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "gemini" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_header_key "x-goog-api-key";
    }
    response {
      resp_passthrough;
    }
  }

  match api = "chat.completions" stream = true {
    request {
      req_map openai_chat_to_gemini_generate_content;
      model_map_default $request.model;
      json_set "$.model" $request.model_mapped;
    }
    upstream {
      set_path concat("/v1beta/models/", $request.model_mapped, ":streamGenerateContent");
      set_query alt "sse";
    }
    response {
      sse_parse gemini_to_openai_chat_chunks;
    }
  }
}
`, baseURL)
}

func providerConfAzureResponse(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "azure-response" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_header_key "api-key";
    }
    response {
      resp_passthrough;
    }
  }

  match api = "chat.completions" stream = true {
    request {
      req_map openai_chat_to_openai_responses;
      set_header "Accept-Encoding" "identity";
    }
    upstream {
      set_path "/openai/responses";
      set_query "api-version" "2025-04-01-preview";
    }
    response {
      sse_parse openai_responses_to_openai_chat_chunks;
    }
  }
}
`, baseURL)
}

func newGinJSONRequest(t *testing.T, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	gc.Request = req
	return gc, rec
}

func mustReadTestData(t *testing.T, rel string) []byte {
	t.Helper()
	p := filepath.Join("testdata", rel)
	// #nosec G304 -- testdata path is fixed by test code, not user input.
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read testdata %s: %v", p, err)
	}
	return b
}

func assertGolden(t *testing.T, rel, actual string) {
	t.Helper()
	p := filepath.Join("testdata", rel)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(p, []byte(actual), 0o600); err != nil {
			t.Fatalf("write golden %s: %v", p, err)
		}
		return
	}
	// #nosec G304 -- golden path is fixed by test code, not user input.
	wantBytes, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden %s: %v", p, err)
	}
	want := strings.TrimSpace(string(wantBytes))
	got := strings.TrimSpace(actual)
	if got != want {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", p, got, want)
	}
}

func normalizeForGolden(s string) string {
	out := strings.TrimSpace(s)
	reCreated := regexp.MustCompile(`"created":\d+`)
	out = reCreated.ReplaceAllString(out, `"created":0`)
	reID := regexp.MustCompile(`"id":"chatcmpl_[^"]+"`)
	out = reID.ReplaceAllString(out, `"id":"chatcmpl_mock"`)
	return out
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
