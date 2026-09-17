package jsext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dop251/goja"
)

type Event struct {
	Event string `json:"event"`
	Data  string `json:"data"`
	ID    string `json:"id"`
	Retry string `json:"retry"`
}
type TerminalEvent func(Event) bool

// TerminalForAPI protects the downstream protocol's terminal events. A host
// with additional protocols can supply its own predicate to NewSSEWriter.
func TerminalForAPI(api string) TerminalEvent {
	return func(e Event) bool {
		if strings.TrimSpace(e.Data) == "[DONE]" {
			return true
		}
		var body struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(e.Data), &body)
		kind := e.Event
		matches := func(want string) bool { return kind == want || body.Type == want }
		switch api {
		case "responses":
			return matches("response.completed") || matches("response.failed") || matches("response.incomplete")
		case "claude.messages":
			return matches("message_stop")
		}
		return false
	}
}

type SSEWriter struct {
	ctx             context.Context
	session         *Session
	dst             io.Writer
	terminal        TerminalEvent
	buffer          []byte
	scan, lineStart int
	err             error
}

func NewSSEWriter(ctx context.Context, s *Session, dst io.Writer, terminal TerminalEvent) *SSEWriter {
	return &SSEWriter{ctx: ctx, session: s, dst: dst, terminal: terminal}
}
func (w *SSEWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	// Append incrementally so a large upstream read never bypasses the event cap.
	for i, b := range p {
		w.buffer = append(w.buffer, b)
		if int64(len(w.buffer)) > w.session.Limits().EventBytes {
			w.err = &Error{SSEEvent, fmt.Errorf("event exceeds js_event_limit")}
			return i, w.err
		}
		if err := w.drain(false); err != nil {
			w.err = err
			return i, err
		}
	}
	return len(p), nil
}
func (w *SSEWriter) drain(final bool) error {
	for w.scan < len(w.buffer) {
		c := w.buffer[w.scan]
		if c != '\n' && c != '\r' {
			w.scan++
			continue
		}
		end := w.scan + 1
		if c == '\r' {
			if end == len(w.buffer) && !final {
				return nil
			}
			if end < len(w.buffer) && w.buffer[end] == '\n' {
				end++
			}
		}
		empty := w.scan == w.lineStart
		w.scan = end
		w.lineStart = end
		if empty {
			if err := w.frame(w.buffer[:end]); err != nil {
				return err
			}
			w.buffer = append(w.buffer[:0], w.buffer[end:]...)
			w.scan = 0
			w.lineStart = 0
		}
	}
	return nil
}
func (w *SSEWriter) Finish() error {
	if w.err != nil {
		return w.err
	}
	if err := w.drain(true); err != nil {
		return err
	}
	if len(w.buffer) > 0 {
		return w.frame(w.buffer)
	}
	return nil
}
func (w *SSEWriter) frame(raw []byte) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	event, hasData, comments := parseFrame(raw)
	if !hasData || (w.terminal != nil && w.terminal(event)) {
		return w.write(raw)
	}
	s := w.session
	s.views["event"] = s.jsonValue(event)
	// Final request/response views are frozen once, never copied per event.
	var events []Event
	err := s.invoke(w.ctx, SSEEvent, func(v goja.Value) error {
		if goja.IsUndefined(v) {
			return fmt.Errorf("SSE hook must return an event, array or null")
		}
		if goja.IsNull(v) {
			return nil
		}
		var raw []json.RawMessage
		if v.ToObject(s.vm).ClassName() == "Array" {
			if v.ToObject(s.vm).Get("length").ToInteger() > 64 {
				return fmt.Errorf("SSE expansion exceeds 64 events")
			}
			if err := s.decode(v, &raw); err != nil {
				return err
			}
		} else {
			var single json.RawMessage
			if err := s.decode(v, &single); err != nil {
				return err
			}
			raw = []json.RawMessage{single}
		}
		for _, item := range raw {
			event, err := decodeEvent(item)
			if err != nil {
				return err
			}
			events = append(events, event)
		}

		return validateEvents(events, w.terminal)
	})
	if err != nil {
		return err
	}
	if len(events) == 1 && events[0] == event {
		return w.write(raw)
	}
	var out strings.Builder
	out.WriteString(comments.String())
	for _, e := range events {
		if e.Event != "" {
			fmt.Fprintf(&out, "event: %s\n", e.Event)
		}
		if e.ID != "" {
			fmt.Fprintf(&out, "id: %s\n", e.ID)
		}
		if e.Retry != "" {
			fmt.Fprintf(&out, "retry: %s\n", e.Retry)
		}
		for _, line := range strings.Split(e.Data, "\n") {
			fmt.Fprintf(&out, "data: %s\n", strings.TrimSuffix(line, "\r"))
		}
		out.WriteByte('\n')
		if int64(out.Len()) > s.config.Limits.EventBytes {
			return &Error{SSEEvent, fmt.Errorf("SSE output exceeds js_event_limit")}
		}
	}
	if len(events) == 0 && comments.Len() > 0 {
		out.WriteByte('\n')
	}
	if out.Len() == 0 {
		return nil
	}
	return w.write([]byte(out.String()))
}
func (w *SSEWriter) write(p []byte) error {
	n, err := w.dst.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err == nil {
		if f, ok := w.dst.(interface{ Flush() }); ok {
			f.Flush()
		}
	}
	return err
}

func parseFrame(raw []byte) (Event, bool, *strings.Builder) {
	var event Event
	var data []string
	var comments strings.Builder
	hasData := false
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\r", "\n"), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ":") {
			comments.WriteString(line)
			comments.WriteByte('\n')
			continue
		}
		key, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch key {
		case "event":
			event.Event = value
		case "id":
			event.ID = value
		case "retry":
			event.Retry = value
		case "data":
			hasData = true
			data = append(data, value)
		}
	}
	event.Data = strings.Join(data, "\n")
	return event, hasData, &comments
}

func decodeEvent(raw json.RawMessage) (Event, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Event{}, err
	}
	if fields == nil {
		return Event{}, fmt.Errorf("SSE array entries must be event objects")
	}
	if _, ok := fields["data"]; !ok {
		return Event{}, fmt.Errorf("SSE event requires a data string")
	}
	for key, value := range fields {
		switch key {
		case "event", "data", "id", "retry":
		default:
			return Event{}, fmt.Errorf("unknown SSE field %s", key)
		}
		if len(value) == 0 || value[0] != '"' {
			return Event{}, fmt.Errorf("SSE fields must be strings")
		}
	}
	var event Event
	err := json.Unmarshal(raw, &event)
	return event, err
}

func validateEvents(events []Event, terminal TerminalEvent) error {
	if len(events) > 64 {
		return fmt.Errorf("SSE expansion exceeds 64 events")
	}
	for _, e := range events {
		if strings.ContainsAny(e.Event+e.ID+e.Retry, "\r\n\x00") || strings.Contains(e.Data, "\r") {
			return fmt.Errorf("invalid SSE fields")
		}
		if terminal != nil && terminal(e) {
			return fmt.Errorf("JavaScript cannot synthesize terminal events")
		}
	}
	return nil
}
