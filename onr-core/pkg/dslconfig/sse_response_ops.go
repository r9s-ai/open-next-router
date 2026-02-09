package dslconfig

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

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

func (b *sseEventBuf) writeRaw(w io.Writer) error {
	for _, l := range b.rawLines {
		if _, err := w.Write(l); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (b *sseEventBuf) writeNormalizedJSON(w io.Writer, jsonBytes []byte) error {
	for _, l := range b.metaLines {
		if _, err := w.Write(l); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "data: "); err != nil {
		return err
	}
	if _, err := w.Write(jsonBytes); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n\n")
	return err
}

func (b *sseEventBuf) flush(w io.Writer, meta *dslmeta.Meta, rules []SSEJSONDelIfRule, ops []JSONOp) error {
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
	if err := applyJSONOpsToObject(meta, obj, ops); err != nil {
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

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimSuffix(line, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
		}

		if len(bytes.TrimSpace(line)) == 0 {
			if err2 := ev.flush(w, meta, rules, ops); err2 != nil {
				return err2
			}
			ev.reset()
		} else {
			ev.addLine(line)
		}

		if err != nil {
			if err == io.EOF {
				if !ev.isEmpty() {
					if err2 := ev.flush(w, meta, rules, ops); err2 != nil {
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
		_ = jsonDel(obj, r.DelPath)
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

func applyJSONOpsToObject(meta *dslmeta.Meta, obj map[string]any, ops []JSONOp) error {
	if obj == nil || len(ops) == 0 {
		return nil
	}
	for _, op := range ops {
		switch op.Op {
		case jsonOpSet:
			val := evalJSONValueExpr(meta, op.ValueExpr)
			if err := jsonSet(obj, op.Path, val); err != nil {
				return err
			}
		case jsonOpDel:
			if err := jsonDel(obj, op.Path); err != nil {
				return err
			}
		case jsonOpRename:
			if err := jsonRename(obj, op.FromPath, op.ToPath); err != nil {
				return err
			}
		default:
			// validated at load time; ignore unknown here
		}
	}
	return nil
}
