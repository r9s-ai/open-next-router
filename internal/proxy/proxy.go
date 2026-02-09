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

	"github.com/r9s-ai/open-next-router/internal/auth"
	"github.com/r9s-ai/open-next-router/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/pkg/trafficdump"
	"github.com/r9s-ai/open-next-router/pkg/usageestimate"
)

const (
	contentEncodingIdentity = "identity"
	contentEncodingGzip     = "gzip"
	contentTypeJSON         = "application/json"
)

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

	resp, cancelUpstream, err := c.doUpstreamRequest(gc, provider, pf, m, reqBody)
	if err != nil {
		return nil, err
	}
	defer cancelUpstream()
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

	respOutBody, outCT, didTransform, err := mapNonStreamResponse(respBody, resp, respDir)
	if err != nil {
		return nil, err
	}

	// metrics are extracted from the response after response mapping (resp_map),
	// but before response json ops (json_del/json_set/json_rename) so operators can strip fields
	// from downstream without losing upstream usage/finish_reason signals.
	metricsBody := respOutBody

	usage, usageStage := estimateNonStreamUsage(c.UsageEst, pf, m, api, model, reqBody, metricsBody)
	finishReason := extractNonStreamFinishReason(pf, m, metricsBody)

	respOutBody, outCT, didTransform, err = applyNonStreamResponseJSONOps(respOutBody, outCT, resp, m, respDir.JSONOps, didTransform)
	if err != nil {
		return nil, err
	}

	copyHeadersToClient(gc, resp.Header, didTransform)
	if strings.TrimSpace(outCT) != "" {
		gc.Writer.Header().Set("Content-Type", outCT)
	}

	gc.Status(resp.StatusCode)
	if _, err := gc.Writer.Write(respOutBody); err != nil {
		return nil, err
	}

	// traffic dump: proxy response (final downstream bytes)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		ct := strings.ToLower(outCT)
		binary := !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
		limited, truncated := trafficdump.LimitBytes(respOutBody, rec.MaxBytes())
		trafficdump.AppendProxyResponse(gc, limited, binary, truncated, resp.StatusCode)
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

func mapNonStreamResponse(respBody []byte, resp *http.Response, respDir dslconfig.ResponseDirective) ([]byte, string, bool, error) {
	respOutBody := respBody
	outCT := ""
	if resp != nil {
		outCT = resp.Header.Get("Content-Type")
	}
	didTransform := false

	if strings.TrimSpace(respDir.Op) != "resp_map" {
		return respOutBody, outCT, didTransform, nil
	}

	decoded, err := maybeDecodeUpstreamBody(respBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, "", false, err
	}
	srcBody := respBody
	if decoded != nil {
		srcBody = decoded
	}
	switch strings.ToLower(strings.TrimSpace(respDir.Mode)) {
	case "openai_responses_to_openai_chat":
		respOutBody, err = apitransform.MapOpenAIResponsesToChatCompletions(srcBody)
		if err != nil {
			return nil, "", false, err
		}
		return respOutBody, contentTypeJSON, true, nil
	case "openai_to_anthropic_messages":
		respOutBody, err = apitransform.MapOpenAIChatCompletionsToClaudeMessagesResponse(srcBody)
		if err != nil {
			return nil, "", false, err
		}
		return respOutBody, contentTypeJSON, true, nil
	case "openai_to_gemini_chat", "openai_to_gemini_generate_content":
		respOutBody, err = apitransform.MapOpenAIChatCompletionsToGeminiGenerateContentResponse(srcBody)
		if err != nil {
			return nil, "", false, err
		}
		return respOutBody, contentTypeJSON, true, nil
	default:
		return respOutBody, outCT, didTransform, nil
	}
}

func estimateNonStreamUsage(
	estCfg *usageestimate.Config,
	pf dslconfig.ProviderFile,
	meta *dslmeta.Meta,
	api string,
	model string,
	reqBody []byte,
	metricsBody []byte,
) (usage map[string]any, usageStage string) {
	if cfg, ok := pf.Usage.Select(meta); ok {
		u, _, err := dslconfig.ExtractUsage(meta, cfg, metricsBody)
		if err == nil && u != nil {
			out := usageestimate.Estimate(estCfg, usageestimate.Input{
				API:           api,
				Model:         model,
				UpstreamUsage: u,
				RequestBody:   reqBody,
				ResponseBody:  metricsBody,
			})
			return usageMap(out.Usage), out.Stage
		}
	}
	out := usageestimate.Estimate(estCfg, usageestimate.Input{
		API:          api,
		Model:        model,
		RequestBody:  reqBody,
		ResponseBody: metricsBody,
	})
	return usageMap(out.Usage), out.Stage
}

func extractNonStreamFinishReason(pf dslconfig.ProviderFile, meta *dslmeta.Meta, metricsBody []byte) string {
	finishReason := ""
	if frCfg, ok := pf.Finish.Select(meta); ok {
		if v, err := dslconfig.ExtractFinishReason(meta, frCfg, metricsBody); err == nil {
			finishReason = strings.TrimSpace(v)
		}
	}
	return finishReason
}

