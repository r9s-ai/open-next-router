package proxy

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

const derivedUsageTestProviderConf = `
syntax "next-router/0.1";

provider "derived-audio" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    response { resp_passthrough; }
  }

  match api = "audio.translations" {
    metrics {
      usage_extract custom;
      usage_fact audio.translate second source=derived path="$.request_audio_duration_seconds";
      usage_fact output token source=derived path="$.output_text_tokens";
      usage_fact input token source=derived path="$.input_text_tokens";
    }
  }
}
`

// 固定 onr proxy 侧 translations 派生键的生成:请求音频时长、输出文本 token
// 预估(与 relay 侧派生键对齐;此前 onr 运行时仅覆盖 audio_duration_seconds)。
func TestPopulateNonStreamDerivedUsage_TranslationsKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "derived-audio.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(derivedUsageTestProviderConf), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pf, err := dslconfig.ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "audio.mp3")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("not-audio-bytes")); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	_ = writer.Close()

	gin.SetMode(gin.TestMode)
	gc, _ := gin.CreateTestContext(httptest.NewRecorder())
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/translations", body)
	gc.Request.Header.Set("Content-Type", writer.FormDataContentType())

	meta := &dslmeta.Meta{API: "audio.translations"}
	resp := &http.Response{StatusCode: http.StatusOK}
	respBody := []byte(`{"text":"Hello. Welcome to our service."}`)

	populateNonStreamDerivedUsage(gc, meta, pf, "whisper-1", resp, respBody)

	if meta.DerivedUsage == nil {
		t.Fatalf("expected derived usage map")
	}
	// 非法音频字节 → 单文件时长 fallback 1.0 秒。
	if got := meta.DerivedUsage["request_audio_duration_seconds"]; got != 1.0 {
		t.Fatalf("request_audio_duration_seconds got %v want 1.0", got)
	}
	if got, ok := meta.DerivedUsage["output_text_tokens"].(float64); !ok || got <= 0 {
		t.Fatalf("output_text_tokens got %v want >0", meta.DerivedUsage["output_text_tokens"])
	}
	// 请求根无 input 字段:input_text_tokens 不产键,不误报。
	if _, exists := meta.DerivedUsage["input_text_tokens"]; exists {
		t.Fatalf("unexpected input_text_tokens without request input: %#v", meta.DerivedUsage)
	}
}
