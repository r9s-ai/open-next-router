package proxy

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

type streamDumpState struct {
	enabled  bool
	appended bool

	upBuf []byte
	upTr  bool
	prBuf []byte
	prTr  bool

	streamBytes                   int64
	streamErrMsg                  string
	streamIgnoredClientDisconnect bool
}

// newStreamDumpState returns a non-nil dump state.
func newStreamDumpState(gc *gin.Context) *streamDumpState {
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		return &streamDumpState{enabled: true}
	}
	return &streamDumpState{}
}

// SetUpstream requires a valid streamDumpState receiver.
func (d *streamDumpState) SetUpstream(buf []byte, truncated bool) {
	d.upBuf = buf
	d.upTr = truncated
}

// SetProxy requires a valid streamDumpState receiver.
func (d *streamDumpState) SetProxy(buf []byte, truncated bool) {
	d.prBuf = buf
	d.prTr = truncated
}

// SetStreamResult requires a valid streamDumpState receiver.
func (d *streamDumpState) SetStreamResult(bytesCopied int64, err error, ignoredClientDisconnect bool) {
	d.streamBytes = bytesCopied
	if err != nil {
		d.streamErrMsg = err.Error()
	}
	d.streamIgnoredClientDisconnect = ignoredClientDisconnect
}

// Append requires a valid streamDumpState receiver.
func (d *streamDumpState) Append(gc *gin.Context, resp *http.Response) {
	if !d.enabled || d.appended {
		return
	}
	// Always write response sections once (even if body is empty), to make dumps debuggable.
	// Note: stream bytes might be partial when the client disconnects.
	ctUp := strings.ToLower(resp.Header.Get("Content-Type"))
	upBinary := isBinaryDumpPayload(ctUp, d.upBuf)
	trafficdump.AppendUpstreamResponse(gc, resp.Status, resp.Header, d.upBuf, upBinary, d.upTr)

	ctPr := strings.ToLower(gc.Writer.Header().Get("Content-Type"))
	if strings.TrimSpace(ctPr) == "" {
		ctPr = ctUp
	}
	prBinary := isBinaryDumpPayload(ctPr, d.prBuf)
	trafficdump.AppendProxyResponse(gc, d.prBuf, prBinary, d.prTr, resp.StatusCode)
	trafficdump.AppendStreamSummary(gc, d.streamBytes, d.streamErrMsg, d.streamIgnoredClientDisconnect)
	d.appended = true
}
