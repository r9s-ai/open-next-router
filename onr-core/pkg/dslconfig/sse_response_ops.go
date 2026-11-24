package dslconfig

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

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
// Large-event risk: if an upstream sends a single data: line carrying megabytes
// of JSON (e.g. a provider that returns a full document as one SSE event), every
// concurrent stream hitting such an event will each allocate proportionally,
// raising GC pressure or causing OOM at high concurrency. There is currently no
// size limit on accumulated lines.
//
// Resolution decision: mainstream AI API upstreams (OpenAI, Anthropic) emit
// token-delta events of at most a few KB each, so the unbounded accumulation is
// not a practical risk for the current workload. If a deployment targets custom
// upstreams with documented large-event patterns, a byte cap should be added in
// addLine — it must return an error when exceeded so flush and its callers
// (TransformSSEEventDataJSON, SSEEventDataJSONWriter) can surface it cleanly.
type sseEventBuf struct {
	rawLines  [][]byte
	dataParts [][]byte
	metaLines [][]byte
}

func (b *sseEventBuf) reset() {
	b.rawLines = b.rawLines[:0]
	b.dataParts = b.dataParts[:0]
	b.metaLines = b.metaLines[:0]
}

func (b *sseEventBuf) isEmpty() bool {
	return len(b.rawLines) == 0 && len(b.dataParts) == 0 && len(b.metaLines) == 0
}

func (b *sseEventBuf) addLine(line []byte) {
	if len(line) == 0 {
		return
	}
	b.rawLines = append(b.rawLines, append([]byte(nil), line...))
	if bytes.HasPrefix(line, []byte("data:")) {
		rest := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		b.dataParts = append(b.dataParts, append([]byte(nil), rest...))
		return
	}
	b.metaLines = append(b.metaLines, append([]byte(nil), line...))
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

// Write implements [io.Writer]. It buffers bytes until complete SSE lines are available,
// then applies the configured rules and ops to each complete event.
func (w *SSEEventDataJSONWriter) Write(p []byte) (int, error) {
	written := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			w.partial.Write(p)
			break
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
		w.ev.addLine(trimmed) // addLine copies trimmed before we reset
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

		if len(bytes.TrimSpace(line)) == 0 {
			if err2 := ev.flush(w, meta, rules, ops, counts); err2 != nil {
				return err2
			}
			ev.reset()
		} else {
			ev.addLine(line)
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
