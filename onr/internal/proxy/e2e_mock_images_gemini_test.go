package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
)

// providerConfGeminiImages mirrors the images.generations block shipped in
// config/providers/gemini.conf. The usage_fact rules are inlined because the
// temp registry used by these tests loads provider files only, not the
// config/modes presets; they match usage_mode "openai_images_generations".
func providerConfGeminiImages(baseURL string) string {
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

  match api = "images.generations" {
    metrics {
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token path="$.usage.completion_tokens";
      usage_fact output.image token path="$.usage.output_tokens_details.image_tokens";
      usage_fact image.generate image source="response" count_path="$.data[*]";
    }
    request {
      req_map openai_images_to_gemini_generate_content;
    }
    response {
      resp_map gemini_to_openai_images;
    }
    upstream {
      set_path template("/v1beta/models/${request.model_mapped}:generateContent");
    }
  }
}
`, baseURL)
}

// TestE2EMock_ImagesGenerations_Gemini covers the full OpenAI images -> Gemini
// generateContent -> OpenAI images round trip: request mapping, response
// mapping, and usage extraction (which runs after resp_map and therefore can
// only see fields the resp_map builtin carried over).
func TestE2EMock_ImagesGenerations_Gemini(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath string
	var gotUpstreamBody map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotUpstreamBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
		  "candidates": [
		    {"content": {"parts": [
		      {"text": "a red fox running through snow"},
		      {"inlineData": {"mimeType": "image/png", "data": "AAAAbase64"}}
		    ]}}
		  ],
		  "usageMetadata": {
		    "promptTokenCount": 12,
		    "candidatesTokenCount": 1290,
		    "thoughtsTokenCount": 64,
		    "totalTokenCount": 1366,
		    "candidatesTokensDetails": [{"modality": "IMAGE", "tokenCount": 1290}]
		  }
		}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"gemini.conf": providerConfGeminiImages(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"gemini-3-pro-image","prompt":"a red fox","size":"1792x1024","quality":"hd"}`))

	res, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}

	// 请求侧:OpenAI images 形状被改写成 Gemini generateContent。
	if want := "/v1beta/models/gemini-3-pro-image:generateContent"; gotPath != want {
		t.Fatalf("upstream path got %q want %q", gotPath, want)
	}
	genConfig, _ := gotUpstreamBody["generationConfig"].(map[string]any)
	imageConfig, _ := genConfig["imageConfig"].(map[string]any)
	if imageConfig["aspectRatio"] != "16:9" {
		t.Fatalf("aspectRatio got %v want 16:9 (body=%#v)", imageConfig["aspectRatio"], gotUpstreamBody)
	}
	if imageConfig["imageSize"] != "4K" {
		t.Fatalf("imageSize got %v want 4K", imageConfig["imageSize"])
	}

	// 响应侧:inlineData 还原成 OpenAI images 形状。
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal downstream body: %v (%s)", err, rec.Body.String())
	}
	data, _ := out["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data len got %d want 1 (%s)", len(data), rec.Body.String())
	}
	item, _ := data[0].(map[string]any)
	if item["b64_json"] != "AAAAbase64" {
		t.Fatalf("b64_json got %v", item["b64_json"])
	}

	// 计费侧:图像 token 维度必须能被 usage_fact 读到。resp_map 若丢掉
	// candidatesTokensDetails,这里就会挂 0 —— 这正是最初的缺口。
	if got, want := asInt(res.Usage["output_image_tokens"]), 1290; got != want {
		t.Fatalf("output_image_tokens=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
	// Go 适配器口径:completion = total - prompt = 1366-12,含 thoughts。
	if got, want := asInt(res.Usage["output_tokens"]), 1354; got != want {
		t.Fatalf("output_tokens=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
	if got, want := asInt(res.Usage["input_tokens"]), 12; got != want {
		t.Fatalf("input_tokens=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
	if got, want := asInt(res.Usage["image_generate_images"]), 1; got != want {
		t.Fatalf("image_generate_images=%d want=%d", got, want)
	}
}

// 上游 200 但没出图时必须失败,而不是回一个 200 空 data 还照常计费。
func TestE2EMock_ImagesGenerations_Gemini_NoImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"promptFeedback":{"blockReason":"SAFETY"},"usageMetadata":{"promptTokenCount":12,"totalTokenCount":12}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"gemini.conf": providerConfGeminiImages(mock.URL),
	})
	gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"gemini-3-pro-image","prompt":"something blocked"}`))

	_, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.generations", false)
	var uerr *apitransform.UpstreamResponseError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected *UpstreamResponseError, got %v", err)
	}
	if uerr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status got %d want 500", uerr.StatusCode)
	}
}

// 请求侧校验命中时不应打到上游。
func TestE2EMock_ImagesGenerations_Gemini_RejectedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamHit := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(mock.Close)

	// 每种拒绝都必须带上与 relay Go 侧一致的 code 和出错参数,
	// 否则客户端只能靠解析文案来区分原因。
	cases := []struct {
		name      string
		body      string
		wantCode  string
		wantParam string
	}{
		{"response_format_url", `{"model":"gemini-3-pro-image","prompt":"x","response_format":"url"}`,
			apitransform.CodeRequestInvalidParameter, "response_format"},
		{"response_format_URL", `{"model":"gemini-3-pro-image","prompt":"x","response_format":"URL"}`,
			apitransform.CodeRequestInvalidParameter, "response_format"},
		{"n_gt_1", `{"model":"gemini-3-pro-image","prompt":"x","n":4}`,
			apitransform.CodeRequestNOutOfRange, "n"},
		{"prompt_missing", `{"model":"gemini-3-pro-image"}`,
			apitransform.CodeRequestPromptMissing, "prompt"},
		{"size_below_gemini3", `{"model":"gemini-2.5-flash-image","prompt":"x","size":"1024x1024"}`,
			apitransform.CodeRequestSizeNotSupported, "size"},
		{"quality_below_gemini3", `{"model":"gemini-2.5-flash-image","prompt":"x","quality":"hd"}`,
			apitransform.CodeRequestInvalidParameter, "quality"},
		{"unsupported_aspect", `{"model":"gemini-3-pro-image","prompt":"x","size":"999x999"}`,
			apitransform.CodeRequestSizeNotSupported, "size"},
		{"bad_quality", `{"model":"gemini-3-pro-image","prompt":"x","quality":"ultra"}`,
			apitransform.CodeRequestInvalidParameter, "quality"},
	}
	for _, tc := range cases {
		c := newMockE2EClient(t, map[string]string{
			"gemini.conf": providerConfGeminiImages(mock.URL),
		})
		gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(tc.body))
		_, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.generations", false)
		var merr *apitransform.RequestMappingError
		if !errors.As(err, &merr) {
			t.Fatalf("%s: expected *RequestMappingError, got %v", tc.name, err)
		}
		if merr.Code != tc.wantCode {
			t.Fatalf("%s: code got %q want %q", tc.name, merr.Code, tc.wantCode)
		}
		if merr.Param != tc.wantParam {
			t.Fatalf("%s: param got %q want %q", tc.name, merr.Param, tc.wantParam)
		}
	}
	if upstreamHit {
		t.Fatal("rejected requests must not reach upstream")
	}
}
