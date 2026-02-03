package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	xproxy "golang.org/x/net/proxy"

	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/pkg/trafficdump"
	"github.com/r9s-ai/open-next-router/pkg/usageestimate"
)

const contentEncodingIdentity = "identity"

type ProviderKey struct {
	Name            string
	Value           string
	BaseURLOverride string
}

type Result struct {
	Provider       string
	ProviderKey    string
	ProviderSource string
	API            string
	Stream         bool
	Model          string
	Status         int
	LatencyMs      int64
	Usage          map[string]any
	UsageStage     string
	FinishReason   string
}

type Client struct {
	HTTP         *http.Client
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Registry     *dslconfig.Registry
	UsageEst     *usageestimate.Config

	// ProxyByProvider maps provider name -> outbound HTTP proxy URL.
	// Example: {"openai": "http://127.0.0.1:7890"}.
	ProxyByProvider map[string]string

	mu          sync.Mutex
	httpByProxy map[string]*http.Client
}

type proxyCtx struct {
	start           time.Time
	provider        string
	key             ProviderKey
	api             string
	stream          bool
	pf              dslconfig.ProviderFile
	meta            *dslmeta.Meta
	model           string
	reqBody         []byte
	respDir         dslconfig.ResponseDirective
	reqTransform    dslconfig.RequestTransform
	hasReqTransform bool
}

func (c *Client) ProxyJSON(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	stream bool,
) (*Result, error) {
	bctx, err := c.buildProxyCtx(gc, provider, key, api, stream)
	if err != nil {
		return nil, err
	}
	start := bctx.start
	pf := bctx.pf
	m := bctx.meta
	model := bctx.model
	reqBody := bctx.reqBody
	respDir := bctx.respDir

	resp, err := c.doUpstreamRequest(gc, provider, pf, m, reqBody)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// If upstream returns SSE, treat it as streaming regardless of client "stream" flag.
	effectiveStream := isEffectiveStream(stream, resp)

	if !effectiveStream {
		return c.handleNonStreamResponse(gc, provider, key, api, stream, start, pf, m, model, reqBody, respDir, resp)
	}
	return c.handleStreamResponse(gc, provider, key, api, start, pf, m, model, reqBody, respDir, resp)
}

func (c *Client) handleNonStreamResponse(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	stream bool,
	start time.Time,
	pf dslconfig.ProviderFile,
	m *dslmeta.Meta,
	model string,
	reqBody []byte,
	respDir dslconfig.ResponseDirective,
	resp *http.Response,
) (*Result, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// traffic dump: upstream response (original bytes)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		binary := !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
		limited, truncated := trafficdump.LimitBytes(respBody, rec.MaxBytes())
		trafficdump.AppendUpstreamResponse(gc, resp.Status, resp.Header, limited, binary, truncated)
	}

	// response transform (best-effort)
	respOutBody := respBody
	outCT := resp.Header.Get("Content-Type")
	didTransform := false
	if strings.TrimSpace(respDir.Op) == "resp_map" && strings.EqualFold(strings.TrimSpace(respDir.Mode), "openai_responses_to_openai_chat") {
		decoded, err := maybeDecodeUpstreamBody(respBody, resp.Header.Get("Content-Encoding"))
		if err != nil {
			return nil, err
		}
		srcBody := respBody
		if decoded != nil {
			srcBody = decoded
		}
		respOutBody, err = dslconfig.MapOpenAIResponsesToChatCompletions(srcBody)
		if err != nil {
			return nil, err
		}
		outCT = "application/json"
		didTransform = true
	}

	copyHeadersToClient(gc, resp.Header, didTransform)
	if strings.TrimSpace(outCT) != "" {
		gc.Writer.Header().Set("Content-Type", outCT)
	}

	gc.Status(resp.StatusCode)
	if _, err := gc.Writer.Write(respOutBody); err != nil {
		return nil, err
	}

	// traffic dump: proxy response (mapped bytes)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		ct := strings.ToLower(outCT)
		binary := !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
		limited, truncated := trafficdump.LimitBytes(respOutBody, rec.MaxBytes())
		trafficdump.AppendProxyResponse(gc, limited, binary, truncated, resp.StatusCode)
	}

	var usage map[string]any
	usageStage := ""
	if cfg, ok := pf.Usage.Select(m); ok {
		u, _, err := dslconfig.ExtractUsage(m, cfg, respOutBody)
		if err == nil && u != nil {
			out := usageestimate.Estimate(c.UsageEst, usageestimate.Input{
				API:           api,
				Model:         model,
				UpstreamUsage: u,
				RequestBody:   reqBody,
				ResponseBody:  respOutBody,
			})
			usage = usageMap(out.Usage)
			usageStage = out.Stage
		}
	}
	if usage == nil {
		out := usageestimate.Estimate(c.UsageEst, usageestimate.Input{
			API:          api,
			Model:        model,
			RequestBody:  reqBody,
			ResponseBody: respOutBody,
		})
		usage = usageMap(out.Usage)
		usageStage = out.Stage
	}

	finishReason := ""
	if frCfg, ok := pf.Finish.Select(m); ok {
		if v, err := dslconfig.ExtractFinishReason(m, frCfg, respOutBody); err == nil {
			finishReason = strings.TrimSpace(v)
		}
	}

	return &Result{
		Provider:       provider,
		ProviderKey:    key.Name,
		ProviderSource: "dsl",
		API:            api,
		Stream:         stream,
		Model:          model,
		Status:         resp.StatusCode,
		LatencyMs:      time.Since(start).Milliseconds(),
		Usage:          usage,
		UsageStage:     usageStage,
		FinishReason:   finishReason,
	}, nil
}

