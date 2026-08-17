package requesttransform

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
)

func buildEditUpload(t *testing.T, prompt string, imageBytes string, withMask bool) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range map[string]string{"model": "gemini-3-pro-image", "prompt": prompt} {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	writeFile := func(field, content string) {
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+field+`.png"`)
		hdr.Set("Content-Type", "image/png")
		part, err := w.CreatePart(hdr)
		if err != nil {
			t.Fatalf("CreatePart: %v", err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	writeFile("image", imageBytes)
	if withMask {
		writeFile("mask", "MASK")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func editTransform() *dslconfig.RequestTransform {
	return &dslconfig.RequestTransform{
		InlineFiles: []dslconfig.ReqInlineFileRule{
			{Field: "image", MaxBytes: 1 << 20, MaxCount: 8},
			{Field: "mask", MaxBytes: 1 << 20, MaxCount: 1},
		},
		ReqMapMode: "openai_images_edits_to_gemini_generate_content",
	}
}

// End to end through Apply: a multipart upload becomes a Gemini
// generateContent JSON body carrying the image inline.
func TestApply_InlineFilesFeedReqMap(t *testing.T) {
	body, contentType := buildEditUpload(t, "make it blue", "PNGBYTES", true)
	root, err := parseMultipartRoot(body, contentType)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}

	result, err := Apply(&dslmeta.Meta{}, contentType, body, root, editTransform(), ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The upstream body must be JSON, and the header must say so: forwarding
	// the client's multipart content type would describe a payload that is no
	// longer there and the upstream would reject the request.
	if result.ContentType != contentTypeJSON {
		t.Fatalf("ContentType=%q want %q", result.ContentType, contentTypeJSON)
	}
	var got map[string]any
	if err := json.Unmarshal(result.Body, &got); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, result.Body)
	}
	contents, _ := got["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents=%v", got["contents"])
	}
	parts, _ := contents[0].(map[string]any)["parts"].([]any)
	if len(parts) != 4 {
		t.Fatalf("parts=%d want 4 (image + mask + instruction + prompt)", len(parts))
	}
	inline, _ := parts[0].(map[string]any)["inlineData"].(map[string]any)
	if inline["data"] != "UE5HQllURVM=" {
		t.Fatalf("image bytes did not survive: %v", inline)
	}
}

// Without the directive the file parts stay discarded, so req_map cannot see an
// image and rejects the request. This is the behaviour every provider that does
// not opt in keeps, and it is why the upload is never buffered for them.
func TestApply_WithoutInlineFilesReqMapSeesNoImage(t *testing.T) {
	body, contentType := buildEditUpload(t, "make it blue", "PNGBYTES", false)
	root, err := parseMultipartRoot(body, contentType)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}

	transform := editTransform()
	transform.InlineFiles = nil
	if _, err := Apply(&dslmeta.Meta{}, contentType, body, root, transform, ApplyOptions{}); err == nil {
		t.Fatal("expected a rejection when the image was never inlined")
	}
}

// An oversized upload is the client's doing, so it must surface as a validation
// error the proxy turns into a 400, not an opaque transform failure.
func TestApply_OversizedUploadIsAValidationError(t *testing.T) {
	body, contentType := buildEditUpload(t, "p", strings.Repeat("x", 4096), false)
	root, err := parseMultipartRoot(body, contentType)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}

	transform := editTransform()
	transform.InlineFiles[0].MaxBytes = 16

	_, err = Apply(&dslmeta.Meta{}, contentType, body, root, transform, ApplyOptions{})
	var verr *ValidationError
	if !asValidationError(err, &verr) {
		t.Fatalf("err=%T %v want *ValidationError", err, err)
	}
	if !strings.Contains(verr.Message, "image") {
		t.Fatalf("message=%q should name the offending field", verr.Message)
	}
}

// A JSON request reaching a match that declares req_inline_file must pass
// through untouched: the directive describes an upload that is not there.
func TestApply_InlineFilesIgnoreNonMultipartBody(t *testing.T) {
	transform := &dslconfig.RequestTransform{
		InlineFiles: []dslconfig.ReqInlineFileRule{{Field: "image", MaxBytes: 1 << 20, MaxCount: 8}},
	}
	body := []byte(`{"prompt":"p"}`)

	result, err := Apply(&dslmeta.Meta{}, "application/json", body, map[string]any{"prompt": "p"}, transform, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, ok := result.Root["image"]; ok {
		t.Fatal("a JSON request grew an image field")
	}
	if result.ContentType != "application/json" {
		t.Fatalf("ContentType=%q want application/json", result.ContentType)
	}
}

// parseMultipartRoot builds the request root the proxy would hand to Apply for
// a multipart body: text fields only, files discarded.
func parseMultipartRoot(body []byte, contentType string) (map[string]any, error) {
	snapshot, err := requestcanon.Inspect(body, contentType, requestcanon.InspectOptions{AllowNonJSON: true})
	if err != nil {
		return nil, err
	}
	return snapshot.Root, nil
}

func asValidationError(err error, target **ValidationError) bool {
	return errors.As(err, target)
}
