package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestStreamFormatCheckerCountsEventPayloads(t *testing.T) {
	t.Parallel()

	checker := NewStreamFormatChecker(&dslmeta.Meta{API: "responses"})
	if checker.Result() == nil {
		t.Fatal("Result()=nil, want initialized result")
	}

	_ = checker.OnSSEEventDataJSON("response.created", []byte(`{"type":"response.created"}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`{"type":"response.output_text.delta"}`))
	_ = checker.OnSSEEventDataJSON("response.completed", nil)
	_ = checker.OnSSEEventDataJSON("response.completed", []byte(`{"type":"response.completed"}`))

	res := checker.Result()
	if got := res.Api; got != "responses" {
		t.Fatalf("Api=%q want responses", got)
	}
	if got := res.ChunkCount; got != 3 {
		t.Fatalf("ChunkCount=%d want 3", got)
	}
	if got := res.EventsSeen["response.created"]; got != 1 {
		t.Fatalf("EventsSeen[response.created]=%d want 1", got)
	}
	if got := res.EventsSeen["response.output_text.delta"]; got != 1 {
		t.Fatalf("EventsSeen[response.output_text.delta]=%d want 1", got)
	}
	if got := res.EventsSeen["response.completed"]; got != 1 {
		t.Fatalf("EventsSeen[response.completed]=%d want 1", got)
	}
	if res.Payload == nil {
		t.Fatal("Payload=nil, want payload summary")
	}
	if got := res.Payload.CheckedCount; got != 3 {
		t.Fatalf("Payload.CheckedCount=%d want 3", got)
	}
	if got := res.Payload.ValidCount; got != 3 {
		t.Fatalf("Payload.ValidCount=%d want 3", got)
	}
	if got := res.Payload.InvalidCount; got != 0 {
		t.Fatalf("Payload.InvalidCount=%d want 0", got)
	}
}

func TestStreamFormatCheckerClassifiesDataOnlyPayloads(t *testing.T) {
	t.Parallel()

	checker := NewStreamFormatChecker(&dslmeta.Meta{})
	_ = checker.OnSSEEventDataJSON("", []byte(`{"choices":[{"delta":{"content":"hi"},"finish_reason":null}]}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`{"choices":[],"usage":{"prompt_tokens":1}}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`{"candidates":[{"finishReason":"STOP"}]}`))

	res := checker.Result()
	assertEventCount(t, res.EventsSeen, "chat.completion.chunk", 1)
	assertEventCount(t, res.EventsSeen, "chat.completion.finish", 1)
	assertEventCount(t, res.EventsSeen, "chat.completion.usage", 1)
	assertEventCount(t, res.EventsSeen, "gemini.generate_content.chunk", 1)
	assertEventCount(t, res.EventsSeen, "gemini.generate_content.finish", 1)
	if got := res.Payload.CheckedCount; got != 5 {
		t.Fatalf("Payload.CheckedCount=%d want 5", got)
	}
	if got := res.Payload.ValidCount; got != 5 {
		t.Fatalf("Payload.ValidCount=%d want 5", got)
	}
	if got := res.Payload.InvalidCount; got != 0 {
		t.Fatalf("Payload.InvalidCount=%d want 0", got)
	}
}

func TestStreamFormatCheckerRecordsPayloadIssues(t *testing.T) {
	t.Parallel()

	checker := NewStreamFormatChecker(&dslmeta.Meta{})
	_ = checker.OnSSEEventDataJSON("", []byte(`not-json`))
	_ = checker.OnSSEEventDataJSON("response.completed", []byte(`{"type":"response.created"}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`{"choices":{}}`))
	_ = checker.OnSSEEventDataJSON("", []byte(`[{"type":"response.created"}]`))

	res := checker.Result()
	assertEventCount(t, res.EventsSeen, "fallback_data", 3)
	assertEventCount(t, res.EventsSeen, "response.completed", 1)
	if got := res.Payload.CheckedCount; got != 4 {
		t.Fatalf("Payload.CheckedCount=%d want 4", got)
	}
	if got := res.Payload.InvalidCount; got != 4 {
		t.Fatalf("Payload.InvalidCount=%d want 4", got)
	}
	assertIssue(t, res.Payload.Issues, "fallback_data", "invalid_json", "")
	assertIssue(t, res.Payload.Issues, "response.completed", "event_type_mismatch", "type")
	assertIssue(t, res.Payload.Issues, "fallback_data", "invalid_choices_field", "choices")
	assertIssue(t, res.Payload.Issues, "fallback_data", "invalid_payload_root", "")
}

func assertEventCount(t *testing.T, events map[string]int, event string, want int) {
	t.Helper()
	if got := events[event]; got != want {
		t.Fatalf("EventsSeen[%s]=%d want %d", event, got, want)
	}
}

func assertIssue(t *testing.T, issues []PayloadIssue, event string, code string, path string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Event == event && issue.Code == code && issue.Path == path {
			return
		}
	}
	t.Fatalf("missing issue event=%q code=%q path=%q in %#v", event, code, path, issues)
}
