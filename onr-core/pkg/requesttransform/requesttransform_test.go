package requesttransform

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestApply_JSONOpsThenReqMap(t *testing.T) {
	t.Parallel()

	meta := &dslmeta.Meta{DSLModelMapped: "gpt-4.1"}
	body := []byte(`{"model":"gpt-4o-mini","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"hello"}],"metadata":{"trace_id":"req-transform"}}`)
	value := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []any{
			map[string]any{"role": "system", "content": "You are helpful."},
			map[string]any{"role": "user", "content": "hello"},
		},
		"metadata": map[string]any{"trace_id": "req-transform"},
	}

	result, err := Apply(meta, "application/json", body, value, dslconfig.RequestTransform{
		JSONOps: []dslconfig.JSONOp{
			{Op: "json_set", Path: "$.metadata.origin", ValueExpr: `"dsl"`},
		},
		ReqMapMode: "openai_chat_to_openai_responses",
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	root, ok := result.Value.(map[string]any)
	if !ok {
		t.Fatalf("Value type = %T", result.Value)
	}
	if _, exists := root["messages"]; exists {
		t.Fatalf("messages should be removed after req_map, got=%v", root)
	}
	if got, want := root["model"], "gpt-4.1"; got != want {
		t.Fatalf("model=%v want=%v", got, want)
	}
	metaMap, _ := root["metadata"].(map[string]any)
	if got, want := metaMap["origin"], "dsl"; got != want {
		t.Fatalf("metadata.origin=%v want=%v", got, want)
	}
}

func TestApply_ReqMapOnRawBodyWithoutParsedValue(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`)
	result, err := Apply(nil, "application/json", body, nil, dslconfig.RequestTransform{
		ReqMapMode: "openai_chat_to_openai_responses",
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Root == nil {
		t.Fatalf("expected parsed root after req_map")
	}
	if _, exists := result.Root["messages"]; exists {
		t.Fatalf("messages should be removed after req_map, got=%v", result.Root)
	}
}

func TestApplyReqMap_RejectsEncodedClientRequest(t *testing.T) {
	t.Parallel()

	_, err := ApplyReqMap("openai_chat_to_openai_responses", []byte(`{}`), ApplyOptions{
		ContentEncoding: "gzip",
	})
	if err == nil {
		t.Fatalf("expected encoded request error")
	}
}

func TestApply_MultipartPreservesOriginalBody(t *testing.T) {
	t.Parallel()

	original := []byte("--boundary\r\n...")
	value := map[string]any{"model": "gpt-image-1", "n": "2"}
	result, err := Apply(&dslmeta.Meta{}, "multipart/form-data; boundary=boundary", original, value, dslconfig.RequestTransform{}, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if string(result.Body) != string(original) {
		t.Fatalf("multipart body changed: %q", string(result.Body))
	}
}
