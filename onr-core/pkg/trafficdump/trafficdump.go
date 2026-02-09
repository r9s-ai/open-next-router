package trafficdump

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestid"
)

const (
	ctxKeyRecorder = "onr.traffic_dump_recorder"
)

var imageB64FieldRegex = regexp.MustCompile(`"(b64_json|image|mask)"\s*:\s*"[^"]*"`)

type Config struct {
	Enabled     bool
	Dir         string
	FilePath    string
	MaxBytes    int
	MaskSecrets bool
}

type Recorder struct {
	mu       sync.Mutex
	f        *os.File
	maxBytes int
	mask     bool
	closed   bool
}

func Enabled(cfg Config) bool { return cfg.Enabled }

func RequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v := strings.TrimSpace(c.GetString(requestid.HeaderKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.GetHeader(requestid.HeaderKey)); v != "" {
		return v
	}
	id := requestid.Gen()
	c.Set(requestid.HeaderKey, id)
	c.Header(requestid.HeaderKey, id)
	return id
}

func Start(c *gin.Context, cfg Config) (*Recorder, error) {
	return StartWithRequestID(c, cfg, "")
}

// StartWithRequestID starts a new traffic dump recorder using a provided request_id.
//
// This is designed to allow external projects to reuse this package while keeping
// their own request_id generation logic.
//
// Template variables for cfg.FilePath:
//   - {{.request_id}} (recommended)
func StartWithRequestID(c *gin.Context, cfg Config, requestID string) (*Recorder, error) {
	if c == nil {
		return nil, errors.New("context is nil")
	}
	if strings.TrimSpace(cfg.Dir) == "" {
		return nil, errors.New("traffic_dump.dir is empty")
	}
	if strings.TrimSpace(cfg.FilePath) == "" {
		return nil, errors.New("traffic_dump.file_path is empty")
	}
	if cfg.MaxBytes < 0 {
		return nil, errors.New("traffic_dump.max_bytes must be non-negative")
	}

	rid := strings.TrimSpace(requestID)
	if rid == "" {
		rid = RequestID(c)
	} else {
		c.Set(requestid.HeaderKey, rid)
		c.Header(requestid.HeaderKey, rid)
	}

	data := map[string]string{
		"request_id": rid,
	}
	tmpl, err := template.New("path").Parse(cfg.FilePath)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	dir := strings.TrimSpace(cfg.Dir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, buf.String())
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	// #nosec G304 -- path is derived from configured dump dir and template.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}

	r := &Recorder{
		f:        f,
		maxBytes: cfg.MaxBytes,
		mask:     cfg.MaskSecrets,
	}
	c.Set(ctxKeyRecorder, r)

	r.writeLine("=== META ===")
	r.writeLine(fmt.Sprintf("time=%s", time.Now().Format(time.RFC3339)))
	r.writeLine(fmt.Sprintf("request_id=%s", rid))
	r.writeLine(fmt.Sprintf("method=%s", c.Request.Method))
	r.writeLine(fmt.Sprintf("path=%s", maskURLIfNeeded(c.Request.URL.String(), r.mask)))
	r.writeLine(fmt.Sprintf("client_ip=%s", c.ClientIP()))
	r.writeLine("headers:")
	for k, vals := range c.Request.Header {
		for _, v := range vals {
			r.writeLine(fmt.Sprintf("  %s: %s", k, maskIfNeeded(k, v, r.mask)))
		}
	}
	r.writeLine("")

	return r, nil
}

func FromContext(c *gin.Context) *Recorder {
	if c == nil {
		return nil
	}
	v, ok := c.Get(ctxKeyRecorder)
	if !ok {
		return nil
	}
	rec, _ := v.(*Recorder)
	return rec
}

func (r *Recorder) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	_ = r.f.Close()
}

func (r *Recorder) MaxBytes() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxBytes
}

func (r *Recorder) writeLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	_, _ = r.f.WriteString(s)
	_, _ = r.f.WriteString("\n")
}

