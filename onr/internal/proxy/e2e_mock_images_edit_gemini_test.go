package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
)

// providerConfGeminiImageEdits mirrors the images.edits block shipped in
// config/providers/gemini.conf. The usage_fact rules are inlined because the
// temp registry used by these tests loads provider files only, not the
// config/modes presets; they match usage_mode "openai_images_generations".
func providerConfGeminiImageEdits(baseURL string) string {
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

  match api = "images.edits" {
    metrics {
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token path="$.usage.completion_tokens";
      usage_fact output.image token path="$.usage.output_tokens_details.image_tokens";
      usage_fact image.generate image source="response" count_path="$.data[*]";
    }
    request {
      req_inline_file field="image" max_bytes=20971520 max_count=8;
      req_inline_file field="mask" max_bytes=20971520 max_count=1;

      req_map openai_images_edits_to_gemini_generate_content;
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

const geminiEditUpstreamResponse = `{
  "candidates": [
    {"content": {"parts": [
      {"text": "a fox with a blue sky behind it"},
      {"inlineData": {"mimeType": "image/png", "data": "RURJVEVE"}}
    ]}}
  ],
  "usageMetadata": {
    "promptTokenCount": 260,
    "candidatesTokenCount": 1290,
    "totalTokenCount": 1550,
    "candidatesTokensDetails": [{"modality": "IMAGE", "tokenCount": 1290}]
  }
}`

// newGinImageEditRequest builds the multipart body an OpenAI images.edits
// client sends: text fields plus one or more image files and an optional mask.
func newGinImageEditRequest(t *testing.T, fields map[string]string, files [][3]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField %s: %v", k, err)
		}
	}
	for _, f := range files {
		field, filename, content := f[0], f[1], f[2]
		fw, err := w.CreateFormFile(field, filename)
		if err != nil {
			t.Fatalf("CreateFormFile %s: %v", field, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", field, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	gc.Request = req
	return gc, rec
}

// TestE2EMock_ImagesEdits_Gemini covers the full OpenAI images.edits ->
// Gemini generateContent -> OpenAI images round trip. The uploaded file only
// reaches the upstream because req_inline_file put it in the request root:
// canonicalization discards file parts by default.
func TestE2EMock_ImagesEdits_Gemini(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath, gotContentType string
	var gotUpstreamBody map[string]any

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotUpstreamBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiEditUpstreamResponse))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"gemini.conf": providerConfGeminiImageEdits(mock.URL),
	})
	gc, rec := newGinImageEditRequest(t,
		map[string]string{
			"model":   "gemini-3-pro-image",
			"prompt":  "make the sky blue",
			"size":    "16:9",
			"quality": "hd",
		},
		[][3]string{
			{"image", "fox.png", "ORIGINAL"},
			{"mask", "mask.png", "MASKBYTES"},
		})

	res, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.edits", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}

	if want := "/v1beta/models/gemini-3-pro-image:generateContent"; gotPath != want {
		t.Fatalf("upstream path got %q want %q", gotPath, want)
	}
	// req_map produced a JSON body, so the header must say JSON. Forwarding the
	// client's multipart content type would describe a payload (boundary and
	// all) that is no longer there, and Gemini rejects the request.
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Fatalf("upstream Content-Type got %q want application/json", gotContentType)
	}

	// 请求侧:上传的文件必须变成 inlineData,顺序是 原图 -> mask -> mask 说明 -> prompt。
	contents, _ := gotUpstreamBody["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents got %v want one entry", gotUpstreamBody["contents"])
	}
	content, _ := contents[0].(map[string]any)
	if content["role"] != "user" {
		t.Fatalf("role got %v want user", content["role"])
	}
	parts, _ := content["parts"].([]any)
	if len(parts) != 4 {
		t.Fatalf("parts got %d want 4 (image, mask, mask instruction, prompt): %#v", len(parts), parts)
	}

	image, _ := parts[0].(map[string]any)["inlineData"].(map[string]any)
	if image["mimeType"] != "image/png" {
		t.Fatalf("image mimeType got %v want image/png", image["mimeType"])
	}
	if want := base64.StdEncoding.EncodeToString([]byte("ORIGINAL")); image["data"] != want {
		t.Fatalf("image data got %v want %q", image["data"], want)
	}
	mask, _ := parts[1].(map[string]any)["inlineData"].(map[string]any)
	if want := base64.StdEncoding.EncodeToString([]byte("MASKBYTES")); mask["data"] != want {
		t.Fatalf("mask data got %v want %q", mask["data"], want)
	}
	if text, _ := parts[2].(map[string]any)["text"].(string); !strings.Contains(text, "mask") {
		t.Fatalf("parts[2] got %v want the mask instruction", parts[2])
	}
	if text, _ := parts[3].(map[string]any)["text"].(string); text != "make the sky blue" {
		t.Fatalf("parts[3] got %v want the prompt last", parts[3])
	}

	genConfig, _ := gotUpstreamBody["generationConfig"].(map[string]any)
	imageConfig, _ := genConfig["imageConfig"].(map[string]any)
	if imageConfig["aspectRatio"] != "16:9" || imageConfig["imageSize"] != "4K" {
		t.Fatalf("imageConfig got %#v want 16:9/4K", imageConfig)
	}

	// 响应侧:编辑的响应就是普通 generateContent 响应,复用 gemini_to_openai_images。
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal downstream body: %v (%s)", err, rec.Body.String())
	}
	data, _ := out["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data len got %d want 1 (%s)", len(data), rec.Body.String())
	}
	if item, _ := data[0].(map[string]any); item["b64_json"] != "RURJVEVE" {
		t.Fatalf("b64_json got %v", item["b64_json"])
	}

	// 计费侧:编辑和生成共用同一份用量口径。
	if got, want := asInt(res.Usage["output_image_tokens"]), 1290; got != want {
		t.Fatalf("output_image_tokens=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
	if got, want := asInt(res.Usage["input_tokens"]), 260; got != want {
		t.Fatalf("input_tokens=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
	// Go 适配器口径:completion = total - prompt。
	if got, want := asInt(res.Usage["output_tokens"]), 1290; got != want {
		t.Fatalf("output_tokens=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
	if got, want := asInt(res.Usage["image_generate_images"]), 1; got != want {
		t.Fatalf("image_generate_images=%d want=%d", got, want)
	}
}

// 只有一张原图、没有 mask 时,不应凭空多出一段 mask 说明 —— 那句话描述的是
// 一张根本没发送的图,会把模型推向没人要求的编辑。
func TestE2EMock_ImagesEdits_Gemini_NoMask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotUpstreamBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotUpstreamBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiEditUpstreamResponse))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"gemini.conf": providerConfGeminiImageEdits(mock.URL),
	})
	gc, _ := newGinImageEditRequest(t,
		map[string]string{"model": "gemini-3-pro-image", "prompt": "make it blue"},
		[][3]string{{"image", "fox.png", "ORIGINAL"}})

	if _, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.edits", false); err != nil {
		t.Fatalf("proxy error: %v", err)
	}

	contents, _ := gotUpstreamBody["contents"].([]any)
	content, _ := contents[0].(map[string]any)
	parts, _ := content["parts"].([]any)
	if len(parts) != 2 {
		t.Fatalf("parts got %d want 2 (image + prompt): %#v", len(parts), parts)
	}
	if text, _ := parts[1].(map[string]any)["text"].(string); text != "make it blue" {
		t.Fatalf("parts[1] got %v want the prompt", parts[1])
	}
}