func applyNonStreamResponseJSONOps(
	respOutBody []byte,
	outCT string,
	resp *http.Response,
	meta *dslmeta.Meta,
	ops []dslconfig.JSONOp,
	didTransform bool,
) ([]byte, string, bool, error) {
	if len(ops) == 0 || resp == nil {
		return respOutBody, outCT, didTransform, nil
	}

	ctLower := strings.ToLower(strings.TrimSpace(outCT))
	ceLower := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	trim := bytes.TrimSpace(respOutBody)
	looksJSON := strings.Contains(ctLower, "json") || (len(trim) > 0 && trim[0] == '{')
	if strings.Contains(ctLower, "json") && ceLower == contentEncodingGzip {
		looksJSON = true
	}
	if !looksJSON {
		return respOutBody, outCT, didTransform, nil
	}

	bodyForOps := respOutBody
	var root any
	if err := json.Unmarshal(bodyForOps, &root); err != nil {
		decoded, derr := maybeDecodeUpstreamBody(respOutBody, resp.Header.Get("Content-Encoding"))
		if derr != nil {
			return nil, "", false, derr
		}
		if decoded == nil {
			return nil, "", false, fmt.Errorf("invalid json response for response json ops: %w", err)
		}
		bodyForOps = decoded
		if err := json.Unmarshal(bodyForOps, &root); err != nil {
			return nil, "", false, fmt.Errorf("invalid json response for response json ops: %w", err)
		}
		didTransform = true
	}

	obj, ok := root.(map[string]any)
	if !ok || obj == nil {
		return respOutBody, outCT, didTransform, nil
	}
	outAny, err := dslconfig.ApplyJSONOps(meta, obj, ops)
	if err != nil {
		return nil, "", false, err
	}
	outBytes, err := json.Marshal(outAny)
	if err != nil {
		return nil, "", false, err
	}
	respOutBody = outBytes
	if strings.TrimSpace(outCT) == "" || !strings.Contains(ctLower, "json") {
		outCT = "application/json"
	}
	return respOutBody, outCT, true, nil
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
	// copy headers
	copyHeadersToClient(gc, resp.Header, false)

	// Always keep a tail buffer for best-effort usage extraction from SSE.
	tailLimit := 256 << 10 // 256KB
	if c.UsageEst != nil && c.UsageEst.MaxStreamCollectBytes > 0 {
		tailLimit = c.UsageEst.MaxStreamCollectBytes
	}
	usageTail := &tailBuffer{limit: tailLimit}

	dump := newStreamDumpState(gc)
	defer dump.Append(gc, resp)

	n, err := streamToDownstream(gc, m, respDir, resp, usageTail, dump)
	ignoredDisconnect := isClientDisconnectErr(err)
	dump.SetStreamResult(n, err, ignoredDisconnect)
	if err != nil && !ignoredDisconnect {
		return nil, err
	}
	if f, ok := gc.Writer.(http.Flusher); ok {
		f.Flush()
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
	if strings.TrimSpace(model) == "" {
		if v, ok := gc.Get("onr.model"); ok {
			model = strings.TrimSpace(fmt.Sprintf("%v", v))
		}
	}
	if mo := auth.TokenModelOverride(gc); mo != "" {
		model = mo
		if root != nil {
			if _, exists := root["model"]; exists {
				root["model"] = mo
			}
		}
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
	if mo := strings.TrimSpace(model); mo != "" {
		if newPath, ok := replaceGeminiModelInPath(m.RequestURLPath, mo); ok {
			m.RequestURLPath = newPath
		}
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
		return apitransform.MapOpenAIChatCompletionsToResponsesRequest(reqBody)
	case "anthropic_to_openai_chat":
		return apitransform.MapClaudeMessagesToOpenAIChatCompletions(reqBody)
	case "gemini_to_openai_chat":
		return apitransform.MapGeminiGenerateContentToOpenAIChatCompletions(reqBody)
	default:
		return nil, fmt.Errorf("unsupported req_map mode %q", t.ReqMapMode)
	}
}

func (c *Client) doUpstreamRequest(gc *gin.Context, provider string, pf dslconfig.ProviderFile, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	if m == nil {
		return nil, func() {}, errors.New("meta is nil")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/")
	if baseURL == "" {
		return nil, func() {}, errors.New("upstream base_url is empty")
	}
	upstreamURL := baseURL + m.RequestURLPath

	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	req, err := http.NewRequestWithContext(reqCtx, gc.Request.Method, upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		cancel()
		return nil, func() {}, err
	}

	req.Header.Set("Content-Type", "application/json")
	pf.Headers.Apply(m, req.Header)

	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, req.Method, upstreamURL, req.Header, limited, false, truncated)
	}

	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	return resp, cancel, nil
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
	case contentEncodingGzip:
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
