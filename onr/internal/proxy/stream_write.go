package proxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

type countingWriter struct {
	n            int64
	w            io.Writer
	firstWriteAt time.Time
}

func (w *countingWriter) Write(p []byte) (int, error) {
	if w.firstWriteAt.IsZero() && len(p) > 0 {
		w.firstWriteAt = time.Now()
	}
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

func streamToDownstream(
	gc *gin.Context,
	meta *dslmeta.Meta,
	respDir *dslconfig.ResponseDirective,
	resp *http.Response,
	usageTail *tailBuffer,
	metricsTap *sseMetricsTap,
	tapRawSSEForMetrics bool,
	dump *streamDumpState,
) (int64, time.Time, error) {
	var (
		needSSEOps bool
		mode       string
		rawMode    string
	)
	if respDir != nil {
		needSSEOps = len(respDir.JSONOps) > 0 || len(respDir.SSEJSONDelIf) > 0
		mode = strings.ToLower(strings.TrimSpace(respDir.Mode))
		rawMode = respDir.Mode
	}
	useStrategyTransform := respDir != nil && strings.TrimSpace(respDir.Op) == "sse_parse" &&
		apitransform.SupportsSSETransformMode(mode)

	var upstreamDump *limitedBuffer
	var proxyDump *limitedBuffer

	rec := trafficdump.FromContext(gc)
	if rec != nil && rec.MaxBytes() > 0 {
		upstreamDump = &limitedBuffer{limit: rec.MaxBytes()}
		proxyDump = &limitedBuffer{limit: rec.MaxBytes()}
	}

	dst := io.Writer(gc.Writer)
	if proxyDump != nil {
		dst = io.MultiWriter(dst, proxyDump)
	}
	cw := &countingWriter{w: dst}

	var err error
	if useStrategyTransform {
		err = streamStrategyTransform(gc, resp, meta, rawMode, respDir, needSSEOps,
			upstreamDump, usageTail, metricsTap, tapRawSSEForMetrics, cw)
	} else {
		src, serr := buildPassthroughSource(gc, resp, needSSEOps)
		if serr != nil {
			return 0, time.Time{}, serr
		}
		if upstreamDump != nil {
			src = io.TeeReader(src, upstreamDump)
		}
		src = io.TeeReader(src, usageTail)
		if metricsTap != nil {
			src = io.TeeReader(src, metricsTap)
		}

		ctLower := strings.ToLower(strings.TrimSpace(gc.Writer.Header().Get("Content-Type")))
		if ctLower == "" {
			ctLower = strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
		}
		isSSE := strings.Contains(ctLower, "text/event-stream")

		if needSSEOps && isSSE {
			err = dslconfig.TransformSSEEventDataJSON(src, cw, meta, respDir.SSEJSONDelIf, respDir.JSONOps)
		} else {
			_, err = io.Copy(cw, src)
		}
	}

	if dump != nil && upstreamDump != nil && proxyDump != nil {
		dump.SetUpstream(upstreamDump.Bytes(), upstreamDump.Truncated())
		dump.SetProxy(proxyDump.Bytes(), proxyDump.Truncated())
	}
	if metricsTap != nil {
		metricsTap.Finish()
	}

	return cw.n, cw.firstWriteAt, err
}

// streamStrategyTransform runs the sse_parse strategy transform synchronously, writing
// directly into cw without a goroutine or pipe. Upstream decoding, dump capture, and
// metric collection are composed via readers and writers on the source path.
func streamStrategyTransform(
	gc *gin.Context,
	resp *http.Response,
	meta *dslmeta.Meta,
	rawMode string,
	respDir *dslconfig.ResponseDirective,
	needSSEOps bool,
	upstreamDump *limitedBuffer,
	usageTail *tailBuffer,
	metricsTap *sseMetricsTap,
	tapRawSSEForMetrics bool,
	cw *countingWriter,
) error {
	src, closeUp, err := decodeUpstreamIfNeeded(resp, true)
	if err != nil {
		return err
	}
	if closeUp != nil {
		defer func() { _ = closeUp() }()
	}
	if upstreamDump != nil {
		src = io.TeeReader(src, upstreamDump)
	}
	if tapRawSSEForMetrics {
		var taps []io.Writer
		if usageTail != nil {
			taps = append(taps, usageTail)
		}
		if metricsTap != nil {
			taps = append(taps, metricsTap)
		}
		if len(taps) > 0 {
			src = io.TeeReader(src, io.MultiWriter(taps...))
		}
	}

	gc.Writer.Header().Set("Content-Type", "text/event-stream")
	gc.Writer.Header().Set("Cache-Control", "no-cache")
	gc.Status(resp.StatusCode)

	// Build the transform destination from inside out:
	//   innermost: cw (the client writer, possibly including proxyDump)
	//   middle:    SSEEventDataJSONWriter (applies JSON ops per event, if configured)
	//   outermost: MultiWriter fanning to metric writers (when tapping transform output)

	var transformDst io.Writer = cw
	var sseWriter *dslconfig.SSEEventDataJSONWriter
	if needSSEOps {
		sseWriter = dslconfig.NewSSEEventDataJSONWriter(cw, meta, respDir.SSEJSONDelIf, respDir.JSONOps)
		transformDst = sseWriter
	}

	if !tapRawSSEForMetrics {
		// Tap metrics from transform output (post-strategy, pre-JSON-ops).
		var taps []io.Writer
		taps = append(taps, transformDst)
		if usageTail != nil {
			taps = append(taps, usageTail)
		}
		if metricsTap != nil {
			taps = append(taps, metricsTap)
		}
		if len(taps) > 1 {
			transformDst = io.MultiWriter(taps...)
		}
	}

	err = apitransform.TransformSSEByMode(rawMode, src, transformDst)
	if sseWriter != nil && err == nil {
		err = sseWriter.Finish()
	}
	return err
}

func buildPassthroughSource(gc *gin.Context, resp *http.Response, needSSEOps bool) (io.Reader, error) {
	gc.Status(resp.StatusCode)
	if !needSSEOps {
		return resp.Body, nil
	}
	src, closeUp, err := decodeUpstreamIfNeeded(resp, false)
	if err != nil {
		return nil, err
	}
	if closeUp != nil {
		// passthrough branch uses returned reader directly; caller lifecycle is stream-lifetime.
		// keep close bound to body close by wrapping reader.
		src = &readCloserReader{Reader: src, closeFn: closeUp}
	}
	return src, nil
}

func decodeUpstreamIfNeeded(resp *http.Response, forceDecode bool) (io.Reader, func() error, error) {
	ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if ce == contentEncodingGzip {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		resp.Header.Del("Content-Encoding")
		return gr, gr.Close, nil
	}
	if ce != "" && ce != contentEncodingIdentity && forceDecode {
		return nil, nil, fmt.Errorf("cannot transform encoded upstream response (Content-Encoding=%q)", resp.Header.Get("Content-Encoding"))
	}
	if ce != "" && ce != contentEncodingIdentity && !forceDecode {
		return nil, nil, fmt.Errorf("cannot transform encoded upstream response (Content-Encoding=%q)", resp.Header.Get("Content-Encoding"))
	}
	return resp.Body, nil, nil
}

type readCloserReader struct {
	io.Reader
	closeFn func() error
}

func (r *readCloserReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF && r.closeFn != nil {
		_ = r.closeFn()
		r.closeFn = nil
	}
	return n, err
}