func copyHeadersToClient(gc *gin.Context, hdr http.Header, didTransform bool) {
	for k, vs := range hdr {
		if len(vs) == 0 {
			continue
		}
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		if strings.EqualFold(k, "Content-Encoding") && didTransform {
			continue
		}
		gc.Writer.Header().Set(k, vs[0])
	}
}

func (c *Client) handleStreamResponse(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	start time.Time,
	pf dslconfig.ProviderFile,
	m *dslmeta.Meta,
	model string,
	reqBody []byte,
	respDir dslconfig.ResponseDirective,
	resp *http.Response,
) (*Result, error) {
	// stream passthrough
	var (
		dumpBuf       []byte
		dumpTruncated bool
		dumpHandled   bool
	)

	// copy headers
	copyHeadersToClient(gc, resp.Header, false)

	// Always keep a tail buffer for best-effort usage extraction from SSE.
	tailLimit := 256 << 10 // 256KB
	if c.UsageEst != nil && c.UsageEst.MaxStreamCollectBytes > 0 {
		tailLimit = c.UsageEst.MaxStreamCollectBytes
	}
	usageTail := &tailBuffer{limit: tailLimit}

	// stream transform (optional)
	if strings.TrimSpace(respDir.Op) == "sse_parse" && strings.EqualFold(strings.TrimSpace(respDir.Mode), "openai_responses_to_openai_chat_chunks") {
		var src io.Reader = resp.Body
		ce := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
		if ce == "gzip" {
			gr, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil, err
			}
			defer func() {
				_ = gr.Close()
			}()
			src = gr
			// Override encoding for downstream.
			gc.Writer.Header().Del("Content-Encoding")
		} else if ce != "" && ce != contentEncodingIdentity {
			return nil, fmt.Errorf("cannot transform encoded upstream response (Content-Encoding=%q)", resp.Header.Get("Content-Encoding"))
		}

		// Override to downstream chat SSE.
		gc.Writer.Header().Set("Content-Type", "text/event-stream")
		gc.Writer.Header().Set("Cache-Control", "no-cache")

		gc.Status(resp.StatusCode)

		var (
			upDump *limitedBuffer
			prDump *limitedBuffer
		)
		if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
			upDump = &limitedBuffer{limit: rec.MaxBytes()}
			prDump = &limitedBuffer{limit: rec.MaxBytes()}
			// Collect upstream bytes for dump, but collect proxy bytes for metrics/estimation.
			src = io.TeeReader(src, upDump)
			dst := io.MultiWriter(gc.Writer, prDump, usageTail)
			if err := dslconfig.TransformOpenAIResponsesSSEToChatCompletionsSSE(src, dst); err != nil {
				return nil, err
			}
			dumpBuf = upDump.Bytes()
			dumpTruncated = upDump.Truncated()
			// Append both upstream and proxy streams (best-effort).
			trafficdump.AppendUpstreamResponse(gc, resp.Status, resp.Header, dumpBuf, false, dumpTruncated)
			trafficdump.AppendProxyResponse(gc, prDump.Bytes(), false, prDump.Truncated(), resp.StatusCode)
			dumpHandled = true
		} else {
			dst := io.MultiWriter(gc.Writer, usageTail)
			if err := dslconfig.TransformOpenAIResponsesSSEToChatCompletionsSSE(src, dst); err != nil {
				return nil, err
			}
		}
	} else {
		// passthrough
		gc.Status(resp.StatusCode)
		if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
			buf := &limitedBuffer{limit: rec.MaxBytes()}
			tee := io.TeeReader(resp.Body, io.MultiWriter(buf, usageTail))
			if _, err := io.Copy(gc.Writer, tee); err != nil {
				return nil, err
			}
			dumpBuf = buf.Bytes()
			dumpTruncated = buf.Truncated()
		} else {
			tee := io.TeeReader(resp.Body, usageTail)
			if _, err := io.Copy(gc.Writer, tee); err != nil {
				return nil, err
			}
		}
	}
	if f, ok := gc.Writer.(http.Flusher); ok {
		f.Flush()
	}

	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 && !dumpHandled && len(dumpBuf) > 0 && dumpBuf != nil {
		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		binary := !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
		trafficdump.AppendUpstreamResponse(gc, resp.Status, resp.Header, dumpBuf, binary, dumpTruncated)
		trafficdump.AppendProxyResponse(gc, dumpBuf, binary, dumpTruncated, resp.StatusCode)
	}

	// best-effort: extract metrics from SSE stream tail via pkg/dslconfig aggregator
	var upstreamUsage *dslconfig.Usage
	finishReason := ""
	if usageTail.Len() > 0 {
		usageCfg, _ := pf.Usage.Select(m)
		finishCfg, _ := pf.Finish.Select(m)
		agg := dslconfig.NewStreamMetricsAggregator(m, usageCfg, finishCfg)
		agg.OnSSETail(usageTail.Bytes())
		u, _, fr, ok := agg.Result()
		if ok && u != nil {
			upstreamUsage = u
		}
		finishReason = strings.TrimSpace(fr)
	}

	out := usageestimate.Estimate(c.UsageEst, usageestimate.Input{
		API:           api,
		Model:         model,
		UpstreamUsage: upstreamUsage,
		RequestBody:   reqBody,
		StreamTail:    usageTail.Bytes(),
	})
	usage := usageMap(out.Usage)
	return &Result{
		Provider:       provider,
		ProviderKey:    key.Name,
		ProviderSource: "dsl",
		API:            api,
		Stream:         true,
		Model:          model,
		Status:         resp.StatusCode,
		LatencyMs:      time.Since(start).Milliseconds(),
		Usage:          usage,
		UsageStage:     out.Stage,
		FinishReason:   finishReason,
	}, nil
}

