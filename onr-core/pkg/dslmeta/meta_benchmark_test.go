package dslmeta

import (
	"encoding/json"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

func BenchmarkMetaRequestRootJSON_NewCached(b *testing.B) {
	body := []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"},{"role":"user","content":"world"}],"input":"cached-root"}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		meta := &Meta{
			RequestBody:        body,
			RequestContentType: "application/json",
		}
		for j := 0; j < 3; j++ {
			root := meta.RequestRoot()
			if root == nil || root["model"] != "gpt-4.1" {
				b.Fatalf("unexpected root %#v", root)
			}
		}
	}
}

func BenchmarkMetaRequestRootJSON_OldReparse(b *testing.B) {
	body := []byte(`{"model":"gpt-4.1","messages":[{"role":"user","content":"hello"},{"role":"user","content":"world"}],"input":"cached-root"}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 3; j++ {
			root := benchmarkLegacyParseRequestRoot(body, "application/json")
			if root == nil || root["model"] != "gpt-4.1" {
				b.Fatalf("unexpected root %#v", root)
			}
		}
	}
}

func BenchmarkMetaRequestRootMultipart_NewCached(b *testing.B) {
	body, contentType := benchmarkMultipartBody()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		meta := &Meta{
			RequestBody:        body,
			RequestContentType: contentType,
		}
		for j := 0; j < 3; j++ {
			root := meta.RequestRoot()
			if root == nil || root["model"] != "gpt-4.1" {
				b.Fatalf("unexpected root %#v", root)
			}
		}
	}
}

func BenchmarkMetaRequestRootMultipart_OldReparse(b *testing.B) {
	body, contentType := benchmarkMultipartBody()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 3; j++ {
			root := benchmarkLegacyParseRequestRoot(body, contentType)
			if root == nil || root["model"] != "gpt-4.1" {
				b.Fatalf("unexpected root %#v", root)
			}
		}
	}
}

func benchmarkLegacyParseRequestRoot(body []byte, contentType string) map[string]any {
	if len(body) == 0 {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data") {
		return benchmarkLegacyParseMultipartRequestRoot(body, contentType)
	}
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}
	root, _ := data.(map[string]any)
	return root
}

func benchmarkLegacyParseMultipartRequestRoot(body []byte, contentType string) map[string]any {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil
	}
	reader := multipart.NewReader(strings.NewReader(string(body)), boundary)
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return nil
	}
	defer form.RemoveAll()

	root := make(map[string]any)
	for k, vals := range form.Value {
		switch len(vals) {
		case 0:
			continue
		case 1:
			root[k] = vals[0]
		default:
			items := make([]any, 0, len(vals))
			for _, v := range vals {
				items = append(items, v)
			}
			root[k] = items
		}
	}
	if len(root) == 0 {
		return nil
	}
	return root
}

func benchmarkMultipartBody() ([]byte, string) {
	var buf strings.Builder
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("model", "gpt-4.1")
	_ = writer.WriteField("input", "hello multipart")
	_ = writer.WriteField("tags", "one")
	_ = writer.WriteField("tags", "two")
	_ = writer.Close()
	return []byte(buf.String()), writer.FormDataContentType()
}
