package proxy

import (
	"bytes"
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

func (c *Client) ProxyJSON(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	stream bool,
) (*Result, error) {
	start := time.Now()
	pf, ok := c.Registry.GetProvider(provider)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", provider)
	}

	// read request json
	bodyBytes, err := io.ReadAll(gc.Request.Body)
	if err != nil {
		return nil, err
	}
	_ = gc.Request.Body.Close()

	var reqObj any
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &reqObj); err != nil {
			return nil, fmt.Errorf("invalid json: %w", err)
		}
	}
	root, _ := reqObj.(map[string]any)

	model := ""
	if root != nil {
		if v, ok := root["model"].(string); ok {
			model = strings.TrimSpace(v)
		}
	}
	if model == "" {
		// Gemini native endpoints put model in URL path: /v1beta/models/{model}:{action}
		if m2, ok := parseGeminiModelFromPath(gc.Request.URL.Path); ok && strings.TrimSpace(m2) != "" {
			model = strings.TrimSpace(m2)
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

	// route rewrite (set_path/set_query/del_query, base_url default)
	if err := pf.Routing.Apply(m); err != nil {
		return nil, err
	}
	if !pf.Routing.HasMatch(m) {
		return nil, fmt.Errorf("dsl provider no match (provider=%s api=%s stream=%v)", provider, api, stream)
	}

	// request transform (model_map + json ops)
	if t, ok := pf.Request.Select(m); ok {
		t.Apply(m)
		if root != nil && m.DSLModelMapped != "" {
			// Only override when the field exists (OpenAI-style). Gemini native requests do not have "model" in body.
			if _, exists := root["model"]; exists {
				root["model"] = m.DSLModelMapped
			}
		}
		if root != nil && len(t.JSONOps) > 0 {
			out, err := dslconfig.ApplyJSONOps(m, root, t.JSONOps)
			if err != nil {
				return nil, err
			}
			root, _ = out.(map[string]any)
		}
	}

	// Best-effort: for Gemini native endpoints, let model_map rewrite URL model segment.
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(api)), "gemini.") && strings.TrimSpace(m.DSLModelMapped) != "" {
		if newPath, ok := replaceGeminiModelInPath(m.RequestURLPath, m.DSLModelMapped); ok {
			m.RequestURLPath = newPath
		}
	}

	// rebuild body
	outBody := bodyBytes
	if root != nil {
		b, err := json.Marshal(root)
		if err != nil {
			return nil, err
		}
		outBody = b
	}

	// build upstream request
	baseURL := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("upstream base_url is empty")
	}
	upstreamURL := baseURL + m.RequestURLPath

	ctx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, gc.Request.Method, upstreamURL, bytes.NewReader(outBody))
	if err != nil {
		return nil, err
	}

	// headers: start clean, do not forward client Authorization.
	req.Header.Set("Content-Type", "application/json")
	// apply auth + request headers from provider conf
	pf.Headers.Apply(m, req.Header)

	// traffic dump: upstream request
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(outBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, req.Method, upstreamURL, req.Header, limited, false, truncated)
	}

	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// copy headers
	for k, vs := range resp.Header {
		if len(vs) == 0 {
			continue
		}
		gc.Writer.Header().Set(k, vs[0])
	}

	// non-stream: read body, try extract usage, write
	var usage map[string]any
	if !stream && !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
			ct := strings.ToLower(resp.Header.Get("Content-Type"))
			binary := !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
			limited, truncated := trafficdump.LimitBytes(respBody, rec.MaxBytes())
			trafficdump.AppendUpstreamResponse(gc, resp.Status, resp.Header, limited, binary, truncated)
		}

		gc.Status(resp.StatusCode)
		if _, err := gc.Writer.Write(respBody); err != nil {
			return nil, err
		}

		if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
			ct := strings.ToLower(resp.Header.Get("Content-Type"))
			binary := !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
			limited, truncated := trafficdump.LimitBytes(respBody, rec.MaxBytes())
			trafficdump.AppendProxyResponse(gc, limited, binary, truncated, resp.StatusCode)
		}

		if cfg, ok := pf.Usage.Select(m); ok {
			u, _, err := dslconfig.ExtractUsage(m, cfg, respBody)
			if err == nil && u != nil {
				out := usageestimate.Estimate(c.UsageEst, usageestimate.Input{
					API:           api,
					Model:         model,
					UpstreamUsage: u,
					RequestBody:   outBody,
					ResponseBody:  respBody,
				})
				usage = usageMap(out.Usage)
				usageStage := out.Stage

				finishReason := ""
				if frCfg, ok := pf.Finish.Select(m); ok {
					if v, err := dslconfig.ExtractFinishReason(m, frCfg, respBody); err == nil {
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
		}

		out := usageestimate.Estimate(c.UsageEst, usageestimate.Input{
			API:          api,
			Model:        model,
			RequestBody:  outBody,
			ResponseBody: respBody,
		})
		usage = usageMap(out.Usage)

		finishReason := ""
		if frCfg, ok := pf.Finish.Select(m); ok {
			if v, err := dslconfig.ExtractFinishReason(m, frCfg, respBody); err == nil {
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
			UsageStage:     out.Stage,
			FinishReason:   finishReason,
		}, nil
	}

	// stream passthrough
	gc.Status(resp.StatusCode)
	var (
		dumpBuf       []byte
		dumpTruncated bool
	)

	// Always keep a tail buffer for best-effort usage extraction from SSE.
	// Note: usage is usually sent near the end of the stream when enabled.
	tailLimit := 256 << 10 // 256KB
	if c.UsageEst != nil && c.UsageEst.MaxStreamCollectBytes > 0 {
		tailLimit = c.UsageEst.MaxStreamCollectBytes
	}
	usageTail := &tailBuffer{limit: tailLimit}

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
	if f, ok := gc.Writer.(http.Flusher); ok {
		f.Flush()
	}

	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 && len(dumpBuf) > 0 {
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
		RequestBody:   outBody,
		StreamTail:    usageTail.Bytes(),
	})
	usage = usageMap(out.Usage)
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

func (c *Client) httpClientForProvider(provider string) (*http.Client, error) {
	// default client
	base := c.HTTP
	if base == nil {
		base = http.DefaultClient
	}

	raw := ""
	if c != nil && c.ProxyByProvider != nil {
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
