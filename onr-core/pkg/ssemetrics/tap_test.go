package ssemetrics

import (
	"errors"
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

type recordingDataHandler struct {
	payloads []string
}

func (h *recordingDataHandler) OnSSEDataJSON(payload []byte) error {
	h.payloads = append(h.payloads, string(payload))
	return nil
}

type recordingFinishDataHandler struct {
	recordingDataHandler
	finished int
}

func (h *recordingFinishDataHandler) Finish() {
	h.finished++
}

type recordingFinishEventHandler struct {
	recordingHandler
	finished int
}

func (h *recordingFinishEventHandler) Finish() {
	h.finished++
}

func TestTap_WriteForwardsEventAndPayload(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{}
	tap := NewTap(WithEventDataHandler(handler))

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
	dataHandler := &recordingDataHandler{}
	tap := NewTap(WithEventDataHandler(handler), WithDataHandler(dataHandler))

	tap.ProcessLine("event: message_delta")
	tap.ProcessLine(`data: {"delta":"hello"}`)
	tap.ProcessLine("")
	tap.Finish()

	if len(dataHandler.payloads) != 1 || dataHandler.payloads[0] != `{"delta":"hello"}` {
		t.Fatalf("data handler payloads=%v want delta payload", dataHandler.payloads)
	}
	if len(handler.payloads) != 1 || handler.payloads[0] != dataHandler.payloads[0] {
		t.Fatalf("handler payloads=%v want [%q]", handler.payloads, dataHandler.payloads[0])
	}
}

func TestTap_LargeChunkAcrossWrites(t *testing.T) {
	t.Parallel()

	handler := &recordingHandler{}
	tap := NewTap(WithEventDataHandler(handler))

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
	tap := NewTap(WithEventDataHandler(handler))

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

func TestTap_FinishForwardsToHandlers(t *testing.T) {
	t.Parallel()

	eventHandler := &recordingFinishEventHandler{}
	dataHandler := &recordingFinishDataHandler{}
	tap := NewTap(WithEventDataHandler(eventHandler), WithDataHandler(dataHandler))

	tap.ProcessLine(`data: {"delta":"hello"}`)
	tap.Finish()

	if dataHandler.finished != 1 {
		t.Fatalf("data handler finished=%d want=1", dataHandler.finished)
	}
	if eventHandler.finished != 1 {
		t.Fatalf("event handler finished=%d want=1", eventHandler.finished)
	}
	if len(dataHandler.payloads) != 1 || dataHandler.payloads[0] != `{"delta":"hello"}` {
		t.Fatalf("data handler payloads=%v want delta payload", dataHandler.payloads)
	}
}

type erroringHandler struct {
	err error
}

func (h erroringHandler) OnSSEEventDataJSON(string, []byte) error {
	return h.err
}

func TestSSEEventHandlerChain_ForwardsToAllHandlers(t *testing.T) {
	t.Parallel()

	first := &recordingHandler{}
	second := &recordingHandler{}
	chain := NewEventDataHandlerChain(nil, first, erroringHandler{err: errors.New("ignored")}, second)

	if err := chain.OnSSEEventDataJSON("response.completed", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("OnSSEEventDataJSON err=%v, want nil", err)
	}

	for name, handler := range map[string]*recordingHandler{
		"first":  first,
		"second": second,
	} {
		if len(handler.events) != 1 || handler.events[0] != "response.completed" {
			t.Fatalf("%s events=%v want [response.completed]", name, handler.events)
		}
		if len(handler.payloads) != 1 || handler.payloads[0] != `{"ok":true}` {
			t.Fatalf("%s payloads=%v want payload", name, handler.payloads)
		}
	}
}

func TestSSEHandlerChains_ForwardFinish(t *testing.T) {
	t.Parallel()

	eventHandler := &recordingFinishEventHandler{}
	dataHandler := &recordingFinishDataHandler{}

	NewEventDataHandlerChain(nil, eventHandler).Finish()
	NewDataHandlerChain(nil, dataHandler).Finish()

	if eventHandler.finished != 1 {
		t.Fatalf("event handler finished=%d want=1", eventHandler.finished)
	}
	if dataHandler.finished != 1 {
		t.Fatalf("data handler finished=%d want=1", dataHandler.finished)
	}
}
