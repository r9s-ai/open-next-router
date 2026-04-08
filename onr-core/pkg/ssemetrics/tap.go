package ssemetrics

import (
	"bytes"
	"strings"
)

// EventDataHandler consumes one complete SSE event/data JSON payload.
type EventDataHandler interface {
	OnSSEEventDataJSON(event string, payload []byte) error
}

// Tap incrementally parses SSE framing and forwards complete event/data payloads.
//
// It is intentionally transport-agnostic: callers may feed raw bytes via Write,
// or pre-split logical lines via ProcessLine.
type Tap struct {
	handler   EventDataHandler
	onPayload func(payload []byte)

	lineBuf  []byte
	curEvent string
	curData  [][]byte
}

// Option customizes a Tap.
type Option func(*Tap)

// WithPayloadHook registers a side-effect hook that runs before the payload is
// forwarded to the event handler.
func WithPayloadHook(fn func(payload []byte)) Option {
	return func(t *Tap) {
		if t == nil {
			return
		}
		t.onPayload = fn
	}
}

// NewTap constructs a shared SSE framing tap.
func NewTap(handler EventDataHandler, opts ...Option) *Tap {
	t := &Tap{handler: handler}
	for _, opt := range opts {
		if opt != nil {
			opt(t)
		}
	}
	if t.handler == nil && t.onPayload == nil {
		return nil
	}
	return t
}

// Write feeds raw SSE bytes into the tap.
func (t *Tap) Write(p []byte) (int, error) {
	if t == nil || len(p) == 0 {
		return len(p), nil
	}

	for _, b := range p {
		if b == '\n' {
			t.processLineBytes(bytes.TrimRight(t.lineBuf, "\r"))
			t.lineBuf = t.lineBuf[:0]
			continue
		}
		t.lineBuf = append(t.lineBuf, b)
	}
	return len(p), nil
}

// ProcessLine feeds one logical SSE line into the tap.
func (t *Tap) ProcessLine(line string) {
	if t == nil {
		return
	}
	t.processLineBytes([]byte(strings.TrimRight(line, "\r")))
}

// Finish flushes any buffered line/event state.
func (t *Tap) Finish() {
	if t == nil {
		return
	}
	if len(t.lineBuf) > 0 {
		t.processLineBytes(bytes.TrimRight(t.lineBuf, "\r"))
		t.lineBuf = t.lineBuf[:0]
	}
	t.flush()
}

func (t *Tap) processLineBytes(line []byte) {
	switch {
	case len(bytes.TrimSpace(line)) == 0:
		t.flush()
	case bytes.HasPrefix(line, []byte("event:")):
		t.curEvent = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
	case bytes.HasPrefix(line, []byte("data:")):
		t.curData = append(t.curData, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
	}
}

func (t *Tap) flush() {
	if t == nil || len(t.curData) == 0 {
		if t != nil {
			t.curEvent = ""
		}
		return
	}
	payload := bytes.TrimSpace(bytes.Join(t.curData, []byte{'\n'}))
	t.curData = t.curData[:0]
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		t.curEvent = ""
		return
	}
	if t.onPayload != nil {
		t.onPayload(payload)
	}
	if t.handler != nil {
		_ = t.handler.OnSSEEventDataJSON(t.curEvent, payload)
	}
	t.curEvent = ""
}