func (c *Client) buildProxyCtx(gc *gin.Context, provider string, key ProviderKey, api string, stream bool) (*proxyCtx, error) {
	start := time.Now()
	pf, ok := c.Registry.GetProvider(provider)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", provider)
	}

	bodyBytes, root, model, err := readRequestJSON(gc)
	if err != nil {
		return nil, err
	}

	m := &dslmeta.Meta{
		API:             strings.TrimSpace(api),
		IsStream:        stream,
		ActualModelName: strings.TrimSpace(model),
		APIKey:          strings.TrimSpace(key.Value),
		BaseURL:         strings.TrimSpace(key.BaseURLOverride),
		RequestURLPath:  gc.Request.URL.RequestURI(),
		StartTime:       time.Now(),
	}

	if err := pf.Routing.Apply(m); err != nil {
		return nil, err
	}
	if !pf.Routing.HasMatch(m) {
		return nil, fmt.Errorf("dsl provider no match (provider=%s api=%s stream=%v)", provider, api, stream)
	}

	respDir, _ := pf.Response.Select(m)

	reqTransform, hasReqTransform, root, err := applyRequestTransform(pf, m, root)
	if err != nil {
		return nil, err
	}
	applyGeminiModelRewrite(api, m)

	reqBody, err := marshalMaybeJSON(bodyBytes, root)
	if err != nil {
		return nil, err
	}
	reqBody, err = applyReqMap(gc, reqTransform, hasReqTransform, reqBody)
	if err != nil {
		return nil, err
	}

	return &proxyCtx{
		start:           start,
		provider:        provider,
		key:             key,
		api:             api,
		stream:          stream,
		pf:              pf,
		meta:            m,
		model:           model,
		reqBody:         reqBody,
		respDir:         respDir,
		reqTransform:    reqTransform,
		hasReqTransform: hasReqTransform,
	}, nil
}

