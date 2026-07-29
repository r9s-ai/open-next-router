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
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestvalidate"
)

// providerConfMinimaxImages mirrors the images.generations block shipped in
// config/providers/minimax.conf.
func providerConfMinimaxImages(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "minimax" {
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
    request {
      req_required body "$.prompt";
      req_len body "$.prompt" max=1500;
      req_range body "$.n" min=1 max=9;
      req_enum body "$.response_format" "url" "b64_json" "base64";

      req_map openai_images_to_minimax_image;
    }
    upstream {
      set_path "/v1/image_generation";
    }
    response {
      resp_map minimax_image_to_openai_images;
    }
    metrics {
      usage_fact image.generate image source="response" count_path="$.data[*]";
    }
  }
}
`, baseURL)
}

// TestE2EMock_ImagesGenerations_Minimax covers the full OpenAI images -> Minimax
// /v1/image_generation -> OpenAI images round trip.
func TestE2EMock_ImagesGenerations_Minimax(t *testing.T) {
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
		  "id": "abc",
		  "data": {"image_urls": ["https://example.com/a.png", "https://example.com/b.png"]},
		  "metadata": {"success_count": 2, "failed_count": 0},
		  "base_resp": {"status_code": 0, "status_msg": "success"}
		}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"minimax.conf": providerConfMinimaxImages(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"image-01","prompt":"a red fox","size":"1280x720","n":2}`))

	res, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}

	// 请求侧:OpenAI 形状被重塑成 minimax 的字段。
	if gotPath != "/v1/image_generation" {
		t.Fatalf("upstream path got %q", gotPath)
	}
	if gotUpstreamBody["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio got %v (body=%#v)", gotUpstreamBody["aspect_ratio"], gotUpstreamBody)
	}
	if gotUpstreamBody["response_format"] != "url" {
		t.Fatalf("response_format got %v want url", gotUpstreamBody["response_format"])
	}
	if gotUpstreamBody["n"] != float64(2) {
		t.Fatalf("n got %v want 2", gotUpstreamBody["n"])
	}
	// size 不是 minimax 的参数,不能透传过去。
	if _, ok := gotUpstreamBody["size"]; ok {
		t.Fatalf("size must not reach minimax: %#v", gotUpstreamBody)
	}

	// 响应侧:data 从对象(平行数组)变成 OpenAI 的对象数组。
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal downstream body: %v (%s)", err, rec.Body.String())
	}
	data, _ := out["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data len got %d want 2 (%s)", len(data), rec.Body.String())
	}
	first, _ := data[0].(map[string]any)
	if first["url"] != "https://example.com/a.png" {
		t.Fatalf("url got %v", first["url"])
	}

	// 计费侧:minimax 没有 token 用量,按张数计。
	if got, want := asInt(res.Usage["image_generate_images"]), 2; got != want {
		t.Fatalf("image_generate_images=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
}

// b64_json 需要翻译成 minimax 的 "base64",回程再翻回 b64_json。
func TestE2EMock_ImagesGenerations_Minimax_Base64(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotUpstreamBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotUpstreamBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"image_base64":["AAAA"]},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"minimax.conf": providerConfMinimaxImages(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"image-01","prompt":"x","response_format":"b64_json"}`))

	if _, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false); err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if gotUpstreamBody["response_format"] != "base64" {
		t.Fatalf("upstream response_format got %v want base64", gotUpstreamBody["response_format"])
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	data, _ := out["data"].([]any)
	first, _ := data[0].(map[string]any)
	if first["b64_json"] != "AAAA" {
		t.Fatalf("b64_json got %v (%s)", first["b64_json"], rec.Body.String())
	}
}

// 省略 n 时上游应收到 n=1。Go 适配器把缺失的 n 解码成 0 后直接拒,
// 导致不带 n 的请求全部 400;这里补默认值。
func TestE2EMock_ImagesGenerations_Minimax_OmittedNDefaultsToOne(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotUpstreamBody map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotUpstreamBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"image_urls":["https://example.com/a.png"]},"base_resp":{"status_code":0}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"minimax.conf": providerConfMinimaxImages(mock.URL),
	})
	gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"image-01","prompt":"x"}`))

	if _, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false); err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if gotUpstreamBody["n"] != float64(1) {
		t.Fatalf("upstream n got %v want 1 (body=%#v)", gotUpstreamBody["n"], gotUpstreamBody)
	}
}

