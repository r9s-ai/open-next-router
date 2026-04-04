package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
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
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
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

func TestE2EMock_AudioTranscriptions_OpenAI_Multipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotModel string
	var gotFileName string
	var gotContentType string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			name := strings.TrimSpace(part.FormName())
			if strings.TrimSpace(part.FileName()) != "" {
				gotFileName = strings.TrimSpace(part.FileName())
			}
			valueBytes, rerr := io.ReadAll(part)
			_ = part.Close()
			if rerr != nil {
				t.Fatalf("ReadAll part: %v", rerr)
			}
			if name == "model" {
				gotModel = strings.TrimSpace(string(valueBytes))
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"hello"}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAI(mock.URL),
	})
	gc, rec := newGinMultipartRequest(t, "/v1/audio/transcriptions", map[string]string{
		"model": "whisper-1",
	}, "file", "sample.wav", []byte("fake-audio"))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.transcriptions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if gotModel != "whisper-1" {
		t.Fatalf("unexpected upstream model: %q", gotModel)
	}
	if gotFileName != "sample.wav" {
		t.Fatalf("unexpected upstream filename: %q", gotFileName)
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(gotContentType)), "multipart/form-data;") {
		t.Fatalf("unexpected upstream content-type: %q", gotContentType)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"text":"hello"}` {
		t.Fatalf("unexpected downstream body: %s", rec.Body.String())
	}
}

func TestE2EMock_ImagesGenerations_OpenAI_StreamUsageFromLargeSSEEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	largeB64 := strings.Repeat("A", 400000)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w,
			"event: image_generation.completed\n"+
				`data: {"type":"image_generation.completed","b64_json":"`+largeB64+`","usage":{"input_tokens":12,"output_tokens":4508,"total_tokens":4520}}`+
				"\n\n",
		)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIImageStream(mock.URL),
	})
	gc, rec := newGinJSONRequest(t, []byte(`{"model":"gpt-image-1.5","prompt":"otter","stream":true}`))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "images.generations", true)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.Usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := asInt(res.Usage["input_tokens"]), 12; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 4508; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 4520; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_ImagesGenerations_OpenAI_MiniRealUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "images_generations_gpt_image_1_mini_real.json"))
	fixtureReq := mustReadTestData(t, "fixtures/image_generation_request.json")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIImages(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if got, want := asInt(res.Usage["input_tokens"]), 13; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 1056; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 1069; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["image_generate_images"]), 1; got != want {
		t.Fatalf("image_generate_images=%d want=%d", got, want)
	}
	if res.UsageStage != "upstream" {
		t.Fatalf("usage_stage=%q want=upstream", res.UsageStage)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_AudioTranscriptions_OpenAI_RealUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "audio_transcriptions_gpt_4o_mini_transcribe_real.json"))
	fileBody := mustReadSharedProxyTestData(t, filepath.Join("openai", "audio_speech_gpt_4o_mini_tts_real.mp3"))

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAI(mock.URL),
	})
	gc, rec := newGinMultipartRequest(t, "/v1/audio/transcriptions", map[string]string{
		"model": "gpt-4o-mini-transcribe",
	}, "file", "speech.mp3", fileBody)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.transcriptions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if got, want := asInt(res.Usage["input_tokens"]), 10; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 2; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 12; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if res.UsageStage != "upstream" {
		t.Fatalf("usage_stage=%q want=upstream", res.UsageStage)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_AudioSpeech_OpenAI_RealDerivedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "audio_speech_gpt_4o_mini_tts_real.mp3"))
	fixtureReq := mustReadTestData(t, "fixtures/audio_speech_request.json")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/speech" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIAudioSpeech(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/audio/speech", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.speech", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if got, want := asFloat(res.Usage["audio_tts_seconds"]), 2.352; got != want {
		t.Fatalf("audio_tts_seconds=%v want=%v", got, want)
	}
	if got := asInt(res.Usage["total_tokens"]); got != 0 {
		t.Fatalf("total_tokens=%d want=0", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
	if rec.Body.Len() != len(mockResp) {
		t.Fatalf("downstream body len=%d want=%d", rec.Body.Len(), len(mockResp))
	}
}

func TestE2EMock_AudioSpeech_OpenAI_RealDerivedUsageWithPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "audio_speech_gpt_4o_mini_tts_real.mp3"))
	fixtureReq := mustReadTestData(t, "fixtures/audio_speech_request.json")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/speech" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIAudioSpeech(mock.URL),
	})
	pricePath := filepath.Join(t.TempDir(), "price.yaml")
	priceYAML := `
version: v1
unit: usd_per_1m_tokens
entries:
  - provider: openai
    model: gpt-4o-mini-tts
    cost:
      audio_tts_seconds: 0.015
