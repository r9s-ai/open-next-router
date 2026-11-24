package dslconfig

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

// ErrSSELineTooLong is returned when a single SSE line (content before \n)
// exceeds maxSSELineBytes. It bounds partial-line accumulation in
// SSEEventDataJSONWriter.Write and the ReadBytes buffer in
// TransformSSEEventDataJSON.
var ErrSSELineTooLong = errors.New("sse: line exceeds maximum byte length")

// ErrSSEEventTooLarge is returned when the total byte length of all lines
// accumulated in one SSE event by sseEventBuf.addLine exceeds maxSSEEventBytes.
var ErrSSEEventTooLarge = errors.New("sse: event exceeds maximum byte length")

// maxSSELineBytes is the maximum byte length of a single SSE line (without \n).
// Declared as var so tests can override it without allocating large buffers.
var maxSSELineBytes = 1 * 1024 * 1024 // 1 MiB

// maxSSEEventBytes is the maximum total byte length of all lines in one SSE
// event as tracked by sseEventBuf.addLine.
// Declared as var so tests can override it without allocating large buffers.
var maxSSEEventBytes = 4 * 1024 * 1024 // 4 MiB

// sseEventBuf accumulates the lines of one SSE event between field lines and
// the blank-line terminator. rawLines holds every non-empty line verbatim;
// dataParts holds the trimmed payload of each "data:" line; metaLines holds
// all non-data field lines (event:, id:, retry:, comments).
//
// Memory model: addLine copies each line into the struct so the caller's read
// buffer can be reused immediately (primary copy, lives until reset()).
// writeRaw and writeNormalizedJSON assemble a bytes.Buffer from these copies
// to do a single w.Write call (secondary copy, transient — freed after Write
// returns). The JSON transform path additionally allocates for json.Unmarshal
// (parsed object) and json.Marshal (output bytes). Peak memory per in-flight
// event is roughly 2× raw event size for passthrough events, and up to ~5×
// for large JSON transform events.
//
// Two independent unbounded-accumulation paths are both bounded by size caps:
//
//  1. Partial-line path: SSEEventDataJSONWriter.Write checks w.partial.Len()+
//     pending bytes against maxSSELineBytes before accumulating; TransformSSEEventDataJSON
//     checks len(line) against maxSSELineBytes after ReadBytes trims the newline.
//     Either path returns ErrSSELineTooLong when exceeded.
//
//  2. Event-level path: addLine tracks totalBytes across all lines in the current
//     event and returns ErrSSEEventTooLarge when the cumulative length would exceed
//     maxSSEEventBytes, bounding per-event and per-stream allocation.
type sseEventBuf struct {
	rawLines   [][]byte
	dataParts  [][]byte
	metaLines  [][]byte
	totalBytes int
}

func (b *sseEventBuf) reset() {
	b.rawLines = b.rawLines[:0]
	b.dataParts = b.dataParts[:0]
	b.metaLines = b.metaLines[:0]
	b.totalBytes = 0
}

func (b *sseEventBuf) isEmpty() bool {
	return len(b.rawLines) == 0 && len(b.dataParts) == 0 && len(b.metaLines) == 0
}

// addLine appends line to the event buffer. It returns ErrSSEEventTooLarge if
// the cumulative byte length of all lines in the current event would exceed
// maxSSEEventBytes. line must be a complete SSE field line without a trailing \n.
func (b *sseEventBuf) addLine(line []byte) error {
	if len(line) == 0 {
		return nil
	}
	if b.totalBytes+len(line) > maxSSEEventBytes {
		return ErrSSEEventTooLarge
	}
	b.totalBytes += len(line)
	b.rawLines = append(b.rawLines, append([]byte(nil), line...))
	if bytes.HasPrefix(line, []byte("data:")) {
		rest := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		b.dataParts = append(b.dataParts, append([]byte(nil), rest...))
		return nil
	}
	b.metaLines = append(b.metaLines, append([]byte(nil), line...))
	return nil
}

func (b *sseEventBuf) payload() []byte {
	return bytes.TrimSpace(bytes.Join(b.dataParts, []byte{'\n'}))
}

func (b *sseEventBuf) eventName() string {
	for _, line := range b.metaLines {
		if bytes.HasPrefix(line, []byte("event:")) {
			return string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
		}
	}
	return ""
}

