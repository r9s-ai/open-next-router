package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAPI(t *testing.T) {
	got, err := resolveAPI("", "openai-chat")
	if err != nil {
		t.Fatalf("resolveAPI failed: %v", err)
	}
	if got != "chat.completions" {
		t.Fatalf("api=%q want chat.completions", got)
	}

	if _, err := resolveAPI("responses", "openai-chat"); err == nil {
		t.Fatalf("expected conflicting api and route to fail")
	}
}

func TestBuildEstimateInputJSON(t *testing.T) {
	entry := dumpEntry{
		ID: 1,
		Request: dumpSide{Body: dumpBody{
			Format:  "json",
			Content: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
		}},
		Response: dumpSide{Body: dumpBody{
			Format:  "json",
			Content: []byte(`{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`),
		}},
	}

	in, err := buildEstimateInput(entry, "chat.completions", "gpt-4o-mini", false)
	if err != nil {
		t.Fatalf("buildEstimateInput failed: %v", err)
	}
	if string(in.RequestBody) != `{"messages":[{"role":"user","content":"hello"}]}` {
		t.Fatalf("unexpected request body: %s", in.RequestBody)
	}
	if len(in.ResponseBody) == 0 || len(in.StreamTail) != 0 {
		t.Fatalf("unexpected response body/stream sizes: response=%d stream=%d", len(in.ResponseBody), len(in.StreamTail))
	}
	if in.UpstreamUsage == nil {
		t.Fatalf("expected upstream usage")
	}
	if in.UpstreamUsage.InputTokens != 3 || in.UpstreamUsage.OutputTokens != 2 || in.UpstreamUsage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %+v", *in.UpstreamUsage)
	}
}

func TestBuildEstimateInputSSE(t *testing.T) {
	entry := dumpEntry{
		ID:      2,
		Request: dumpSide{Body: dumpBody{Format: "empty"}},
		Response: dumpSide{Body: dumpBody{
			Format: "sse",
			Events: []dumpSSEEvent{
				{Event: "content_block_delta", Data: []byte(`{"delta":{"text":"hello "}}`)},
				{Event: "content_block_delta", Data: []byte(`{"delta":{"text":"world"}}`)},
				{Event: "message_start", Data: []byte(`{"message":{"usage":{"input_tokens":7}}}`)},
				{Data: []byte(`{"usage":{"output_tokens":4}}`)},
				{Data: []byte(`"[DONE]"`)},
			},
		}},
	}

	in, err := buildEstimateInput(entry, "claude.messages", "claude-sonnet-4-5", false)
	if err != nil {
		t.Fatalf("buildEstimateInput failed: %v", err)
	}
	if len(in.StreamTail) != 0 {
		t.Fatalf("unexpected stream tail: %q", string(in.StreamTail))
	}
	resp := string(in.ResponseBody)
	if !strings.Contains(resp, "hello world") {
		t.Fatalf("unexpected response body: %q", resp)
	}
	if in.UpstreamUsage == nil {
		t.Fatalf("expected upstream usage")
	}
	if in.UpstreamUsage.InputTokens != 7 || in.UpstreamUsage.OutputTokens != 4 || in.UpstreamUsage.TotalTokens != 11 {
		t.Fatalf("unexpected usage: %+v", *in.UpstreamUsage)
	}
}