`
	if err := os.WriteFile(pricePath, []byte(priceYAML), 0o600); err != nil {
		t.Fatalf("write price: %v", err)
	}
	resolver, err := pricing.LoadResolver(pricePath, "")
	if err != nil {
		t.Fatalf("LoadResolver: %v", err)
	}
	c.SetPricingResolver(resolver)
	c.SetPricingEnabled(true)

	gc, _ := newGinJSONRequestPath(t, "/v1/audio/speech", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.speech", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.Cost == nil {
		t.Fatalf("expected cost")
	}
	if got, want := asFloat(res.Cost["cost_total"]), 0.03528; math.Abs(got-want) > 1e-9 {
		t.Fatalf("cost_total=%v want=%v", got, want)
	}
	if got, want := asString(res.Cost["cost_model"]), "gpt-4o-mini-tts"; got != want {
		t.Fatalf("cost_model=%q want=%q", got, want)
	}
}

func TestE2EMock_AudioSpeech_OpenAI_RealDerivedUsageWithOverrideOnlyPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "audio_speech_gpt_4o_mini_tts_real.mp3"))
	fixtureReq := mustReadTestData(t, "fixtures/audio_speech_request.json")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/speech" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIAudioSpeech(mock.URL),
	})
	pricePath := filepath.Join(t.TempDir(), "price.yaml")
	priceYAML := `
version: v1
unit: usd_per_1m_tokens
entries:
  - provider: openai
    model: gpt-4o-mini
    cost:
      input: 0.15
      output: 0.60
`
	if err := os.WriteFile(pricePath, []byte(priceYAML), 0o600); err != nil {
		t.Fatalf("write price: %v", err)
	}
	overridesPath := filepath.Join(t.TempDir(), "price_overrides.yaml")
	overridesYAML := `
version: v1
providers:
  openai:
    models:
      gpt-4o-mini-tts:
        cost:
          audio_tts_seconds: 0.015
`
	if err := os.WriteFile(overridesPath, []byte(overridesYAML), 0o600); err != nil {
		t.Fatalf("write overrides: %v", err)
	}
	resolver, err := pricing.LoadResolver(pricePath, overridesPath)
	if err != nil {
		t.Fatalf("LoadResolver: %v", err)
	}
	c.SetPricingResolver(resolver)
	c.SetPricingEnabled(true)

	gc, _ := newGinJSONRequestPath(t, "/v1/audio/speech", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.speech", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if res.Cost == nil {
		t.Fatalf("expected cost")
	}
	if got, want := asFloat(res.Cost["cost_total"]), 0.03528; math.Abs(got-want) > 1e-9 {
		t.Fatalf("cost_total=%v want=%v", got, want)
	}
	if got, want := asString(res.Cost["cost_model"]), "gpt-4o-mini-tts"; got != want {
		t.Fatalf("cost_model=%q want=%q", got, want)
	}
}

func TestE2EMock_ChatCompletions_OpenAI_MultimodalNonStreamRealUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "chat_completions_multimodal_real.json"))
	fixtureReq := mustReadTestData(t, "fixtures/chat_multimodal_request.json")

	var gotModel string
	var gotContentItems int

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		gotModel = asString(req["model"])
		msgs, _ := req["messages"].([]any)
		if len(msgs) > 0 {
			msg0, _ := msgs[0].(map[string]any)
			content, _ := msg0["content"].([]any)
			gotContentItems = len(content)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIMultimodal(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/chat/completions", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "chat.completions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if gotModel != "gpt-4o-mini" {
		t.Fatalf("unexpected upstream model: %q", gotModel)
	}
	if gotContentItems != 2 {
		t.Fatalf("unexpected multimodal content item count: %d", gotContentItems)
	}
	if res.Usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := asInt(res.Usage["input_tokens"]), 36852; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 1; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 36853; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if res.UsageStage != "upstream" {
		t.Fatalf("usage_stage=%q want=upstream", res.UsageStage)
	}
	if res.FinishReason != "stop" {
		t.Fatalf("finish_reason=%q want=stop", res.FinishReason)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode downstream body: %v", err)
	}
	usage, _ := out["usage"].(map[string]any)
	if asInt(usage["prompt_tokens"]) != 36852 || asInt(usage["completion_tokens"]) != 1 || asInt(usage["total_tokens"]) != 36853 {
		t.Fatalf("unexpected downstream usage: %#v", usage)
	}
}

func TestE2EMock_ChatCompletions_OpenAI_MultimodalStreamRealUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "chat_completions_multimodal_real.sse"))
	fixtureReq := mustReadTestData(t, "fixtures/chat_multimodal_request.json")

	var includeUsage bool

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		streamOptions, _ := req["stream_options"].(map[string]any)
		if v, ok := streamOptions["include_usage"].(bool); ok {
			includeUsage = v
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIMultimodal(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/chat/completions", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "chat.completions", true)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if !includeUsage {
		t.Fatalf("expected stream_options.include_usage=true")
	}
	if res.Usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := asInt(res.Usage["input_tokens"]), 36852; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 1; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 36853; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if res.UsageStage != "upstream" {
		t.Fatalf("usage_stage=%q want=upstream", res.UsageStage)
	}
	if res.FinishReason != "stop" {
		t.Fatalf("finish_reason=%q want=stop", res.FinishReason)
	}
	body := rec.Body.String()
	if !containsAll(body, `"content":"OK"`, `"total_tokens":36853`, "data: [DONE]") {
		t.Fatalf("unexpected stream body:\n%s", body)
	}
}

func TestE2EMock_Responses_OpenAI_MultimodalNonStreamRealUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "responses_multimodal_real.json"))
	fixtureReq := mustReadTestData(t, "fixtures/responses_multimodal_request.json")

	var gotInputItems int

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		input, _ := req["input"].([]any)
		gotInputItems = len(input)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIMultimodal(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/responses", fixtureReq)
	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "responses", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if gotInputItems != 1 {
		t.Fatalf("unexpected input item count: %d", gotInputItems)
	}
	if res.Usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := asInt(res.Usage["input_tokens"]), 36852; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 2; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 36854; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if res.UsageStage != "upstream" {
		t.Fatalf("usage_stage=%q want=upstream", res.UsageStage)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode downstream body: %v", err)
	}
	usage, _ := out["usage"].(map[string]any)
	if asInt(usage["input_tokens"]) != 36852 || asInt(usage["output_tokens"]) != 2 || asInt(usage["total_tokens"]) != 36854 {
		t.Fatalf("unexpected downstream usage: %#v", usage)
	}
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

func providerConfOpenAI(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "audio.transcriptions" {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact audio.stt second path="$.usage.seconds";
    }
    upstream {
      set_path "/v1/audio/transcriptions";
    }
  }
}
`, baseURL)
}

