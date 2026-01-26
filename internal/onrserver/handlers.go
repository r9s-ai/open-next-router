package onrserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/edgefn/open-next-router/internal/config"
	"github.com/edgefn/open-next-router/internal/proxy"
	"github.com/edgefn/open-next-router/internal/requestid"
	"github.com/edgefn/open-next-router/internal/trafficdump"
)

func makeHandler(cfg *config.Config, st *state, pclient *proxy.Client, api string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("onr.api", api)
		bodyBytes, stream, model, err := peekJSONBody(c)
		if err != nil {
			writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "invalid_json", err.Error())
			return
		}

		if rec := trafficdump.FromContext(c); rec != nil && rec.MaxBytes() > 0 {
			ct := ""
			if c.Request != nil {
				ct = c.Request.Header.Get("Content-Type")
			}
			lct := strings.ToLower(ct)
			binary := !strings.Contains(lct, "json") && !strings.HasPrefix(lct, "text/")
			limited, truncated := trafficdump.LimitBytes(bodyBytes, rec.MaxBytes())
			trafficdump.AppendOriginRequest(c, limited, binary, truncated)
		}

		// restore body for downstream proxy layer
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		provider, source := selectProvider(st, c.GetHeader("x-onr-provider"), model)
		c.Set("onr.provider", provider)
		c.Set("onr.provider_source", source)
		c.Set("onr.model", model)
		c.Set("onr.stream", stream)
		if provider == "" {
			writeOpenAIError(
				c,
				http.StatusBadRequest,
				"invalid_request_error",
				"provider_not_selected",
				"no provider selected: set x-onr-provider or configure models.yaml",
			)
			return
		}

		keys := st.Keys()
		k, ok := keys.NextKey(provider)
		if !ok {
			writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "missing_upstream_key", "no upstream key for provider: "+provider)
			return
		}

		res, perr := pclient.ProxyJSON(c, provider, proxy.ProviderKey{
			Name:            k.Name,
			Value:           k.Value,
			BaseURLOverride: k.BaseURLOverride,
		}, api, stream)
		if perr != nil {
			writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "proxy_error", perr.Error())
			return
		}
		if res != nil {
			c.Set("onr.latency_ms", res.LatencyMs)
		}

		_ = cfg
	}
}

func selectProvider(st *state, headerProvider string, model string) (provider string, source string) {
	if p := strings.ToLower(strings.TrimSpace(headerProvider)); p != "" {
		return p, "header"
	}
	if m := strings.TrimSpace(model); m != "" {
		if mr := st.ModelRouter(); mr != nil {
			if p, ok := mr.NextProvider(m); ok && p != "" {
				return p, "model"
			}
		}
	}
	return "", ""
}

func peekJSONBody(c *gin.Context) ([]byte, bool, string, error) {
	b, err := ioReadAllLimit(c.Request.Body, 16<<20) // 16MB
	if err != nil {
		return nil, false, "", err
	}

	if len(bytes.TrimSpace(b)) == 0 {
		return b, false, "", nil
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return b, false, "", err
	}
	model, _ := obj["model"].(string)
	stream := false
	if v, ok := obj["stream"].(bool); ok {
		stream = v
	}
	return b, stream, strings.TrimSpace(model), nil
}

func writeOpenAIError(c *gin.Context, status int, typ, code, msg string) {
	if c != nil {
		if rid := strings.TrimSpace(c.GetString(requestid.HeaderKey)); rid != "" {
			msg = msg + " (request id: " + rid + ")"
		}
	}
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    typ,
			"code":    code,
		},
	})
}

func ioReadAllLimit(rc io.ReadCloser, limit int64) ([]byte, error) {
	defer func() { _ = rc.Close() }()
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, rc, limit+1); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if int64(buf.Len()) > limit {
		return nil, errors.New("request body too large")
	}
	return buf.Bytes(), nil
}