func (r *Recorder) writeBlock(title string, content []byte, binary bool, truncated bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	_, _ = r.f.WriteString(title)
	_, _ = r.f.WriteString("\n")
	if binary {
		_, _ = r.f.WriteString("[base64]\n")
		enc := base64.StdEncoding.EncodeToString(content)
		_, _ = r.f.WriteString(enc)
		_, _ = r.f.WriteString("\n")
	} else {
		_, _ = r.f.Write(content)
		if len(content) == 0 || content[len(content)-1] != '\n' {
			_, _ = r.f.WriteString("\n")
		}
	}
	if truncated {
		_, _ = r.f.WriteString("[truncated]\n")
	}
	_, _ = r.f.WriteString("\n")
}

func maskIfNeeded(key, val string, on bool) string {
	if !on {
		return val
	}
	lk := strings.ToLower(key)
	if strings.Contains(lk, "authorization") ||
		strings.Contains(lk, "api-key") ||
		lk == "x-api-key" ||
		lk == "cookie" ||
		strings.Contains(lk, "token") {
		return "[REDACTED]"
	}
	return val
}

func maskURLIfNeeded(rawURL string, on bool) string {
	if !on {
		return rawURL
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	q := u.Query()
	if len(q) == 0 {
		return rawURL
	}

	shouldRedactKey := func(k string) bool {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "" {
			return false
		}
		// Gemini native uses `key=...` query parameter.
		if lk == "key" || lk == "api_key" || lk == "apikey" {
			return true
		}
		// common patterns
		if strings.Contains(lk, "token") || strings.Contains(lk, "secret") {
			return true
		}
		return false
	}

	changed := false
	for k := range q {
		if !shouldRedactKey(k) {
			continue
		}
		// Keep the key, redact all values.
		q.Set(k, "[REDACTED]")
		changed = true
	}
	if !changed {
		return rawURL
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func isBinaryByContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	return !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
}

func isImageEditPath(path string) bool {
	return strings.HasPrefix(path, "/v1/images/edits")
}

func omitBinaryBodyForDump(path string, ct string) bool {
	if !isImageEditPath(path) {
		return false
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	return strings.HasPrefix(ct, "multipart/form-data")
}

func redactImageBase64Fields(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	return imageB64FieldRegex.ReplaceAllFunc(body, func(m []byte) []byte {
		s := string(m)
		idx := strings.Index(s, ":")
		if idx < 0 {
			return []byte(`"[REDACTED]"`)
		}
		return []byte(s[:idx+1] + `"[[OMITTED]]"`)
	})
}

func AppendOriginRequest(c *gin.Context, body []byte, binary bool, truncated bool) {
	if r := FromContext(c); r != nil {
		ct := ""
		path := ""
		if c != nil && c.Request != nil && c.Request.URL != nil {
			ct = c.Request.Header.Get("Content-Type")
			path = c.Request.URL.Path
		}
		if omitBinaryBodyForDump(path, ct) && (binary || isBinaryByContentType(ct)) {
			summary := fmt.Sprintf("[binary body omitted] content_type=%s content_length=%d captured_bytes=%d", ct, c.Request.ContentLength, len(body))
			r.writeBlock("=== ORIGIN REQUEST ===", []byte(summary+"\n"), false, false)
			return
		}
		if isImageEditPath(path) && !binary {
			body = redactImageBase64Fields(body)
		}
		r.writeBlock("=== ORIGIN REQUEST ===", body, binary, truncated)
	}
}

func AppendUpstreamRequest(
	c *gin.Context,
	method string,
	url string,
	headers map[string][]string,
	body []byte,
	binary bool,
	truncated bool,
) {
	if r := FromContext(c); r != nil {
		r.writeLine("=== UPSTREAM REQUEST ===")
		r.writeLine(fmt.Sprintf("%s %s", method, maskURLIfNeeded(url, r.mask)))
		ct := ""
		for k, vals := range headers {
			for _, v := range vals {
				r.writeLine(fmt.Sprintf("  %s: %s", k, maskIfNeeded(k, v, r.mask)))
			}
			if strings.EqualFold(k, "Content-Type") && len(vals) > 0 {
				ct = vals[0]
			}
		}
		r.writeLine("")
		path := ""
		if c != nil && c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		if omitBinaryBodyForDump(path, ct) && (binary || isBinaryByContentType(ct)) {
			summary := fmt.Sprintf("[binary body omitted] content_type=%s captured_bytes=%d", ct, len(body))
			r.writeBlock("", []byte(summary+"\n"), false, false)
			return
		}
		if isImageEditPath(path) && !binary {
			body = redactImageBase64Fields(body)
		}
		r.writeBlock("", body, binary, truncated)
	}
}

func AppendUpstreamResponse(
	c *gin.Context,
	statusLine string,
	headers map[string][]string,
	body []byte,
	binary bool,
	truncated bool,
) {
	if r := FromContext(c); r != nil {
		r.writeLine("=== UPSTREAM RESPONSE ===")
		r.writeLine(statusLine)
		ct := ""
		for k, vals := range headers {
			for _, v := range vals {
				r.writeLine(fmt.Sprintf("  %s: %s", k, maskIfNeeded(k, v, r.mask)))
			}
			if strings.EqualFold(k, "Content-Type") && len(vals) > 0 {
				ct = vals[0]
			}
		}
		r.writeLine("")
		path := ""
		if c != nil && c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		if omitBinaryBodyForDump(path, ct) && (binary || isBinaryByContentType(ct)) {
			summary := fmt.Sprintf("[binary body omitted] content_type=%s captured_bytes=%d", ct, len(body))
			r.writeBlock("", []byte(summary+"\n"), false, false)
			return
		}
		if isImageEditPath(path) && !binary {
			body = redactImageBase64Fields(body)
		}
		r.writeBlock("", body, binary, truncated)
	}
}

func AppendProxyResponse(c *gin.Context, body []byte, binary bool, truncated bool, statusCode int) {
	if r := FromContext(c); r != nil {
		r.writeLine("=== PROXY RESPONSE ===")
		r.writeLine(fmt.Sprintf("status=%d", statusCode))
		r.writeLine("")
		ct := ""
		path := ""
		if c != nil && c.Writer != nil {
			ct = c.Writer.Header().Get("Content-Type")
		}
		if c != nil && c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		if omitBinaryBodyForDump(path, ct) && (binary || isBinaryByContentType(ct)) {
			summary := fmt.Sprintf("[binary body omitted] content_type=%s captured_bytes=%d", ct, len(body))
			r.writeBlock("", []byte(summary+"\n"), false, false)
			return
		}
		if isImageEditPath(path) && !binary {
			body = redactImageBase64Fields(body)
		}
		r.writeBlock("", body, binary, truncated)
	}
}

// AppendStreamSummary appends a best-effort streaming summary section to the dump.
// It is intentionally generic, so both passthrough and transformed streams can reuse it.
func AppendStreamSummary(c *gin.Context, bytesCopied int64, errMsg string, ignoredClientDisconnect bool) {
	if r := FromContext(c); r != nil {
		r.writeLine("=== STREAM ===")
		r.writeLine(fmt.Sprintf("bytes_copied=%d", bytesCopied))
		if strings.TrimSpace(errMsg) != "" {
			r.writeLine(fmt.Sprintf("error=%s", errMsg))
		}
		if ignoredClientDisconnect {
			r.writeLine("ignored_client_disconnect=true")
		}
		r.writeLine("")
	}
}

func LimitBytes(b []byte, max int) (out []byte, truncated bool) {
	if max <= 0 {
		return nil, false
	}
	if len(b) <= max {
		return b, false
	}
	return b[:max], true
}