// 业务错误藏在 HTTP 200 的 base_resp 里,必须被 resp_map builtin 判为错误,
// 并把 minimax 的 status_msg/status_code 带给客户端。
func TestE2EMock_ImagesGenerations_Minimax_BaseRespError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":2013,"status_msg":"invalid params"}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"minimax.conf": providerConfMinimaxImages(mock.URL),
	})
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"image-01","prompt":"x"}`))

	_, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false)
	var uerr *apitransform.UpstreamResponseError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected *UpstreamResponseError, got %v (body=%s)", err, rec.Body.String())
	}
	if uerr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status got %d want 400", uerr.StatusCode)
	}
	// 上游的 status_msg 必须带给客户端,否则丢掉了唯一的失败原因。
	if uerr.Message != "invalid params" {
		t.Fatalf("message got %q want %q", uerr.Message, "invalid params")
	}
	if uerr.Code != "minimax_2013" {
		t.Fatalf("code got %q want minimax_2013", uerr.Code)
	}
}

// 上游 200 且 base_resp 正常,但一张图都没有:必须报错而不是回空 data。
func TestE2EMock_ImagesGenerations_Minimax_NoImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"image_urls":[]},"base_resp":{"status_code":0,"status_msg":"success"}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"minimax.conf": providerConfMinimaxImages(mock.URL),
	})
	gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"image-01","prompt":"x"}`))

	_, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false)
	var uerr *apitransform.UpstreamResponseError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected *UpstreamResponseError, got %v", err)
	}
	if uerr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status got %d want 500", uerr.StatusCode)
	}
}

// 校验分工:通用边界由 req_* 指令拦下,minimax 特有的 size 由 req_map 拦下,
// 两类都不应打到上游。
func TestE2EMock_ImagesGenerations_Minimax_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamHit := false
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(mock.Close)

	longPrompt := make([]rune, 1501)
	for i := range longPrompt {
		longPrompt[i] = '好'
	}

	cases := []struct {
		name      string
		body      string
		byBuiltin bool
	}{
		{"prompt_missing", `{"model":"image-01"}`, false},
		{"prompt_too_long", fmt.Sprintf(`{"model":"image-01","prompt":%q}`, string(longPrompt)), false},
		{"n_too_large", `{"model":"image-01","prompt":"x","n":10}`, false},
		// 显式 n=0 由 req_range 拒绝(OpenAI images 与 Go 适配器同样要求 n>=1),
		// 到不了 builtin 的缺省逻辑 —— 那条只处理"字段缺失解码成 0"。
		{"n_zero", `{"model":"image-01","prompt":"x","n":0}`, false},
		{"bad_response_format", `{"model":"image-01","prompt":"x","response_format":"webp"}`, false},
		{"bad_size", `{"model":"image-01","prompt":"x","size":"999x999"}`, true},
		{"size_out_of_range", `{"model":"image-01","prompt":"x","size":"256x256"}`, true},
	}
	for _, tc := range cases {
		c := newMockE2EClient(t, map[string]string{
			"minimax.conf": providerConfMinimaxImages(mock.URL),
		})
		gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(tc.body))
		_, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false)
		if err == nil {
			t.Fatalf("%s: expected rejection, got nil error", tc.name)
		}
		if tc.byBuiltin {
			var merr *apitransform.RequestMappingError
			if !errors.As(err, &merr) {
				t.Fatalf("%s: expected *RequestMappingError, got %v", tc.name, err)
			}
			if merr.Param != "size" {
				t.Fatalf("%s: param got %q want size", tc.name, merr.Param)
			}
			continue
		}
		var verr *requestvalidate.RequestValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("%s: expected *RequestValidationError, got %v", tc.name, err)
		}
	}
	if upstreamHit {
		t.Fatal("rejected requests must not reach upstream")
	}
}

// 中文 prompt 恰好 1500 字应当通过:req_len 按 Unicode 码点计数,
// 而 Go 的 len(prompt) 按字节算会误拒(1500 个中文字约 4500 字节)。
func TestE2EMock_ImagesGenerations_Minimax_CJKPromptLength(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"image_urls":["https://example.com/a.png"]},"base_resp":{"status_code":0}}`))
	}))
	t.Cleanup(mock.Close)

	prompt := make([]rune, 1500)
	for i := range prompt {
		prompt[i] = '好'
	}

	c := newMockE2EClient(t, map[string]string{
		"minimax.conf": providerConfMinimaxImages(mock.URL),
	})
	gc, _ := newGinJSONRequestPath(t, "/v1/images/generations",
		[]byte(fmt.Sprintf(`{"model":"image-01","prompt":%q}`, string(prompt))))

	res, err := c.ProxyJSON(gc, "minimax", ProviderKey{Name: "minimax-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("1500 个中文字符应当通过,却被拒: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
}
