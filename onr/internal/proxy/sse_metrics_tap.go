package proxy

import (
	"bytes"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

// sseMetricsTap incrementally parses SSE framing from the post-strategy stream
// and feeds each complete data payload into the metrics aggregator.
type sseMetricsTap struct {
	agg *dslconfig.StreamMetricsAggregator

	lineBuf  []byte
	curEvent string
	curData  [][]byte
}

func newSSEMetricsTap(agg *dslconfig.StreamMetricsAggregator) *sseMetricsTap {
	if agg == nil {
		return nil
	}
	return &sseMetricsTap{agg: agg}
}

func (t *sseMetricsTap) Write(p []byte) (int, error) {
	if t == nil || t.agg == nil || len(p) == 0 {
		return len(p), nil
	}

	for _, b := range p {
		if b == '\n' {
			t.processLine(bytes.TrimRight(t.lineBuf, "\r"))
			t.lineBuf = t.lineBuf[:0]
			continue
		}
		t.lineBuf = append(t.lineBuf, b)
	}
	return len(p), nil
}

func (t *sseMetricsTap) Finish() {
	if t == nil || t.agg == nil {
		return
	}
	if len(t.lineBuf) > 0 {
		t.processLine(bytes.TrimRight(t.lineBuf, "\r"))
		t.lineBuf = t.lineBuf[:0]
	}
	t.flush()
}

func (t *sseMetricsTap) processLine(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		t.flush()
		return
	}
	if bytes.HasPrefix(line, []byte("event:")) {
		t.curEvent = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
		return
	}
	if bytes.HasPrefix(line, []byte("data:")) {
		t.curData = append(t.curData, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
	}
}

func (t *sseMetricsTap) flush() {
	if t == nil || t.agg == nil || len(t.curData) == 0 {
		if t != nil {
			t.curEvent = ""
		}
		return
	}
	payload := bytes.TrimSpace(bytes.Join(t.curData, []byte{'\n'}))
	t.curData = t.curData[:0]
	_ = t.agg.OnSSEEventDataJSON(t.curEvent, payload)
	t.curEvent = ""
}
