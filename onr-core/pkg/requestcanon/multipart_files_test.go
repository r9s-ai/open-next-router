package requestcanon

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
)

// buildMultipart writes a multipart body with the given text fields and files.
// files is field -> list of (filename, contentType, content); an empty
// contentType omits the part header so the sniffing path is exercised.
func buildMultipart(t *testing.T, text map[string]string, files [][4]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range text {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	for _, f := range files {
		field, filename, contentType, content := f[0], f[1], f[2], f[3]
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition",
			`form-data; name="`+field+`"; filename="`+filename+`"`)
		if contentType != "" {
			hdr.Set("Content-Type", contentType)
		}
		part, err := w.CreatePart(hdr)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func TestExtractMultipartFiles_ReturnsOnlyRequestedFields(t *testing.T) {
	body, contentType := buildMultipart(t,
		map[string]string{"model": "gemini-3-pro-image", "prompt": "make it blue"},
		[][4]string{
			{"image", "a.png", "image/png", "AAA"},
			{"image", "b.png", "image/png", "BBBB"},
			{"mask", "m.png", "image/png", "MM"},
			{"unwanted", "u.bin", "application/octet-stream", "ignored"},
		})

	files, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 1024, MaxCount: 8},
		{Field: "mask", MaxBytes: 1024, MaxCount: 1},
	})
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if _, ok := files["unwanted"]; ok {
		t.Fatal("a field nobody asked for was buffered")
	}
	if got := len(files["image"]); got != 2 {
		t.Fatalf("image count=%d want 2", got)
	}
	// Order matters: a multimodal model reads its parts in order.
	if string(files["image"][0].Data) != "AAA" || string(files["image"][1].Data) != "BBBB" {
		t.Fatalf("repeated field lost its order: %q", files["image"])
	}
	if files["image"][0].Filename != "a.png" || files["image"][0].ContentType != "image/png" {
		t.Fatalf("part metadata lost: %+v", files["image"][0])
	}
	if got := len(files["mask"]); got != 1 {
		t.Fatalf("mask count=%d want 1", got)
	}
}

// A declared-but-absent field is normal: a config lists mask unconditionally
// and most edit requests do not send one.
func TestExtractMultipartFiles_AbsentFieldIsNotAnError(t *testing.T) {
	body, contentType := buildMultipart(t, map[string]string{"prompt": "p"},
		[][4]string{{"image", "a.png", "image/png", "AAA"}})

	files, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 1024, MaxCount: 8},
		{Field: "mask", MaxBytes: 1024, MaxCount: 1},
	})
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if _, ok := files["mask"]; ok {
		t.Fatal("absent field materialized an entry")
	}
}

// An oversized upload must be rejected, not truncated: a half-read PNG is a
// corrupt image the upstream would reject with a confusing error.
func TestExtractMultipartFiles_OversizedFileIsRejectedNotTruncated(t *testing.T) {
	body, contentType := buildMultipart(t, nil,
		[][4]string{{"image", "a.png", "image/png", strings.Repeat("x", 100)}})

	_, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 10, MaxCount: 8},
	})
	var tooLarge *TooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("err=%v want *TooLargeError", err)
	}
	if tooLarge.MaxBytes != 10 {
		t.Fatalf("MaxBytes=%d want 10", tooLarge.MaxBytes)
	}
}

func TestExtractMultipartFiles_TooManyFilesIsRejected(t *testing.T) {
	body, contentType := buildMultipart(t, nil, [][4]string{
		{"image", "a.png", "image/png", "A"},
		{"image", "b.png", "image/png", "B"},
		{"image", "c.png", "image/png", "C"},
	})

	_, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 1024, MaxCount: 2},
	})
	var tooMany *TooManyError
	if !errors.As(err, &tooMany) {
		t.Fatalf("err=%v want *TooManyError", err)
	}
}

// A client that omits the part's Content-Type still needs a usable MIME type,
// because Gemini's inlineData requires one.
func TestExtractMultipartFiles_SniffsMissingContentType(t *testing.T) {
	png := string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0})
	body, contentType := buildMultipart(t, nil,
		[][4]string{{"image", "a.png", "", png}})

	files, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 1024, MaxCount: 8},
	})
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if got := files["image"][0].ContentType; got != "image/png" {
		t.Fatalf("sniffed content type=%q want image/png", got)
	}
}

// Nothing should be read when no rule asks for it — this is what keeps ordinary
// passthrough of an upload from buffering the whole file.
func TestExtractMultipartFiles_NoRequestsReadsNothing(t *testing.T) {
	body, contentType := buildMultipart(t, nil,
		[][4]string{{"image", "a.png", "image/png", "AAA"}})

	files, err := ExtractMultipartFiles(body, contentType, nil)
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files=%v want none", files)
	}
}

func TestExtractMultipartFiles_NonMultipartBodyIsIgnored(t *testing.T) {
	files, err := ExtractMultipartFiles([]byte(`{"prompt":"p"}`), "application/json",
		[]FileFieldRequest{{Field: "image", MaxBytes: 1024, MaxCount: 8}})
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files=%v want none", files)
	}
}

// Many clients (including Go's own CreateFormFile) label every upload
// application/octet-stream. Gemini's inlineData needs a real image type, so the
// filename extension has to win over that generic label — this is what the
// relay Go adaptor's GetFileContentType does.
func TestExtractMultipartFiles_OctetStreamFallsBackToExtension(t *testing.T) {
	body, contentType := buildMultipart(t, nil,
		[][4]string{{"image", "fox.png", "application/octet-stream", "not really a png"}})

	files, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 1024, MaxCount: 8},
	})
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if got := files["image"][0].ContentType; got != "image/png" {
		t.Fatalf("content type=%q want image/png from the extension", got)
	}
}

// A declared type that is not octet-stream is authoritative and must not be
// second-guessed by the extension.
func TestExtractMultipartFiles_DeclaredTypeWinsOverExtension(t *testing.T) {
	body, contentType := buildMultipart(t, nil,
		[][4]string{{"image", "fox.png", "image/webp", "AAA"}})

	files, err := ExtractMultipartFiles(body, contentType, []FileFieldRequest{
		{Field: "image", MaxBytes: 1024, MaxCount: 8},
	})
	if err != nil {
		t.Fatalf("ExtractMultipartFiles: %v", err)
	}
	if got := files["image"][0].ContentType; got != "image/webp" {
		t.Fatalf("content type=%q want the declared image/webp", got)
	}
}
