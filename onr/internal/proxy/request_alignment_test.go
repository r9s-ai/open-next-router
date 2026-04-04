package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestE2EMock_RequestAlignment_ImagesGenerations_JSONOpsPreserveUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "images.generations")

	var gotPath string
	var gotContentType string
	var gotPayload map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"created":123,"data":[{"b64_json":"img"}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(`{
		"model":"gpt-image-1",
		"prompt":"draw a cat",
		"service_tier":"auto",
		"custom_meta":{"trace_id":"img-e2e"}
	}`))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentJSONGolden(t, golden, gotPath, gotContentType, gotPayload)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_RequestAlignment_AudioSpeech_JSONOpsPreserveUnknownFieldsAndDerivedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "audio.speech")

	mockResp := mustReadSharedProxyTestData(t, filepath.Join("openai", "audio_speech_gpt_4o_mini_tts_real.mp3"))
	var gotPath string
	var gotContentType string
	var gotPayload map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/speech" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockResp)
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/audio/speech", []byte(`{
		"model":"gpt-4o-mini-tts",
		"voice":"alloy",
		"input":"hello world",
		"service_tier":"auto",
		"custom_meta":{"trace_id":"tts-e2e"}
	}`))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.speech", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentJSONGolden(t, golden, gotPath, gotContentType, gotPayload)
	if got := asFloat(res.Usage["audio_tts_seconds"]); got <= 0 {
		t.Fatalf("audio_tts_seconds=%v want>0", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
	if rec.Body.Len() != len(mockResp) {
		t.Fatalf("downstream body len=%d want=%d", rec.Body.Len(), len(mockResp))
	}
}

func TestE2EMock_RequestAlignment_ImagesEdits_MultipartPreservesUnknownFieldsAndRequestFallbackUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "images.edits")

	var gotPath string
	var gotContentType string
	var gotFormValues map[string][]string
	var gotFileKeys []string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/edits" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		cloneReq := r.Clone(r.Context())
		cloneReq.Body = io.NopCloser(bytes.NewReader(body))
		if err := cloneReq.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}

		gotFormValues = cloneMultipartValues(cloneReq.MultipartForm.Value)
		gotFileKeys = collectMultipartFileKeys(cloneReq.MultipartForm)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"abc"}],"usage":{"input_tokens":0,"output_tokens":9,"total_tokens":9}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinMultipartRequest(t, "/v1/images/edits", map[string]string{
		"model":         "gpt-image-1.5",
		"prompt":        "draw a cat",
		"n":             "2",
		"provider_flag": "alpha",
	}, "image", "sample.png", []byte("fake-image"))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "images.edits", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentMultipartGolden(t, golden, gotPath, gotContentType, gotFormValues, gotFileKeys)
	if got, want := asInt(res.Usage["image_edit_images"]), 2; got != want {
		t.Fatalf("image_edit_images=%d want=%d", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_RequestAlignment_Responses_JSONOpsPreserveUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "responses")

	var gotPath string
	var gotContentType string
	var gotPayload map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_test_1","status":"completed","model":"gpt-4.1","output":[{"type":"output_text","text":"hello"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/responses", []byte(`{
		"model":"gpt-4.1",
		"input":"hello",
		"service_tier":"auto",
		"metadata":{"trace_id":"resp-e2e"}
	}`))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "responses", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentJSONGolden(t, golden, gotPath, gotContentType, gotPayload)
	if got, want := asInt(res.Usage["input_tokens"]), 3; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 4; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 7; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_RequestAlignment_Embeddings_JSONOpsPreserveUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "embeddings")

	var gotPath string
	var gotContentType string
	var gotPayload map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":5,"total_tokens":5}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/embeddings", []byte(`{
		"model":"text-embedding-3-small",
		"input":"hello",
		"service_tier":"auto",
		"custom_meta":{"trace_id":"embed-e2e"}
	}`))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "embeddings", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentJSONGolden(t, golden, gotPath, gotContentType, gotPayload)
	if got, want := asInt(res.Usage["input_tokens"]), 5; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 5; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_RequestAlignment_ChatCompletions_JSONOpsPreserveUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "chat.completions")

	var gotPath string
	var gotContentType string
	var gotPayload map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":123,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":6,"completion_tokens":2,"total_tokens":8}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/chat/completions", []byte(`{
		"model":"gpt-4o",
		"messages":[{"role":"user","content":"hello"}],
		"service_tier":"auto",
		"custom_meta":{"trace_id":"chat-e2e"}
	}`))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "chat.completions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentJSONGolden(t, golden, gotPath, gotContentType, gotPayload)
	if got, want := asInt(res.Usage["input_tokens"]), 6; got != want {
		t.Fatalf("input_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["output_tokens"]), 2; got != want {
		t.Fatalf("output_tokens=%d want=%d", got, want)
	}
	if got, want := asInt(res.Usage["total_tokens"]), 8; got != want {
		t.Fatalf("total_tokens=%d want=%d", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_RequestAlignment_AudioTranscriptions_MultipartPreservesUnknownFieldsAndUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "audio.transcriptions")

	var gotPath string
	var gotContentType string
	var gotFormValues map[string][]string
	var gotFileKeys []string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		cloneReq := r.Clone(r.Context())
		cloneReq.Body = io.NopCloser(bytes.NewReader(body))
		if err := cloneReq.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		gotFormValues = cloneMultipartValues(cloneReq.MultipartForm.Value)
		gotFileKeys = collectMultipartFileKeys(cloneReq.MultipartForm)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"hello","usage":{"seconds":1.5,"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinMultipartRequest(t, "/v1/audio/transcriptions", map[string]string{
		"model":         "gpt-4o-mini-transcribe",
		"provider_flag": "alpha",
	}, "file", "speech.mp3", []byte("fake-audio"))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.transcriptions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentMultipartGolden(t, golden, gotPath, gotContentType, gotFormValues, gotFileKeys)
	if got, want := asFloat(res.Usage["audio_stt_seconds"]), 1.5; got != want {
		t.Fatalf("audio_stt_seconds=%v want=%v", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func TestE2EMock_RequestAlignment_AudioTranslations_MultipartPreservesUnknownFieldsAndUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	golden := mustLoadProxyRequestAlignmentGolden(t, "audio.translations")

	var gotPath string
	var gotContentType string
	var gotFormValues map[string][]string
	var gotFileKeys []string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/translations" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		cloneReq := r.Clone(r.Context())
		cloneReq.Body = io.NopCloser(bytes.NewReader(body))
		if err := cloneReq.ParseMultipartForm(32 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		gotFormValues = cloneMultipartValues(cloneReq.MultipartForm.Value)
		gotFileKeys = collectMultipartFileKeys(cloneReq.MultipartForm)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"hello","usage":{"seconds":2.5,"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIRequestAlignment(mock.URL),
	})
	gc, rec := newGinMultipartRequest(t, "/v1/audio/translations", map[string]string{
		"model":         "whisper-1",
		"provider_flag": "beta",
	}, "file", "speech.mp3", []byte("fake-audio"))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "audio.translations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	assertProxyRequestAlignmentMultipartGolden(t, golden, gotPath, gotContentType, gotFormValues, gotFileKeys)
	if got, want := asFloat(res.Usage["audio_translate_seconds"]), 2.5; got != want {
		t.Fatalf("audio_translate_seconds=%v want=%v", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}

func providerConfOpenAIRequestAlignment(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    request {
      json_del "$.service_tier";
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
      usage_fact image.edit image source=request expr="$.n" fallback=true;
    }
    upstream {
      set_path "/v1/images/edits";
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

  match api = "responses" {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_read token path="$.usage.input_tokens_details.cached_tokens";
      usage_fact server_tool.web_search call count_path="$.output[*]" type="web_search_call" status="completed";
      usage_fact server_tool.web_search call count_path="$.response.output[*]" type="web_search_call" status="completed" fallback=true;
    }
    upstream {
      set_path "/v1/responses";
    }
  }

  match api = "embeddings" {
    metrics {
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token expr="0";
    }
    upstream {
      set_path "/v1/embeddings";
    }
  }

  match api = "chat.completions" stream = false {
    metrics {
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact input token path="$.usage.input_tokens" fallback=true;
      usage_fact output token path="$.usage.completion_tokens";
      usage_fact output token path="$.usage.output_tokens" fallback=true;
      usage_fact cache_read token path="$.usage.prompt_tokens_details.cached_tokens";
      usage_fact cache_read token path="$.usage.input_tokens_details.cached_tokens" fallback=true;
      usage_fact cache_read token path="$.usage.cached_tokens" fallback=true;
    }
    upstream {
      set_path "/v1/chat/completions";
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

  match api = "audio.translations" {
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact audio.translate second path="$.usage.seconds";
    }
    upstream {
      set_path "/v1/audio/translations";
    }
  }
}
`, baseURL)
}

func cloneMultipartValues(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func collectMultipartFileKeys(form *multipart.Form) []string {
	if form == nil || len(form.File) == 0 {
		return nil
	}
	keys := make([]string, 0, len(form.File))
	for key := range form.File {
		keys = append(keys, key)
	}
	return keys
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