func TestBuildEstimateInputClaudeMessagesAddsCacheInputTokens(t *testing.T) {
	entry := dumpEntry{
		ID:      3,
		Request: dumpSide{Body: dumpBody{Format: "empty"}},
		Response: dumpSide{Body: dumpBody{
			Format: "sse",
			Events: []dumpSSEEvent{
				{Event: "message_start", Data: []byte(`{"message":{"usage":{"input_tokens":8,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":1}}}`)},
				{Event: "message_delta", Data: []byte(`{"usage":{"input_tokens":8,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":12}}`)},
			},
		}},
	}

	in, err := buildEstimateInput(entry, "claude.messages", "claude-haiku-4-5", false)
	if err != nil {
		t.Fatalf("buildEstimateInput failed: %v", err)
	}
	if in.UpstreamUsage == nil {
		t.Fatalf("expected upstream usage")
	}
	if in.UpstreamUsage.InputTokens != 58 || in.UpstreamUsage.OutputTokens != 12 || in.UpstreamUsage.TotalTokens != 70 {
		t.Fatalf("unexpected claude usage: %+v", *in.UpstreamUsage)
	}
	if in.UpstreamUsage.InputTokenDetails == nil ||
		in.UpstreamUsage.InputTokenDetails.CachedTokens != 30 ||
		in.UpstreamUsage.InputTokenDetails.CacheWriteTokens != 20 {
		t.Fatalf("unexpected input token details: %+v", in.UpstreamUsage.InputTokenDetails)
	}
}

func TestBuildEstimateInputNonClaudeDoesNotAddAnthropicCacheInputTokens(t *testing.T) {
	entry := dumpEntry{
		ID:      4,
		Request: dumpSide{Body: dumpBody{Format: "empty"}},
		Response: dumpSide{Body: dumpBody{
			Format:  "json",
			Content: []byte(`{"usage":{"input_tokens":8,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":12}}`),
		}},
	}

	in, err := buildEstimateInput(entry, "responses", "gpt-5-mini", false)
	if err != nil {
		t.Fatalf("buildEstimateInput failed: %v", err)
	}
	if in.UpstreamUsage == nil {
		t.Fatalf("expected upstream usage")
	}
	if in.UpstreamUsage.InputTokens != 8 || in.UpstreamUsage.OutputTokens != 12 || in.UpstreamUsage.TotalTokens != 20 {
		t.Fatalf("unexpected non-claude usage: %+v", *in.UpstreamUsage)
	}
}

func TestBuildEstimateInputRejectsTruncated(t *testing.T) {
	entry := dumpEntry{
		Request: dumpSide{Body: dumpBody{Format: "json", Truncated: true, Content: []byte(`{"ping":"pong"}`)}},
	}
	if _, err := buildEstimateInput(entry, "responses", "gpt-5-mini", false); err == nil {
		t.Fatalf("expected truncated body to fail")
	}
}

func TestRunParsesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
	if err := os.WriteFile(path, []byte(`{
  "id": 1,
  "request": {"body": {"format": "json", "size": 15, "truncated": false, "content": {"ping": "pong"}}},
  "response": {"body": {"format": "json", "size": 43, "truncated": false, "content": {"ok": true, "usage": {"input_tokens": 1, "output_tokens": 2}}}}
}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--file", path, "--route", "openai-responses", "--model", "gpt-5-mini"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status") ||
		!strings.Contains(stdout.String(), "in.actual") ||
		!strings.Contains(stdout.String(), "estimated") ||
		!strings.Contains(stdout.String(), "summary entries=1 estimated=1 skipped=0") ||
		strings.Contains(stdout.String(), "total: actual=") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestRunParsesMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
	if err := os.WriteFile(path, []byte(`[
  {
    "id": 1,
    "request": {"body": {"format": "json", "size": 15, "truncated": false, "content": {"ping": "pong"}}},
    "response": {"body": {"format": "json", "size": 43, "truncated": false, "content": {"ok": true, "usage": {"input_tokens": 1, "output_tokens": 2}}}}
  },
  {
    "id": 2,
    "request": {"body": {"format": "empty", "size": 0, "truncated": false}},
    "response": {"body": {"format": "sse", "size": 71, "truncated": false, "events": [
      {"event": "response.created", "data": {"type": "response.created"}},
      {"data": "[DONE]"}
    ]}}
  }
]`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--file", path, "--route", "openai-responses", "--model", "gpt-5-mini"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "estimated") ||
		!strings.Contains(out, "skipped") ||
		!strings.Contains(out, "token usage not detected") ||
		!strings.Contains(out, "summary entries=2 estimated=1 skipped=1") ||
		strings.Contains(out, "total: actual=") {
		t.Fatalf("unexpected stdout: %s", out)
	}
}

func TestRunParsesJSONStreamEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.log")
	if err := os.WriteFile(path, []byte(`{
  "id": 1,
  "request": {"body": {"format": "json", "size": 15, "truncated": false, "content": {"ping": "pong"}}},
  "response": {"body": {"format": "json", "size": 43, "truncated": false, "content": {"ok": true, "usage": {"input_tokens": 1, "output_tokens": 2}}}}
}
{
  "id": 2,
  "request": {"body": {"format": "empty", "size": 0, "truncated": false}},
  "response": {"body": {"format": "json", "size": 11, "truncated": false, "content": {"ok": true}}}
}
`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--file", path, "--route", "openai-responses", "--model", "gpt-5-mini"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "estimated") ||
		!strings.Contains(out, "skipped") ||
		!strings.Contains(out, "token usage not detected") ||
		!strings.Contains(out, "summary entries=2 estimated=1 skipped=1") ||
		strings.Contains(out, "total: actual=") {
		t.Fatalf("unexpected stdout: %s", out)
	}
}

func TestRunDebugIDWritesExtractedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
	debugDir := filepath.Join(dir, "debug")
	if err := os.WriteFile(path, []byte(`[
  {
    "id": 7,
    "request": {"body": {"format": "json", "size": 15, "truncated": false, "content": {"input": "hello"}}},
    "response": {"body": {"format": "sse", "size": 100, "truncated": false, "events": [
      {"event": "response.output_text.delta", "data": {"type": "response.output_text.delta", "delta": "hello debug"}},
      {"event": "response.completed", "data": {"type": "response.completed", "response": {"usage": {"input_tokens": 1, "output_tokens": 2}}}}
    ]}}
  },
  {
    "id": 8,
    "request": {"body": {"format": "json", "size": 15, "truncated": false, "content": {"input": "skip"}}},
    "response": {"body": {"format": "json", "size": 43, "truncated": false, "content": {"ok": true, "usage": {"input_tokens": 1, "output_tokens": 2}}}}
  }
]`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--file", path, "--api", "responses", "--model", "gpt-5-mini", "--debug-id", "7", "--debug-dir", debugDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "debug dump id=7") ||
		!strings.Contains(out, `request_file=`) ||
		!strings.Contains(out, `request_chars=12`) ||
		!strings.Contains(out, `response_file=`) ||
		!strings.Contains(out, "response_chars=11") ||
		strings.Contains(out, "summary entries=") ||
		strings.Contains(out, "status") ||
		strings.Contains(out, "estimated") {
		t.Fatalf("unexpected stdout: %s", out)
	}
	requestPath := filepath.Join(debugDir, "onr-token-estimate-7-request.txt")
	responsePath := filepath.Join(debugDir, "onr-token-estimate-7-response.txt")
	requestText, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request debug file: %v", err)
	}
	responseText, err := os.ReadFile(responsePath)
	if err != nil {
		t.Fatalf("read response debug file: %v", err)
	}
	if string(requestText) != "input\nhello\n" {
		t.Fatalf("unexpected request debug file: %q", string(requestText))
	}
	if string(responseText) != "hello debug" {
		t.Fatalf("unexpected response debug file: %q", string(responseText))
	}
}

func TestRunDebugIDReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
	if err := os.WriteFile(path, []byte(`{
  "id": 7,
  "request": {"body": {"format": "empty", "size": 0, "truncated": false}},
  "response": {"body": {"format": "empty", "size": 0, "truncated": false}}
}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--file", path, "--api", "responses", "--model", "gpt-5-mini", "--debug-id", "999"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.String() != "" || !strings.Contains(stderr.String(), "error: debug dump id 999 not found") {
		t.Fatalf("unexpected output stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}