func readRequestJSON(gc *gin.Context) (bodyBytes []byte, root map[string]any, model string, err error) {
	bodyBytes, err = io.ReadAll(gc.Request.Body)
	if err != nil {
		return nil, nil, "", err
	}
	_ = gc.Request.Body.Close()

	var reqObj any
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &reqObj); err != nil {
			return bodyBytes, nil, "", fmt.Errorf("invalid json: %w", err)
		}
	}
	root, _ = reqObj.(map[string]any)

	if root != nil {
		if v, ok := root["model"].(string); ok {
			model = strings.TrimSpace(v)
		}
	}
	if strings.TrimSpace(model) == "" {
		if m2, ok := parseGeminiModelFromPath(gc.Request.URL.Path); ok && strings.TrimSpace(m2) != "" {
			model = strings.TrimSpace(m2)
		}
	}
	return bodyBytes, root, model, nil
}

func applyRequestTransform(pf dslconfig.ProviderFile, meta *dslmeta.Meta, root map[string]any) (dslconfig.RequestTransform, bool, map[string]any, error) {
	t, ok := pf.Request.Select(meta)
	if !ok {
		return dslconfig.RequestTransform{}, false, root, nil
	}

	t.Apply(meta)

	if root != nil && meta.DSLModelMapped != "" {
		// Only override when the field exists (OpenAI-style). Gemini native requests do not have "model" in body.
		if _, exists := root["model"]; exists {
			root["model"] = meta.DSLModelMapped
		}
	}
	if root != nil && len(t.JSONOps) > 0 {
		out, err := dslconfig.ApplyJSONOps(meta, root, t.JSONOps)
		if err != nil {
			return dslconfig.RequestTransform{}, false, root, err
		}
		root, _ = out.(map[string]any)
	}
	return t, true, root, nil
}

func applyGeminiModelRewrite(api string, meta *dslmeta.Meta) {
	if meta == nil {
		return
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(api)), "gemini.") {
		return
	}
	if strings.TrimSpace(meta.DSLModelMapped) == "" {
		return
	}
	if newPath, ok := replaceGeminiModelInPath(meta.RequestURLPath, meta.DSLModelMapped); ok {
		meta.RequestURLPath = newPath
	}
}

func marshalMaybeJSON(bodyBytes []byte, root map[string]any) ([]byte, error) {
	if root == nil {
		return bodyBytes, nil
	}
	b, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func applyReqMap(gc *gin.Context, t dslconfig.RequestTransform, hasT bool, reqBody []byte) ([]byte, error) {
	if !hasT || strings.TrimSpace(t.ReqMapMode) == "" {
		return reqBody, nil
	}
	ce := strings.ToLower(strings.TrimSpace(gc.GetHeader("Content-Encoding")))
	if ce != "" && ce != contentEncodingIdentity {
		return nil, fmt.Errorf("cannot transform encoded client request (Content-Encoding=%q)", gc.GetHeader("Content-Encoding"))
	}
	switch strings.ToLower(strings.TrimSpace(t.ReqMapMode)) {
	case "openai_chat_to_openai_responses":
		return dslconfig.MapOpenAIChatCompletionsToResponsesRequest(reqBody)
	default:
		return nil, fmt.Errorf("unsupported req_map mode %q", t.ReqMapMode)
	}
}

func (c *Client) doUpstreamRequest(gc *gin.Context, provider string, pf dslconfig.ProviderFile, m *dslmeta.Meta, reqBody []byte) (*http.Response, error) {
	if m == nil {
		return nil, errors.New("meta is nil")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("upstream base_url is empty")
	}
	upstreamURL := baseURL + m.RequestURLPath

	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, gc.Request.Method, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	pf.Headers.Apply(m, req.Header)

	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, req.Method, upstreamURL, req.Header, limited, false, truncated)
	}

	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	return httpc.Do(req)
}

func isEffectiveStream(clientStream bool, resp *http.Response) bool {
	if clientStream {
		return true
	}
	if resp == nil {
		return false
	}
	upstreamCT := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(upstreamCT, "text/event-stream")
}

func maybeDecodeUpstreamBody(body []byte, contentEncoding string) ([]byte, error) {
	ce := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch ce {
	case "", contentEncodingIdentity:
		return nil, nil
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = gr.Close()
		}()
		decoded, err := io.ReadAll(gr)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("cannot decode upstream response (Content-Encoding=%q)", contentEncoding)
	}
}

