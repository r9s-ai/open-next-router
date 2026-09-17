package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackJavaScriptIslandsAndFileReferences(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "providers.conf")
	output := filepath.Join(dir, "bundle.conf")
	body := "{\n const value=` include never.conf; ${ {x:1}.x } `;\n const regex=/[{}]/;\n ctx.state.value=value;\n}"
	config := `provider "p" {defaults {upstream_config {base_url="https://example.com";} request {request_by_js_block ` + body + ` after_req_map {request_by_js_file not-deployed.js;}}} match api="chat.completions" {upstream {set_path "/v1/chat/completions";} metrics {usage_fact input token path="$.usage.prompt_tokens";usage_fact output token path="$.usage.completion_tokens";}}}`
	if err := os.WriteFile(source, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--providers", source, "--out", output, "--check", "all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("pack code=%d: %s", code, stderr.String())
	}
	bundle, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bundle), body) || !strings.Contains(string(bundle), "not-deployed.js") {
		t.Fatal("bundle changed JS or resolved host-owned files")
	}
}