// writeRaw emits the buffered event unchanged as a single w.Write call.
// All lines are assembled into one bytes.Buffer first because w is typically
// a flush-on-write wrapper (e.g. flushWriter in the relay); a single Write
// means a single Flush per event, which is the invariant this package
// establishes. The bytes.Buffer is the secondary copy described in sseEventBuf
// and is freed immediately after Write returns.
//
// Two alternatives that do not work here:
//   - sync.Pool for buf: a large event causes the pool to retain a large
//     buffer until the next GC cycle, amplifying peak RSS under concurrency.
//   - Chunked writes (flush buf to w mid-event): still multiple w.Write calls,
//     so still multiple Flush calls per event — the problem we are solving.
func (b *sseEventBuf) writeRaw(w io.Writer) error {
	var buf bytes.Buffer
	for _, l := range b.rawLines {
		buf.Write(l)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// writeNormalizedJSON emits the transformed event as a single w.Write call.
// metaLines (event:, id:, etc.) are prepended to the already-marshalled
// jsonBytes and written together so that w sees exactly one Write per event.
// The bytes.Buffer is the secondary copy described in sseEventBuf and is freed
// immediately after Write returns. See writeRaw for the rationale on why
// pooling and chunked writes are not viable here.
func (b *sseEventBuf) writeNormalizedJSON(w io.Writer, jsonBytes []byte) error {
	var buf bytes.Buffer
	for _, l := range b.metaLines {
		buf.Write(l)
		buf.WriteByte('\n')
	}
	buf.WriteString("data: ")
	buf.Write(jsonBytes)
	buf.WriteString("\n\n")
	_, err := w.Write(buf.Bytes())
	return err
}

func (b *sseEventBuf) flush(w io.Writer, meta *dslmeta.Meta, rules []SSEJSONDelIfRule, ops []JSONOp, counts []int) error {
	if b.isEmpty() {
		return nil
	}
	payload := b.payload()
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return b.writeRaw(w)
	}

	obj, ok := parseJSONObject(payload)
	if !ok {
		return b.writeRaw(w)
	}

	applySSEJSONDelIf(obj, rules)
	if err := applyJSONOpsToObject(meta, obj, ops, b.eventName(), counts); err != nil {
		return err
	}
	outJSON, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return b.writeNormalizedJSON(w, outJSON)
}

func parseJSONObject(payload []byte) (map[string]any, bool) {
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

// SSEEventDataJSONWriter is an [io.Writer] that applies [SSEJSONDelIfRule] and [JSONOp]
// rules to each complete SSE event's data payload before forwarding to an inner writer.
// It buffers incomplete lines across Write calls. Callers must invoke Finish after the
// last Write to flush any trailing buffered state.
type SSEEventDataJSONWriter struct {
	dst     io.Writer
	meta    *dslmeta.Meta
	rules   []SSEJSONDelIfRule
	ops     []JSONOp
	counts  []int
	ev      sseEventBuf
	partial bytes.Buffer
}

// NewSSEEventDataJSONWriter returns an [SSEEventDataJSONWriter] that transforms each SSE
// event's JSON data payload inline and forwards the result to dst.
func NewSSEEventDataJSONWriter(dst io.Writer, meta *dslmeta.Meta, rules []SSEJSONDelIfRule, ops []JSONOp) *SSEEventDataJSONWriter {
	return &SSEEventDataJSONWriter{
		dst:    dst,
		meta:   meta,
		rules:  rules,
		ops:    ops,
		counts: make([]int, len(ops)),
	}
}

// Write implements [io.Writer]. It buffers bytes until complete SSE lines are
// available, then applies the configured rules and ops to each complete event.
// It returns ErrSSELineTooLong if any single line (content before \n) would
// exceed maxSSELineBytes, and ErrSSEEventTooLarge if the accumulated byte
// length of one event's lines would exceed maxSSEEventBytes.
func (w *SSEEventDataJSONWriter) Write(p []byte) (int, error) {
	written := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			// No \n in p: bytes will accumulate in w.partial until the next Write.
			// addLine is never called for a line without \n, so this is the only
			// guard against unbounded partial-line growth. addLine's event-total
			// check (maxSSEEventBytes = 4 MiB) would not fire here.
			if w.partial.Len()+len(p) > maxSSELineBytes {
				return 0, ErrSSELineTooLong
			}
			w.partial.Write(p)
			break
		}
		// \n found: w.partial + p[:idx] is the full assembled line. addLine will
		// check the cumulative event total (maxSSEEventBytes = 4 MiB), but that
		// is a weaker per-line bound than maxSSELineBytes (1 MiB). Check here
		// before writing to w.partial so the per-line limit is enforced uniformly
		// regardless of whether the line spans multiple Write calls.
		if w.partial.Len()+idx > maxSSELineBytes {
			return 0, ErrSSELineTooLong
		}
		w.partial.Write(p[:idx])
		p = p[idx+1:]
		if err := w.processLine(); err != nil {
			return 0, err
		}
	}
	return written, nil
}

