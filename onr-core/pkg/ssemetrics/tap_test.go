package ssemetrics

import (
	"strings"
	"testing"
)

type recordingHandler struct {
	events   []string
	payloads []string
}

func (h *recordingHandler) OnSSEEventDataJSON(event string, payload []byte) error {
	h.events = append(h.events, event)
	h.payloads = append(h.payloads, string(payload))
	return nil
}

func TestTap_WriteForwardsEventAndPayload(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{}
	tap := NewTap(handler)

	stream := "" +
		"event: response.completed\n" +
		`data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n"

	if _, err := tap.Write([]byte(stream)); err != nil {
		t.Fatalf("tap.Write: %v", err)
	}
	tap.Finish()

	if len(handler.events) != 1 {
		t.Fatalf("events=%d want=1", len(handler.events))
	}
	if handler.events[0] != "response.completed" {
		t.Fatalf("event=%q want=response.completed", handler.events[0])
	}
	if got := handler.payloads[0]; got != `{"type":"response.completed","response":{"status":"completed"}}` {
		t.Fatalf("payload=%q", got)
	}
}

func TestTap_ProcessLineInvokesPayloadHook(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{}
	var seen string
	tap := NewTap(handler, WithPayloadHook(func(payload []byte) {
		seen = string(payload)
	}))

	tap.ProcessLine("event: message_delta")
	tap.ProcessLine(`data: {"delta":"hello"}`)
	tap.ProcessLine("")
	tap.Finish()

	if seen != `{"delta":"hello"}` {
		t.Fatalf("payload hook=%q want delta payload", seen)
	}
	if len(handler.payloads) != 1 || handler.payloads[0] != seen {
		t.Fatalf("handler payloads=%v want [%q]", handler.payloads, seen)
	}
}

func TestTap_LargeChunkAcrossWrites(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{}
	tap := NewTap(handler)

	stream := "event: image_generation.completed\n" +
		`data: {"type":"image_generation.completed","b64_json":"` + strings.Repeat("A", 400000) + `"}` + "\n\n"

	if _, err := tap.Write([]byte(stream[:200000])); err != nil {
		t.Fatalf("tap.Write chunk1: %v", err)
	}
	if _, err := tap.Write([]byte(stream[200000:])); err != nil {
		t.Fatalf("tap.Write chunk2: %v", err)
	}
	tap.Finish()

	if len(handler.events) != 1 {
		t.Fatalf("events=%d want=1", len(handler.events))
	}
	if handler.events[0] != "image_generation.completed" {
		t.Fatalf("event=%q want=image_generation.completed", handler.events[0])
	}
}

func TestTap_SkipsDonePayload(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{}
	tap := NewTap(handler)

	stream := "" +
		"event: done\n" +
		"data: [DONE]\n\n"

	if _, err := tap.Write([]byte(stream)); err != nil {
		t.Fatalf("tap.Write: %v", err)
	}
	tap.Finish()

	if len(handler.events) != 0 {
		t.Fatalf("events=%v want none", handler.events)
	}
}
