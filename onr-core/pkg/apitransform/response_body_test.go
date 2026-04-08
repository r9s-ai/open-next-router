package apitransform

import (
	"bytes"
	"compress/gzip"
	"testing"
)

func TestDecodeResponseBody_Gzip(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	got, changed, err := DecodeResponseBody(buf.Bytes(), "gzip")
	if err != nil {
		t.Fatalf("DecodeResponseBody: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("got=%s", string(got))
	}
}

func TestApplyResponseJSONOpsBody(t *testing.T) {
	got, changed, err := ApplyResponseJSONOpsBody([]byte(`{"a":1}`), "application/json", func(obj map[string]any) (any, error) {
		obj["b"] = 2
		return obj, nil
	})
	if err != nil {
		t.Fatalf("ApplyResponseJSONOpsBody: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if !containsAll(string(got), `"a":1`, `"b":2`) {
		t.Fatalf("unexpected output: %s", string(got))
	}
}
