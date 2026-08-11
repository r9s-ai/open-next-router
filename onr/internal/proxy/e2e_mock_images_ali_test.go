package proxy

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestvalidate"
)

// providerConfAliImages mirrors the images.generations block shipped in
// config/providers/ali.conf.
func providerConfAliImages(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

usage_mode "image_generations_by_picture" {
  usage_fact image.generate image source="response" count_path="$.data[*]";
}

provider "ali" {
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
      req_len body "$.prompt" max=800;
      req_len body "$.negative_prompt" max=500;
      req_range body "$.n" min=1 max=1;
      req_range body "$.seed" min=0 max=2147483647;

      req_map openai_images_to_qwen_image;
    }
    upstream {
      set_path "/api/v1/services/aigc/multimodal-generation/generation";
    }
    response {
      resp_map qwen_image_to_openai_images;
      resp_inline_url path="$.data[*].url" set="b64_json"
                      when_request="$.response_format" when_eq="b64_json";
    }
    metrics {
      usage_extract image_generations_by_picture;
    }
  }
}
`, baseURL)
}

func newAliClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	c := newMockE2EClient(t, map[string]string{"ali.conf": providerConfAliImages(baseURL)})
	// The asset fetch shares this client, so allow more than the harness default.
	c.HTTP = &http.Client{Timeout: 10 * time.Second}
	return c
}

// qwenResponse is the shape DashScope documents for a successful generation.
func qwenResponse(imageURL string) string {
	return fmt.Sprintf(`{
	  "output": {"choices": [{"finish_reason": "stop",
	    "message": {"role": "assistant", "content": [{"image": %q}]}}]},
	  "usage": {"width": 1328, "height": 1328, "image_count": 1},
	  "request_id": "req-1"
	}`, imageURL)
}

func TestE2EMock_ImagesGenerations_Ali(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath string
	var gotUpstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotUpstreamBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qwenResponse("https://example.invalid/a.png")))
	}))
	t.Cleanup(upstream.Close)

	c := newAliClient(t, upstream.URL)
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"qwen-image","prompt":"a red fox","size":"1024x1024","response_format":"url"}`))

	res, err := c.ProxyJSON(gc, "ali", ProviderKey{Name: "ali-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}

	// 请求侧:OpenAI 的扁平参数被包成 DashScope 的对话结构。
	if gotPath != "/api/v1/services/aigc/multimodal-generation/generation" {
		t.Fatalf("upstream path got %q", gotPath)
	}
	input, _ := gotUpstreamBody["input"].(map[string]any)
	messages, _ := input["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("input.messages got %#v (body=%#v)", input["messages"], gotUpstreamBody)
	}
	params, _ := gotUpstreamBody["parameters"].(map[string]any)
	if params["size"] != "1024*1024" {
		t.Fatalf("size got %v want 1024*1024", params["size"])
	}
	// OpenAI 字段名不能残留在顶层。
	for _, k := range []string{"prompt", "size", "n", "response_format"} {
		if _, exists := gotUpstreamBody[k]; exists {
			t.Fatalf("OpenAI field %q leaked to DashScope: %#v", k, gotUpstreamBody)
		}
	}

	// 响应侧:嵌套三层的 image 被摊平成 OpenAI 的 data[].url。
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal downstream: %v (%s)", err, rec.Body.String())
	}
	item := firstDataItem(t, out)
	if item["url"] != "https://example.invalid/a.png" {
		t.Fatalf("url got %v", item["url"])
	}
	for _, leaked := range []string{"choices", "request_id", "finish_reason"} {
		if _, exists := out[leaked]; exists {
			t.Fatalf("DashScope shape %q leaked: %s", leaked, rec.Body.String())
		}
	}

	if got, want := asInt(res.Usage["image_generate_images"]), 1; got != want {
		t.Fatalf("image_generate_images=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
}

// DashScope never returns inline content, so b64_json is served by fetching the
// link it did return.
func TestE2EMock_ImagesGenerations_Ali_B64JSONFetchesTheImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const asset = "QWEN-IMAGE-BYTES"
	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(asset))
	}))
	t.Cleanup(assets.Close)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qwenResponse(assets.URL + "/a.png")))
	}))
	t.Cleanup(upstream.Close)

	c := newAliClient(t, upstream.URL)
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"qwen-image","prompt":"a red fox","response_format":"b64_json"}`))

	if _, err := c.ProxyJSON(gc, "ali", ProviderKey{Name: "ali-key", Value: "mock-key"}, "images.generations", false); err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	item := firstDataItem(t, out)
	if item["b64_json"] != base64.StdEncoding.EncodeToString([]byte(asset)) {
		t.Fatalf("b64_json got %v (%s)", item["b64_json"], rec.Body.String())
	}
	if _, ok := item["url"]; ok {
		t.Fatalf("url should be replaced once inlined: %s", rec.Body.String())
	}
}

// 业务错误藏在 HTTP 200 的顶层 code 里。
func TestE2EMock_ImagesGenerations_Ali_BusinessErrorInsideHTTP200(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"InvalidParameter","message":"size is invalid","request_id":"req-2"}`))
	}))
	t.Cleanup(upstream.Close)

	c := newAliClient(t, upstream.URL)
	gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"qwen-image","prompt":"x"}`))

	_, err := c.ProxyJSON(gc, "ali", ProviderKey{Name: "ali-key", Value: "mock-key"}, "images.generations", false)
	var uerr *apitransform.UpstreamResponseError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected *UpstreamResponseError, got %v", err)
	}
	if uerr.Message != "size is invalid" {
		t.Fatalf("upstream message lost: %q", uerr.Message)
	}
}

// 通用边界由 req_* 指令拦下,都不应打到上游。
func TestE2EMock_ImagesGenerations_Ali_Rejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamHit := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(upstream.Close)

	long := make([]rune, 801)
	for i := range long {
		long[i] = 'x'
	}
	cases := map[string]string{
		"prompt_missing":  `{"model":"qwen-image"}`,
		"prompt_too_long": fmt.Sprintf(`{"model":"qwen-image","prompt":%q}`, string(long)),
		"n_too_large":     `{"model":"qwen-image","prompt":"x","n":2}`,
		"seed_negative":   `{"model":"qwen-image","prompt":"x","seed":-1}`,
	}
	for name, body := range cases {
		c := newAliClient(t, upstream.URL)
		gc, _ := newGinJSONRequestPath(t, "/v1/images/generations", []byte(body))
		_, err := c.ProxyJSON(gc, "ali", ProviderKey{Name: "ali-key", Value: "mock-key"}, "images.generations", false)
		var verr *requestvalidate.RequestValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("%s: expected *RequestValidationError, got %v", name, err)
		}
	}
	if upstreamHit {
		t.Fatal("rejected requests must not reach upstream")
	}
}

// 800 个中文字符必须通过:req_len 按 Unicode 码点计数,而 Go 的 len(prompt)
// 按字节算(约 2400 字节)会误拒。
func TestE2EMock_ImagesGenerations_Ali_CJKPromptWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qwenResponse("https://example.invalid/a.png")))
	}))
	t.Cleanup(upstream.Close)

	prompt := make([]rune, 800)
	for i := range prompt {
		prompt[i] = '好'
	}
	c := newAliClient(t, upstream.URL)
	gc, _ := newGinJSONRequestPath(t, "/v1/images/generations",
		[]byte(fmt.Sprintf(`{"model":"qwen-image","prompt":%q}`, string(prompt))))

	if _, err := c.ProxyJSON(gc, "ali", ProviderKey{Name: "ali-key", Value: "mock-key"}, "images.generations", false); err != nil {
		t.Fatalf("800 个中文字符应当通过,却被拒: %v", err)
	}
}

// 用量按条目数计价,不关心字节。内联发生在用量快照之后,否则每张被抓取的图片
// 都会以 base64 形式进入用量提取和流量 dump —— 单张上限 10MB。
func TestE2EMock_ImagesGenerations_Ali_UsageCountUnaffectedByInlining(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const asset = "QWEN-BYTES"
	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(asset))
	}))
	t.Cleanup(assets.Close)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(qwenResponse(assets.URL + "/a.png")))
	}))
	t.Cleanup(upstream.Close)

	c := newAliClient(t, upstream.URL)
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(
		`{"model":"qwen-image","prompt":"x","response_format":"b64_json"}`))

	res, err := c.ProxyJSON(gc, "ali", ProviderKey{Name: "ali-key", Value: "mock-key"}, "images.generations", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	// 客户端拿到内联内容,而张数仍按条目计。
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid downstream body: %s", rec.Body.String())
	}
	item := firstDataItem(t, mustUnmarshalObject(t, rec.Body.Bytes()))
	if item["b64_json"] != base64.StdEncoding.EncodeToString([]byte(asset)) {
		t.Fatalf("client should receive inlined content: %s", rec.Body.String())
	}
	if got, want := asInt(res.Usage["image_generate_images"]), 1; got != want {
		t.Fatalf("image_generate_images=%d want=%d (usage=%#v)", got, want, res.Usage)
	}
}

func mustUnmarshalObject(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, body)
	}
	return out
}
