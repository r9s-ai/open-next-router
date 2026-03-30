package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type requestAlignmentGolden struct {
	Path              string              `json:"path"`
	ContentType       string              `json:"content_type"`
	ContentTypePrefix string              `json:"content_type_prefix"`
	JSONRequire       map[string]any      `json:"json_require"`
	JSONAbsent        []string            `json:"json_absent"`
	MultipartValues   map[string][]string `json:"multipart_values"`
	MultipartFileKeys []string            `json:"multipart_file_keys"`
}

func mustLoadProxyRequestAlignmentGolden(t *testing.T, name string) requestAlignmentGolden {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "request_alignment", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read request alignment golden %s: %v", path, err)
	}

	var golden requestAlignmentGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("unmarshal request alignment golden %s: %v", path, err)
	}
	return golden
}

func assertProxyRequestAlignmentJSONGolden(
	t *testing.T,
	golden requestAlignmentGolden,
	path string,
	contentType string,
	payload map[string]any,
) {
	t.Helper()
	if path != golden.Path {
		t.Fatalf("path=%q want=%q", path, golden.Path)
	}
	if golden.ContentType != "" && contentType != golden.ContentType {
		t.Fatalf("content_type=%q want=%q", contentType, golden.ContentType)
	}
	for _, key := range golden.JSONAbsent {
		if _, exists := payload[key]; exists {
			t.Fatalf("json field %s should be absent in %#v", key, payload)
		}
	}
	assertProxyJSONContainsFields(t, payload, golden.JSONRequire)
}

func assertProxyRequestAlignmentMultipartGolden(
	t *testing.T,
	golden requestAlignmentGolden,
	path string,
	contentType string,
	formValues map[string][]string,
	fileKeys []string,
) {
	t.Helper()
	if path != golden.Path {
		t.Fatalf("path=%q want=%q", path, golden.Path)
	}
	if golden.ContentTypePrefix != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), strings.ToLower(golden.ContentTypePrefix)) {
		t.Fatalf("content_type=%q want prefix %q", contentType, golden.ContentTypePrefix)
	}
	for key, want := range golden.MultipartValues {
		got := formValues[key]
		if len(got) != len(want) {
			t.Fatalf("multipart field %s=%v want=%v", key, got, want)
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("multipart field %s=%v want=%v", key, got, want)
			}
		}
	}
	for _, key := range golden.MultipartFileKeys {
		if !containsString(fileKeys, key) {
			t.Fatalf("multipart file keys=%v want contains %q", fileKeys, key)
		}
	}
}

func assertProxyJSONContainsFields(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			t.Fatalf("missing json field %s", key)
		}
		switch wantTyped := wantValue.(type) {
		case map[string]any:
			gotTyped, ok := gotValue.(map[string]any)
			if !ok {
				t.Fatalf("json field %s type mismatch: %#v", key, gotValue)
			}
			assertProxyJSONContainsFields(t, gotTyped, wantTyped)
		default:
			if gotValue != wantValue {
				t.Fatalf("json field %s=%#v want=%#v", key, gotValue, wantValue)
			}
		}
	}
}
