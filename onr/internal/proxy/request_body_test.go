package proxy

import (
	"bytes"
	"mime/multipart"
	"strings"
	"testing"
)

func TestInspectRequestBody_JSON(t *testing.T) {
	t.Parallel()

	info, err := InspectRequestBody([]byte(`{"model":"gpt-4o-mini","stream":true}`), "application/json", false)
	if err != nil {
		t.Fatalf("InspectRequestBody error: %v", err)
	}
	if info.Model != "gpt-4o-mini" {
		t.Fatalf("model=%q", info.Model)
	}
	if !info.Stream {
		t.Fatalf("expected stream=true")
	}
	if info.Root == nil {
		t.Fatalf("expected parsed root")
	}
}

func TestInspectRequestBody_Multipart(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("model", "whisper-1"); err != nil {
		t.Fatalf("WriteField model: %v", err)
	}
	if err := w.WriteField("stream", "true"); err != nil {
		t.Fatalf("WriteField stream: %v", err)
	}
	fw, err := w.CreateFormFile("file", "a.wav")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte("fake-audio")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := InspectRequestBody(body.Bytes(), w.FormDataContentType(), true)
	if err != nil {
		t.Fatalf("InspectRequestBody error: %v", err)
	}
	if info.Model != "whisper-1" {
		t.Fatalf("model=%q", info.Model)
	}
	if !info.Stream {
		t.Fatalf("expected stream=true")
	}
	if info.Root != nil {
		t.Fatalf("expected nil root for multipart")
	}
}

func TestInspectRequestBody_AllowNonJSONRaw(t *testing.T) {
	t.Parallel()

	info, err := InspectRequestBody([]byte("raw-binary-ish"), "application/octet-stream", true)
	if err != nil {
		t.Fatalf("InspectRequestBody error: %v", err)
	}
	if info.Root != nil || info.Model != "" || info.Stream {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestInspectRequestBody_InvalidJSONRejectedWhenRequired(t *testing.T) {
	t.Parallel()

	_, err := InspectRequestBody([]byte("{"), "application/json", false)
	if err == nil || !strings.Contains(err.Error(), "invalid json") {
		t.Fatalf("expected invalid json error, got: %v", err)
	}
}
