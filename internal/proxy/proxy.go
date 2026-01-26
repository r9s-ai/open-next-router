package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/edgefn/open-next-router/internal/trafficdump"
	"github.com/edgefn/open-next-router/pkg/dslconfig"
	"github.com/edgefn/open-next-router/pkg/dslmeta"
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
}

type Client struct {
	HTTP         *http.Client
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Registry     *dslconfig.Registry
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
			root["model"] = m.DSLModelMapped
		}
		if root != nil && len(t.JSONOps) > 0 {
			out, err := dslconfig.ApplyJSONOps(m, root, t.JSONOps)
			if err != nil {
				return nil, err
			}
			root, _ = out.(map[string]any)
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

	resp, err := c.HTTP.Do(req)
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
				usage = map[string]any{
					"input_tokens":  u.InputTokens,
					"output_tokens": u.OutputTokens,
					"total_tokens":  u.TotalTokens,
				}
				if u.InputTokenDetails != nil {
					usage["cache_read_tokens"] = u.InputTokenDetails.CachedTokens
					usage["cache_write_tokens"] = u.InputTokenDetails.CacheWriteTokens
				}
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
		}, nil
	}

	// stream passthrough
	gc.Status(resp.StatusCode)
	var dumpBuf []byte
	var dumpTruncated bool
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		buf := &limitedBuffer{limit: rec.MaxBytes()}
		tee := io.TeeReader(resp.Body, buf)
		if _, err := io.Copy(gc.Writer, tee); err != nil {
			return nil, err
		}
		dumpBuf = buf.Bytes()
		dumpTruncated = buf.Truncated()
	} else {
		if _, err := io.Copy(gc.Writer, resp.Body); err != nil {
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
	return &Result{
		Provider:       provider,
		ProviderKey:    key.Name,
		ProviderSource: "dsl",
		API:            api,
		Stream:         true,
		Model:          model,
		Status:         resp.StatusCode,
		LatencyMs:      time.Since(start).Milliseconds(),
	}, nil
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