func (w *SSEEventDataJSONWriter) processLine() error {
	trimmed := bytes.TrimRight(w.partial.Bytes(), "\r")
	if len(bytes.TrimSpace(trimmed)) == 0 {
		w.partial.Reset()
		if err := w.ev.flush(w.dst, w.meta, w.rules, w.ops, w.counts); err != nil {
			return err
		}
		w.ev.reset()
	} else {
		if err := w.ev.addLine(trimmed); err != nil { // addLine copies trimmed before we reset
			return err
		}
		w.partial.Reset()
	}
	return nil
}

// Finish flushes any buffered trailing content. Must be called after the last Write.
func (w *SSEEventDataJSONWriter) Finish() error {
	if w.partial.Len() > 0 {
		if err := w.processLine(); err != nil {
			return err
		}
	}
	if !w.ev.isEmpty() {
		if err := w.ev.flush(w.dst, w.meta, w.rules, w.ops, w.counts); err != nil {
			return err
		}
		w.ev.reset()
	}
	return nil
}

// TransformSSEEventDataJSON applies response SSE rules to a text/event-stream.
//
// It reads from r and writes to w, event-by-event, using the SSE framing rules:
// lines are grouped until a blank line; each "data:" line contributes a payload line.
//
// For each event:
//   - If joined data payload is "[DONE]" or empty: event is passed through unchanged.
//   - If payload is a JSON object: apply SSEJSONDelIf (in order), then JSONOps (in order),
//     and emit a normalized event (all original non-data lines preserved, data re-emitted as a single JSON line).
//   - Otherwise: event is passed through unchanged.
func TransformSSEEventDataJSON(r io.Reader, w io.Writer, meta *dslmeta.Meta, rules []SSEJSONDelIfRule, ops []JSONOp) error {
	if len(rules) == 0 && len(ops) == 0 {
		_, err := io.Copy(w, r)
		return err
	}

	br := bufio.NewReader(r)
	var ev sseEventBuf
	counts := make([]int, len(ops))

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimSuffix(line, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
		}
		if len(line) > maxSSELineBytes {
			return ErrSSELineTooLong
		}

		if len(bytes.TrimSpace(line)) == 0 {
			if err2 := ev.flush(w, meta, rules, ops, counts); err2 != nil {
				return err2
			}
			ev.reset()
		} else {
			if err2 := ev.addLine(line); err2 != nil {
				return err2
			}
		}

		if err != nil {
			if err == io.EOF {
				if !ev.isEmpty() {
					if err2 := ev.flush(w, meta, rules, ops, counts); err2 != nil {
						return err2
					}
				}
				return nil
			}
			return err
		}
	}
}

func applySSEJSONDelIf(obj map[string]any, rules []SSEJSONDelIfRule) {
	if obj == nil || len(rules) == 0 {
		return
	}
	for _, r := range rules {
		v, ok := jsonGet(obj, r.CondPath)
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		if s != r.Equals {
			continue
		}
		_, _ = jsonDel(obj, r.DelPath)
	}
}

func jsonGet(root map[string]any, path string) (any, bool) {
	parts, err := parseObjectPath(path)
	if err != nil || len(parts) == 0 {
		return nil, false
	}
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok {
			return nil, false
		}
		m, ok := next.(map[string]any)
		if !ok || m == nil {
			return nil, false
		}
		cur = m
	}
	v, ok := cur[parts[len(parts)-1]]
	return v, ok
}

func applyJSONOpsToObject(meta *dslmeta.Meta, obj map[string]any, ops []JSONOp, event string, counts []int) error {
	if obj == nil || len(ops) == 0 {
		return nil
	}
	for i, op := range ops {
		if shouldSkipJSONOp(event, op, counts, i) {
			continue
		}
		changed := false
		switch op.Op {
		case jsonOpSet:
			val, err := evalJSONValueExpr(meta, op.ValueExpr)
			if err != nil {
				return err
			}
			opChanged, err := jsonSet(obj, op.Path, val)
			if err != nil {
				return err
			}
			changed = opChanged
		case jsonOpReplace:
			val, err := evalJSONValueExpr(meta, op.ValueExpr)
			if err != nil {
				return err
			}
			opChanged, err := jsonReplace(obj, op.Path, val)
			if err != nil {
				return err
			}
			changed = opChanged
		case jsonOpSetIfAbsent:
			exists, err := jsonPathExists(obj, op.Path)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			val, err := evalJSONValueExpr(meta, op.ValueExpr)
			if err != nil {
				return err
			}
			opChanged, err := jsonSet(obj, op.Path, val)
			if err != nil {
				return err
			}
			changed = opChanged
		case jsonOpDel:
			opChanged, err := jsonDel(obj, op.Path)
			if err != nil {
				return err
			}
			changed = opChanged
		case jsonOpRename:
			opChanged, err := jsonRename(obj, op.FromPath, op.ToPath)
			if err != nil {
				return err
			}
			changed = opChanged
		default:
			// validated at load time; ignore unknown here
		}
		recordJSONOpChange(changed, op, counts, i)
	}
	return nil
}
