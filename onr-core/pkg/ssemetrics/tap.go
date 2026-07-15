package ssemetrics

import (
	"bytes"
	"strings"
)

// EventDataHandler consumes one complete SSE event/data JSON payload together
// with the current SSE event name. Use this handler when metrics or extraction
// rules depend on event filters such as "response.completed".
type EventDataHandler interface {
	OnSSEEventDataJSON(event string, payload []byte) error
}

// DataHandler consumes one complete SSE data JSON payload without the SSE event
// name. Use this only for payload-level side effects that do not need event
// filtering, such as recording the latest raw upstream chunk.
type DataHandler interface {
	OnSSEDataJSON(payload []byte) error
}

type handlerFinisher interface {
	Finish()
}

type EventDataHandlerChain struct {
	handlers []EventDataHandler
}

func NewEventDataHandlerChain(handlers ...EventDataHandler) *EventDataHandlerChain {
	chain := &EventDataHandlerChain{}
	chain.handlers = append(chain.handlers, handlers...)
	return chain

}

func (h *EventDataHandlerChain) OnSSEEventDataJSON(event string, payload []byte) error {
	for _, handler := range h.handlers {
		if handler == nil {
			continue
		}
		_ = handler.OnSSEEventDataJSON(event, payload)
	}
	return nil
}

func (h *EventDataHandlerChain) Finish() {
	if h == nil {
		return
	}
	for _, handler := range h.handlers {
		if finisher, ok := handler.(handlerFinisher); ok {
			finisher.Finish()
		}
	}
}

type DataHandlerChain struct {
	handlers []DataHandler
}

func NewDataHandlerChain(handlers ...DataHandler) *DataHandlerChain {
	chain := &DataHandlerChain{}
	chain.handlers = append(chain.handlers, handlers...)
	return chain

}

func (h *DataHandlerChain) OnSSEDataJSON(payload []byte) error {
	for _, handler := range h.handlers {
		if handler == nil {
			continue
		}
		_ = handler.OnSSEDataJSON(payload)
	}
	return nil
}

func (h *DataHandlerChain) Finish() {
	if h == nil {
		return
	}
	for _, handler := range h.handlers {
		if finisher, ok := handler.(handlerFinisher); ok {
			finisher.Finish()
		}
	}
}

// Tap incrementally parses SSE framing and forwards complete event/data payloads.
//
// It is intentionally transport-agnostic: callers may feed raw bytes via Write,
// or pre-split logical lines via ProcessLine.
type Tap struct {
	eventDataHandler EventDataHandler
	dataHandler      DataHandler

	lineBuf  []byte
	curEvent string
	curData  [][]byte
}

// Option customizes a Tap.
type Option func(*Tap)

// WithDataHandler registers a payload-only handler. The handler receives the
// joined data payload after SSE framing is parsed, but it does not receive the
// event name.
func WithDataHandler(dataHandler DataHandler) Option {
	return func(t *Tap) {
		if t == nil {
			return
		}
		t.dataHandler = dataHandler
	}
}

// WithEventDataHandler registers a handler that receives both the current SSE
// event name and the joined data payload. Prefer this for metrics extraction,
// finish-reason extraction, and stream format checks that may use event filters.
func WithEventDataHandler(eventdataHandler EventDataHandler) Option {
	return func(t *Tap) {
		if t == nil {
			return
		}
		t.eventDataHandler = eventdataHandler
	}
}

// NewTap constructs a shared SSE framing tap.
// NewTap returns nil only when both event and payload-only handlers are absent.
func NewTap(opt ...Option) *Tap {
	t := &Tap{}

	for _, opt := range opt {
		if opt != nil {
			opt(t)
		}

	}

	if t.eventDataHandler == nil && t.dataHandler == nil {
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
	t.finishHandlers()
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
	if t.dataHandler != nil {
		_ = t.dataHandler.OnSSEDataJSON(payload)
	}
	if t.eventDataHandler != nil {
		_ = t.eventDataHandler.OnSSEEventDataJSON(t.curEvent, payload)
	}
	t.curEvent = ""
}

func (t *Tap) finishHandlers() {
	if finisher, ok := t.dataHandler.(handlerFinisher); ok {
		finisher.Finish()
	}
	if finisher, ok := t.eventDataHandler.(handlerFinisher); ok {
		finisher.Finish()
	}
}
