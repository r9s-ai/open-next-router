package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

const jsSessionKey = "onr.js.session"

func jsSession(gc *gin.Context) *jsext.Session {
	v, _ := gc.Get(jsSessionKey)
	s, _ := v.(*jsext.Session)
	return s
}
func (c *Client) startJS(gc *gin.Context, pf dslconfig.ProviderFile, m *dslmeta.Meta) (*jsext.Session, error) {
	cfg := pf.JS.Select(m.API, m.IsStream)
	if !cfg.Enabled() {
		return nil, nil
	}
	attempt := gc.GetInt("onr.js.attempt") + 1
	gc.Set("onr.js.attempt", attempt)
	session, err := jsext.NewSession(cfg, jsext.Meta{RequestID: firstNonEmpty(gc.GetString("onr.js.request_id"), gc.GetString("X-Request-Id")), Attempt: attempt, Provider: pf.Name, API: m.API, OriginalModel: m.OriginModelName, UpstreamModel: firstNonEmpty(m.DSLModelMapped, m.OriginModelName), Stream: m.IsStream}, pf.JS.HTTP, func(level, message string, fields map[string]any) {
		if c.SystemLogger != nil {
			details := map[string]any{"provider": pf.Name, "fields": fields}
			switch level {
			case "debug":
				c.SystemLogger.Debug(logx.SystemCategoryServer, "JavaScript: "+message, details)
			case "warn":
				c.SystemLogger.Warn(logx.SystemCategoryServer, "JavaScript: "+message, details)
			case "error":
				c.SystemLogger.Error(logx.SystemCategoryServer, "JavaScript: "+message, details)
			default:
				c.SystemLogger.Info(logx.SystemCategoryServer, "JavaScript: "+message, details)
			}
		}
	})
	if err != nil && c.SystemLogger != nil {
		c.SystemLogger.Error(logx.SystemCategoryServer, "JavaScript session initialization failed", map[string]any{"provider": pf.Name, "error": err.Error()})
	}
	gc.Set(jsSessionKey, session)
	return session, err
}
func (c *Client) finishJS(gc *gin.Context, res *Result, err error) {
	s := jsSession(gc)
	if s == nil {
		return
	}
	outcome := jsext.Outcome{Outcome: "success"}
	if res != nil {
		outcome.Status = res.Status
		outcome.Usage = res.Usage
		outcome.FinishReason = res.FinishReason
		if res.Status >= 400 {
			outcome.Outcome = "upstream_error"
		}
	}
	if err != nil {
		outcome.Outcome = "upstream_error"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			outcome.Outcome = "canceled"
		}
		var scriptErr *jsext.Error
		if errors.As(err, &scriptErr) {
			outcome.Outcome = "script_error"
		}
		var reject *jsext.Rejection
		if errors.As(err, &reject) {
			outcome.Outcome = "rejected"
			outcome.Status = reject.Status
		}
	}
	if gc.Request.Context().Err() != nil {
		outcome.Outcome = "canceled"
	}
	if logErr := s.Finish(outcome); logErr != nil && c.SystemLogger != nil {
		c.SystemLogger.Warn(logx.SystemCategoryServer, "JavaScript log hook failed", map[string]any{"error": logErr.Error()})
	}
}

func runRequestJS(gc *gin.Context, s *jsext.Session, stage jsext.Stage, m *dslmeta.Meta, body []byte, contentType string, headers http.Header) ([]byte, map[string]any, error) {
	msg := jsext.Message{Body: string(body), Headers: headers, ContentType: contentType, Method: gc.Request.Method, Path: jsMessagePath(m.RequestURLPath)}
	if err := s.Run(gc.Request.Context(), stage, &msg); err != nil {
		return nil, nil, err
	}
	if !s.Has(stage) {
		return body, nil, nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(msg.Body), &root); err != nil {
		return nil, nil, &jsext.Error{Stage: stage, Cause: fmt.Errorf("request must remain a JSON object")}
	}
	return []byte(msg.Body), root, nil
}

// Only script changes bypass the DSL header-pass allowlist. Credentials and
// transport headers have already been excluded and validated by jsext.
func headerDelta(before, after http.Header) http.Header {
	out := http.Header{}
	for k, v := range before {
		if !equalStrings(v, after[k]) {
			out[k] = append([]string{}, after[k]...)
		}
	}
	for k, v := range after {
		if !equalStrings(v, before[k]) {
			out[k] = append([]string{}, v...)
		}
	}
	return out
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func applyJSHeaders(gc *gin.Context, headers http.Header) {
	raw, _ := gc.Get("onr.js.headers")
	delta, _ := raw.(http.Header)
	for k, v := range delta {
		headers.Del(k)
		for _, item := range v {
			headers.Add(k, item)
		}
	}
}
func responseHeadersJS(gc *gin.Context, status int) error {
	s := jsSession(gc)
	if s == nil {
		return nil
	}
	message := jsext.Message{Headers: gc.Writer.Header(), ContentType: gc.Writer.Header().Get("Content-Type"), Status: status}
	return s.Run(gc.Request.Context(), jsext.ResponseHeaders, &message)
}
func requiresSSEJS(gc *gin.Context) bool { return jsSession(gc).Has(jsext.SSEEvent) }

type jsFlushWriter struct {
	io.Writer
	flush func()
}

func (w jsFlushWriter) Flush() { w.flush() }

func jsRequestHeaders(gc *gin.Context, m *dslmeta.Meta) http.Header {
	if jsSession(gc) != nil {
		return m.RequestHeaders
	}
	return gc.Request.Header
}

func jsMessagePath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Path
}

func readJSResponseBody(session *jsext.Session, resp *http.Response) ([]byte, error) {
	if !session.Has(jsext.Response) {
		return io.ReadAll(resp.Body)
	}
	decoded, closeDecoded, err := decodeUpstreamIfNeeded(resp, true)
	if err != nil {
		return nil, &jsext.Error{Stage: jsext.Response, Cause: err}
	}
	if closeDecoded != nil {
		defer func() { _ = closeDecoded() }()
	}
	body, err := io.ReadAll(io.LimitReader(decoded, session.Limits().BodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > session.Limits().BodyBytes {
		return nil, &jsext.Error{Stage: jsext.Response, Cause: fmt.Errorf("response exceeds js_body_limit")}
	}
	return body, nil
}

func finalRequestJSHeaders(gc *gin.Context) http.Header {
	raw, _ := gc.Get("onr.js.final_headers")
	headers, _ := raw.(http.Header)
	return headers
}
