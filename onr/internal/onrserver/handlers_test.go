package onrserver

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInspectRequestBody_ImagesEditsMultipart(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("model", "gpt-image-1"); err != nil {
		t.Fatalf("WriteField model: %v", err)
	}
	if err := w.WriteField("prompt", "retouch"); err != nil {
		t.Fatalf("WriteField prompt: %v", err)
	}
	fw, err := w.CreateFormFile("image", "sample.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte("fake-image")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	gc.Request.Header.Set("Content-Type", w.FormDataContentType())

	bodyBytes, stream, model, err := inspectRequestBody(gc, "images.edits")
	if err != nil {
		t.Fatalf("inspectRequestBody error: %v", err)
	}
	if len(bodyBytes) == 0 {
		t.Fatalf("expected body bytes")
	}
	if stream {
		t.Fatalf("expected stream=false")
	}
	if got, want := model, "gpt-image-1"; got != want {
		t.Fatalf("model=%q want=%q", got, want)
	}

	rootAny, ok := gc.Get(ctxKeyRequestRoot)
	if !ok {
		t.Fatalf("expected cached request root")
	}
	root, _ := rootAny.(map[string]any)
	if root == nil {
		t.Fatalf("expected parsed multipart root")
	}
	if got, want := root["model"], "gpt-image-1"; got != want {
		t.Fatalf("root model=%v want=%v", got, want)
	}
	if got, want := root["prompt"], "retouch"; got != want {
		t.Fatalf("root prompt=%v want=%v", got, want)
	}
}