func (c *Client) httpClientForProvider(provider string) (*http.Client, error) {
	if c == nil {
		return http.DefaultClient, nil
	}
	// default client
	base := c.HTTP
	if base == nil {
		base = http.DefaultClient
	}

	raw := ""
	if c.ProxyByProvider != nil {
		raw = strings.TrimSpace(c.ProxyByProvider[strings.ToLower(strings.TrimSpace(provider))])
	}
	if raw == "" {
		return base, nil
	}

	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid upstream proxy url for provider=%s: %q", strings.TrimSpace(provider), raw)
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))

	// Cache per proxy URL to preserve connection pooling.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.httpByProxy == nil {
		c.httpByProxy = map[string]*http.Client{}
	}
	if hc, ok := c.httpByProxy[u.String()]; ok && hc != nil {
		return hc, nil
	}

	// Clone transport and customize proxy/dialer.
	var rt *http.Transport
	if bt, ok := base.Transport.(*http.Transport); ok && bt != nil {
		rt = bt.Clone()
	} else if dt, ok := http.DefaultTransport.(*http.Transport); ok && dt != nil {
		rt = dt.Clone()
	} else {
		rt = (&http.Transport{}).Clone()
	}

	switch scheme {
	case "http", "https":
		rt.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		var auth *xproxy.Auth
		if u.User != nil {
			user := strings.TrimSpace(u.User.Username())
			pass, _ := u.User.Password()
			if user != "" {
				auth = &xproxy.Auth{User: user, Password: pass}
			}
		}
		d, err := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("init socks5 dialer for provider=%s: %w", strings.TrimSpace(provider), err)
		}
		// Ensure we don't accidentally pick up ProxyFromEnvironment from cloned transports.
		rt.Proxy = nil
		if cd, ok := d.(xproxy.ContextDialer); ok {
			rt.DialContext = cd.DialContext
		} else {
			rt.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return d.Dial(network, addr)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported upstream proxy scheme for provider=%s: %q", strings.TrimSpace(provider), u.Scheme)
	}

	hc := &http.Client{
		Timeout:   base.Timeout,
		Transport: rt,
	}
	c.httpByProxy[u.String()] = hc
	return hc, nil
}

func parseGeminiModelFromPath(path string) (model string, ok bool) {
	p := strings.TrimSpace(path)
	// /v1beta/models/{model}:{action}
	const prefix = "/v1beta/models/"
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(p, prefix)
	// rest: {model}:{action}
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	model = strings.TrimSpace(parts[0])
	if model == "" {
		return "", false
	}
	return model, true
}

func replaceGeminiModelInPath(pathWithQuery string, newModel string) (string, bool) {
	p := strings.TrimSpace(pathWithQuery)
	const prefix = "/v1beta/models/"
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(p, prefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", false
	}
	return prefix + strings.TrimSpace(newModel) + ":" + parts[1], true
}

func usageMap(u *dslconfig.Usage) map[string]any {
	if u == nil {
		return nil
	}
	// normalize totals for callers
	out := usageestimate.Estimate(nil, usageestimate.Input{UpstreamUsage: u})
	u = out.Usage
	if u == nil {
		return nil
	}
	m := map[string]any{
		"input_tokens":  u.InputTokens,
		"output_tokens": u.OutputTokens,
		"total_tokens":  u.TotalTokens,
	}
	if u.InputTokenDetails != nil {
		m["cache_read_tokens"] = u.InputTokenDetails.CachedTokens
		m["cache_write_tokens"] = u.InputTokenDetails.CacheWriteTokens
	}
	return m
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remain := b.limit - b.buf.Len()
	if remain <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remain {
		_, _ = b.buf.Write(p[:remain])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte   { return b.buf.Bytes() }
func (b *limitedBuffer) Truncated() bool { return b.truncated }

// tailBuffer keeps the last N bytes written.
type tailBuffer struct {
	limit int
	buf   []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	if len(p) >= b.limit {
		b.buf = append(b.buf[:0], p[len(p)-b.limit:]...)
		return len(p), nil
	}
	if len(b.buf)+len(p) <= b.limit {
		b.buf = append(b.buf, p...)
		return len(p), nil
	}
	needDrop := len(b.buf) + len(p) - b.limit
	b.buf = append(b.buf[needDrop:], p...)
	return len(p), nil
}

func (b *tailBuffer) Bytes() []byte { return b.buf }
func (b *tailBuffer) Len() int      { return len(b.buf) }
