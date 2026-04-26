package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestApplyNonStreamResponseJSONOps_UsesObjectWhenPresent(t *testing.T) {
	t.Parallel()

	respOutObj := map[string]any{
		"a":    float64(1),
		"drop": "x",
	}
	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": []string{"gzip"},
		},
	}
	meta := &dslmeta.Meta{API: "chat.completions"}
	ops := []dslconfig.JSONOp{
		{Op: "json_del", Path: "$.drop"},
		{Op: "json_set", Path: "$.b", ValueExpr: "2"},
	}

	outBody, outCT, changed, err := applyNonStreamResponseJSONOps(
		respOutObj,
		[]byte("not-json-and-not-gzip"),
		"application/json",
		resp,
		meta,
		ops,
		false,
	)
	if err != nil {
		t.Fatalf("applyNonStreamResponseJSONOps: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if outCT != "application/json" {
		t.Fatalf("content-type=%q want application/json", outCT)
	}

	var got map[string]any
	if err := json.Unmarshal(outBody, &got); err != nil {
		t.Fatalf("json.Unmarshal output: %v", err)
	}
	if _, ok := got["drop"]; ok {
		t.Fatalf("expected drop removed, got=%#v", got)
	}
	if got["a"] != float64(1) {
		t.Fatalf("a=%v want=1", got["a"])
	}
	if got["b"] != float64(2) {
		t.Fatalf("b=%v want=2", got["b"])
	}
}

func TestApplyNonStreamResponseJSONOps_DecodesBodyWhenObjectMissing(t *testing.T) {
	t.Parallel()

	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write([]byte(`{"a":1,"drop":"x"}`)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": []string{"gzip"},
		},
	}
	meta := &dslmeta.Meta{API: "chat.completions"}
	ops := []dslconfig.JSONOp{
		{Op: "json_del", Path: "$.drop"},
		{Op: "json_set", Path: "$.b", ValueExpr: "2"},
	}

	outBody, outCT, changed, err := applyNonStreamResponseJSONOps(
		nil,
		compressed.Bytes(),
		"application/json",
		resp,
		meta,
		ops,
		false,
	)
	if err != nil {
		t.Fatalf("applyNonStreamResponseJSONOps: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if outCT != "application/json" {
		t.Fatalf("content-type=%q want application/json", outCT)
	}

	var got map[string]any
	if err := json.Unmarshal(outBody, &got); err != nil {
		t.Fatalf("json.Unmarshal output: %v", err)
	}
	if _, ok := got["drop"]; ok {
		t.Fatalf("expected drop removed, got=%#v", got)
	}
	if got["a"] != float64(1) {
		t.Fatalf("a=%v want=1", got["a"])
	}
	if got["b"] != float64(2) {
		t.Fatalf("b=%v want=2", got["b"])
	}
}

func TestApplyNonStreamResponseJSONOps_MarshalsBodyWhenOpsPresentEvenIfNoSemanticChange(t *testing.T) {
	t.Parallel()

	originalBody := []byte(`{"a":"same"}`)
	respOutObj := map[string]any{
		"a": "same",
	}
	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": []string{"gzip"},
		},
	}
	meta := &dslmeta.Meta{API: "chat.completions"}
	ops := []dslconfig.JSONOp{
		{Op: "json_set", Path: "$.a", ValueExpr: "\"same\""},
		{Op: "json_del", Path: "$.missing"},
	}

	outBody, outCT, changed, err := applyNonStreamResponseJSONOps(
		respOutObj,
		originalBody,
		"application/json",
		resp,
		meta,
		ops,
		false,
	)
	if err != nil {
		t.Fatalf("applyNonStreamResponseJSONOps: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if outCT != "application/json" {
		t.Fatalf("content-type=%q want application/json", outCT)
	}
	if string(outBody) != string(originalBody) {
		t.Fatalf("body=%s want marshaled=%s", outBody, originalBody)
	}
}