func providerConfOpenAIImages(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "images.generations" {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact image.generate image count_path="$.data[*]";
    }
    upstream {
      set_path "/v1/images/generations";
    }
  }

  match api = "images.edits" {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact image.edit image count_path="$.data[*]";
    }
    upstream {
      set_path "/v1/images/edits";
    }
  }
}
`, baseURL)
}

func providerConfOpenAIAudioSpeech(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "audio.speech" {
    metrics {
      usage_fact audio.tts second source=derived path="$.audio_duration_seconds";
    }
    upstream {
      set_path "/v1/audio/speech";
    }
  }
}
`, baseURL)
}

func providerConfOpenAIMultimodal(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "chat.completions" stream = false {
    metrics {
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token path="$.usage.completion_tokens";
      usage_fact cache_read token path="$.usage.prompt_tokens_details.cached_tokens";
      finish_reason_path "$.choices[*].finish_reason";
    }
    upstream {
      set_path "/v1/chat/completions";
    }
  }

  match api = "chat.completions" stream = true {
    metrics {
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token path="$.usage.completion_tokens";
      usage_fact cache_read token path="$.usage.prompt_tokens_details.cached_tokens";
      finish_reason_path "$.choices[*].finish_reason";
    }
    request {
      json_set "$.stream_options.include_usage" true;
    }
    upstream {
      set_path "/v1/chat/completions";
    }
  }

  match api = "responses" {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_read token path="$.usage.input_tokens_details.cached_tokens";
      finish_reason_path "$.incomplete_details.reason";
      finish_reason_path "$.response.incomplete_details.reason" fallback=true;
    }
    upstream {
      set_path "/v1/responses";
    }
  }
}
`, baseURL)
}

func providerConfOpenAIImageStream(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "images.generations" stream = true {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact image.generate image count_path="$.data[*]";
    }
    upstream {
      set_path "/v1/images/generations";
    }
  }
}
`, baseURL)
}

func newGinJSONRequest(t *testing.T, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	return newGinJSONRequestPath(t, "/v1/chat/completions", body)
}

func newGinJSONRequestPath(t *testing.T, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	gc.Request = req
	return gc, rec
}

func newGinMultipartRequest(t *testing.T, path string, fields map[string]string, fileField string, fileName string, fileBody []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %s: %v", k, err)
		}
	}
	fw, err := w.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(fileBody); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", w.FormDataContentType())
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

func mustReadSharedProxyTestData(t *testing.T, rel string) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "..", "testdata", "real_upstream", rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read shared testdata %s: %v", p, err)
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

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float32:
		return float64(n)
	case float64:
		return n
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
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
