package proxy

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

type countingWriter struct {
	n int64
	w io.Writer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

func streamToDownstream(
	gc *gin.Context,
	meta *dslmeta.Meta,
	respDir dslconfig.ResponseDirective,
	resp *http.Response,
	usageTail *tailBuffer,
	dump *streamDumpState,
) (int64, error) {
	needSSEOps := len(respDir.JSONOps) > 0 || len(respDir.SSEJSONDelIf) > 0
	mode := strings.ToLower(strings.TrimSpace(respDir.Mode))
	useStrategyTransform := strings.TrimSpace(respDir.Op) == "sse_parse" &&
		(mode == "openai_responses_to_openai_chat_chunks" ||
			mode == "anthropic_to_openai_chunks" ||
			mode == "openai_to_anthropic_chunks" ||
			mode == "openai_to_gemini_chunks" ||
			mode == "gemini_to_openai_chat_chunks")

	var upstreamDump *limitedBuffer
	var proxyDump *limitedBuffer

	rec := trafficdump.FromContext(gc)
	if rec != nil && rec.MaxBytes() > 0 {
		upstreamDump = &limitedBuffer{limit: rec.MaxBytes()}
		proxyDump = &limitedBuffer{limit: rec.MaxBytes()}
	}

	src, err := buildStreamSource(gc, resp, mode, respDir.Mode, needSSEOps, useStrategyTransform, upstreamDump)
	if err != nil {
		return 0, err
	}

	if upstreamDump != nil && !useStrategyTransform {
		src = io.TeeReader(src, upstreamDump)
	}

	// Always tee the post-strategy stream into usageTail (pre-response-ops).
	src = io.TeeReader(src, usageTail)

	dst := io.Writer(gc.Writer)
	if proxyDump != nil {
		dst = io.MultiWriter(dst, proxyDump)
	}
	cw := &countingWriter{w: dst}

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

	if dump != nil && upstreamDump != nil && proxyDump != nil {
		dump.SetUpstream(upstreamDump.Bytes(), upstreamDump.Truncated())
		dump.SetProxy(proxyDump.Bytes(), proxyDump.Truncated())
	}

	return cw.n, err
}

func buildStreamSource(
	gc *gin.Context,
	resp *http.Response,
	mode string,
	rawMode string,
	needSSEOps bool,
	useStrategyTransform bool,
	upstreamDump *limitedBuffer,
) (io.Reader, error) {
	if useStrategyTransform {
		return buildStrategyTransformSource(gc, resp, mode, rawMode, upstreamDump)
	}
	return buildPassthroughSource(gc, resp, needSSEOps)
}

func buildStrategyTransformSource(
	gc *gin.Context,
	resp *http.Response,
	mode string,
	rawMode string,
	upstreamDump *limitedBuffer,
) (io.Reader, error) {
	pr, pw := io.Pipe()
	upSrc, closeUp, err := decodeUpstreamIfNeeded(resp, true)
	if err != nil {
		return nil, err
	}
	if closeUp != nil {
		defer func() { _ = closeUp() }()
	}
	if upstreamDump != nil {
		upSrc = io.TeeReader(upSrc, upstreamDump)
	}

	gc.Writer.Header().Set("Content-Type", "text/event-stream")
	gc.Writer.Header().Set("Cache-Control", "no-cache")
	gc.Status(resp.StatusCode)

	go func() {
		err := runSSEStrategyTransform(mode, rawMode, upSrc, pw)
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
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

func runSSEStrategyTransform(mode, rawMode string, src io.Reader, dst io.Writer) error {
	switch mode {
	case "openai_responses_to_openai_chat_chunks":
		return apitransform.TransformOpenAIResponsesSSEToChatCompletionsSSE(src, dst)
	case "anthropic_to_openai_chunks":
		return apitransform.TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(src, dst)
	case "openai_to_anthropic_chunks":
		return apitransform.TransformOpenAIChatCompletionsSSEToClaudeMessagesSSE(src, dst)
	case "openai_to_gemini_chunks":
		return apitransform.TransformOpenAIChatCompletionsSSEToGeminiSSE(src, dst)
	case "gemini_to_openai_chat_chunks":
		return apitransform.TransformGeminiSSEToOpenAIChatCompletionsSSE(src, dst)
	default:
		return fmt.Errorf("unsupported sse_parse mode %q", rawMode)
	}
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

// streamTransformedOpenAIResponses and streamPassthrough were merged into streamToDownstream
// to support response-phase SSE JSON mutations (json_* / sse_json_del_if).
