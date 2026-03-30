package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestE2EMock_ImagesEdits_OpenAI_Multipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotModel string
	var gotFileName string
	var gotContentType string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/edits" {
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
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"abc"}],"usage":{"input_tokens":50,"output_tokens":80,"total_tokens":130}}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"openai.conf": providerConfOpenAIImages(mock.URL),
	})
	gc, rec := newGinMultipartRequest(t, "/v1/images/edits", map[string]string{
		"model":  "gpt-image-1",
		"prompt": "retouch",
	}, "image", "sample.png", []byte("fake-image"))

	res, err := c.ProxyJSON(gc, "openai", ProviderKey{Name: "openai-key", Value: "mock-key"}, "images.edits", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if got, want := gotModel, "gpt-image-1"; got != want {
		t.Fatalf("unexpected upstream model: %q want=%q", got, want)
	}
	if got, want := gotFileName, "sample.png"; got != want {
		t.Fatalf("unexpected upstream filename: %q want=%q", got, want)
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(gotContentType)), "multipart/form-data;") {
		t.Fatalf("unexpected upstream content-type: %q", gotContentType)
	}
	if got, want := asInt(res.Usage["image_edit_images"]), 1; got != want {
		t.Fatalf("image_edit_images=%d want=%d", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected downstream status: %d", rec.Code)
	}
}
