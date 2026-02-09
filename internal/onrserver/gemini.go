package onrserver

import (
	"bytes"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/internal/auth"
	"github.com/r9s-ai/open-next-router/internal/config"
	"github.com/r9s-ai/open-next-router/internal/proxy"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

func makeGeminiHandler(cfg *config.Config, st *state, pclient *proxy.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, action, err := parseGeminiModelAction(c.Param("path"))
		if err != nil {
			writeOpenAIError(c, "invalid_path", err.Error())
			return
		}
		if mo := auth.TokenModelOverride(c); mo != "" {
			model = mo
		}

		api, stream, ok := geminiAPIFromAction(action)
		if !ok {
			writeOpenAIError(
				c,
				"unsupported_gemini_action",
				"unsupported gemini action: "+action,
			)
			return
		}

		// Gemini native streaming requires `alt=sse`.
		// If the client doesn't provide it, add it by default for better ergonomics.
		if stream && c.Request != nil && c.Request.URL != nil {
			q := c.Request.URL.Query()
			if strings.TrimSpace(q.Get("alt")) == "" {
				q.Set("alt", "sse")
				c.Request.URL.RawQuery = q.Encode()
			}
		}

		c.Set("onr.api", api)
		c.Set("onr.model", model)
		c.Set("onr.stream", stream)

		bodyBytes, _, _, err := peekJSONBody(c)
		if err != nil {
			writeOpenAIError(c, "invalid_json", err.Error())
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

		provider, source := selectProvider(st, auth.TokenProvider(c), c.GetHeader("x-onr-provider"), model)
		c.Set("onr.provider", provider)
		c.Set("onr.provider_source", source)
		if provider == "" {
			writeOpenAIError(
				c,
				"provider_not_selected",
				"no provider selected: set x-onr-provider or configure models.yaml",
			)
			return
		}

		kname := ""
		kval := ""
		kbase := ""
		if uk := auth.TokenUpstreamKey(c); uk != "" {
			kname = "byok"
			kval = uk
		} else {
			keys := st.Keys()
			k, ok := keys.NextKey(provider)
			if !ok {
				writeOpenAIError(c, "missing_upstream_key", "no upstream key for provider: "+provider)
				return
			}
			kname = k.Name
			kval = k.Value
			kbase = k.BaseURLOverride
		}

		res, perr := pclient.ProxyJSON(c, provider, proxy.ProviderKey{
			Name:            kname,
			Value:           kval,
			BaseURLOverride: kbase,
		}, api, stream)
		if perr != nil {
			writeOpenAIError(c, "proxy_error", perr.Error())
			return
		}
		setProxyResultContext(c, res)

		_ = cfg
	}
}

func parseGeminiModelAction(pathParam string) (model string, action string, err error) {
	p := strings.TrimSpace(pathParam)
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", "", errString("missing gemini path")
	}
	// Expect: {model}:{action} (e.g. gemini-2.0-flash:generateContent)
	parts := strings.SplitN(p, ":", 2)
	if len(parts) != 2 {
		return "", "", errString("invalid gemini path, expected /models/{model}:{action}")
	}
	model = strings.TrimSpace(parts[0])
	action = strings.TrimSpace(parts[1])
	if model == "" || action == "" {
		return "", "", errString("invalid gemini path, expected /models/{model}:{action}")
	}
	return model, action, nil
}

func geminiAPIFromAction(action string) (api string, stream bool, ok bool) {
	a := strings.ToLower(strings.TrimSpace(action))
	switch {
	case strings.HasPrefix(a, "generatecontent"):
		return "gemini.generateContent", false, true
	case strings.HasPrefix(a, "streamgeneratecontent"):
		return "gemini.streamGenerateContent", true, true
	default:
		return "", false, false
	}
}

type errString string

func (e errString) Error() string { return string(e) }
