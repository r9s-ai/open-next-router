package requestcanon

import (
	"bytes"
	"mime/multipart"
	"testing"
)

func TestInspectJSON(t *testing.T) {
	t.Parallel()

	snapshot, err := Inspect([]byte(`{"model":"gpt-4o-mini","stream":true}`), "application/json", InspectOptions{})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got, want := snapshot.Model, "gpt-4o-mini"; got != want {
		t.Fatalf("Model=%q want=%q", got, want)
	}
	if !snapshot.Stream {
		t.Fatalf("expected Stream=true")
	}
	if snapshot.Root == nil {
		t.Fatalf("expected parsed root")
	}
}

func TestInspectMultipart(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("model", "whisper-1"); err != nil {
		t.Fatalf("WriteField(model): %v", err)
	}
	if err := w.WriteField("stream", "true"); err != nil {
		t.Fatalf("WriteField(stream): %v", err)
	}
	fw, err := w.CreateFormFile("file", "a.wav")
	if err != nil {
		t.Fatalf("CreateFormFile(file): %v", err)
	}
	if _, err := fw.Write([]byte("fake-audio")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	snapshot, err := Inspect(body.Bytes(), w.FormDataContentType(), InspectOptions{AllowNonJSON: true})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got, want := snapshot.Model, "whisper-1"; got != want {
		t.Fatalf("Model=%q want=%q", got, want)
	}
	if !snapshot.Stream {
		t.Fatalf("expected Stream=true")
	}
	if got, want := snapshot.Root["stream"], "true"; got != want {
		t.Fatalf("Root[stream]=%v want=%v", got, want)
	}
}

func TestInspectAllowNonJSONRaw(t *testing.T) {
	t.Parallel()

	snapshot, err := Inspect([]byte("raw-binary-ish"), "application/octet-stream", InspectOptions{AllowNonJSON: true})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if snapshot.Root != nil || snapshot.Model != "" || snapshot.Stream {
		t.Fatalf("unexpected snapshot = %#v", snapshot)
	}
}

func TestParseRootMultipartForm(t *testing.T) {
	t.Parallel()

	form := &multipart.Form{
		Value: map[string][]string{
			"model": {"gpt-image-1"},
			"n":     {"2"},
			"tags":  {"a", "b"},
		},
	}

	root := RootFromMultipartForm(form)
	if got, want := root["model"], "gpt-image-1"; got != want {
		t.Fatalf("Root[model]=%v want=%v", got, want)
	}
	if got, ok := root["tags"].([]any); !ok || len(got) != 2 {
		t.Fatalf("Root[tags]=%T %#v", root["tags"], root["tags"])
	}
}