// 没带图的编辑请求必须在本地就被拒,不能打到上游。
func TestE2EMock_ImagesEdits_Gemini_RejectsMissingImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamHit := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"gemini.conf": providerConfGeminiImageEdits(mock.URL),
	})
	gc, _ := newGinImageEditRequest(t,
		map[string]string{"model": "gemini-3-pro-image", "prompt": "make it blue"},
		nil)

	_, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.edits", false)
	var mapErr *apitransform.RequestMappingError
	if !errors.As(err, &mapErr) {
		t.Fatalf("expected *RequestMappingError, got %v", err)
	}
	if mapErr.Code != apitransform.CodeRequestMissingRequiredField || mapErr.Param != "image" {
		t.Fatalf("code=%q param=%q want %q/image", mapErr.Code, mapErr.Param, apitransform.CodeRequestMissingRequiredField)
	}
	if upstreamHit {
		t.Fatal("a request with no image reached the upstream")
	}
}

// 超过 max_bytes 的上传必须被拒,而不是截断后发给上游 —— 半张 PNG 只会换回
// 一个让人摸不着头脑的上游错误。
func TestE2EMock_ImagesEdits_Gemini_RejectsOversizedUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamHit := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(mock.Close)

	conf := strings.Replace(
		providerConfGeminiImageEdits(mock.URL),
		`req_inline_file field="image" max_bytes=20971520 max_count=8;`,
		`req_inline_file field="image" max_bytes=16 max_count=8;`,
		1)
	c := newMockE2EClient(t, map[string]string{"gemini.conf": conf})
	gc, _ := newGinImageEditRequest(t,
		map[string]string{"model": "gemini-3-pro-image", "prompt": "make it blue"},
		[][3]string{{"image", "fox.png", strings.Repeat("x", 4096)}})

	_, err := c.ProxyJSON(gc, "gemini", ProviderKey{Name: "gemini-key", Value: "mock-key"}, "images.edits", false)
	if err == nil {
		t.Fatal("expected an oversized upload to be rejected")
	}
	if !strings.Contains(err.Error(), "image") {
		t.Fatalf("err=%q should name the offending field", err)
	}
	if upstreamHit {
		t.Fatal("an oversized upload reached the upstream")
	}
}
