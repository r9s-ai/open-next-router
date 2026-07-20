package onrserver

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestid"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestvalidate"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
	"github.com/r9s-ai/open-next-router/onr/internal/auth"
	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

const openAIInvalidRequestType = "invalid_request_error"

const (
	ctxKeyRequestBody        = "onr.request_body"
	ctxKeyRequestRoot        = "onr.request_root"
	ctxKeyRequestModel       = "onr.request_model"
	ctxKeyRequestContentType = "onr.request_content_type"
)

func makeHandler(cfg *config.Config, st *state, pclient *proxy.Client, api string, requestIDHeaderKey string) gin.HandlerFunc {
	requestIDHeaderKey = requestid.ResolveHeaderKey(requestIDHeaderKey)
	return func(c *gin.Context) {
		c.Set("onr.api", api)
		bodyBytes, stream, model, err := inspectRequestBody(c, api)
		if err != nil {
			writeOpenAIError(c, requestIDHeaderKey, "invalid_json", err.Error())
			return
		}
		if mo := auth.TokenModelOverride(c); mo != "" {
			model = mo
		}

		if rec := trafficdump.FromContext(c); rec != nil && rec.MaxBytes() > 0 {
			ct := ""
			if c.Request != nil {
				ct = c.Request.Header.Get("Content-Type")
			}
			limited, truncated := trafficdump.LimitBytes(bodyBytes, rec.MaxBytes())
			binary := trafficdump.IsBinaryPayload(ct, limited)
			trafficdump.AppendOriginRequest(c, limited, binary, truncated)
		}

		// restore body for downstream proxy layer
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		provider, source := selectProvider(st, auth.TokenProvider(c), c.GetHeader("x-onr-provider"), model)
		c.Set("onr.provider", provider)
		c.Set("onr.provider_source", source)
		c.Set("onr.model", model)
		c.Set("onr.stream", stream)
		if provider == "" {
			writeOpenAIError(
				c,
				requestIDHeaderKey,
				"provider_not_selected",
				"no provider selected: set x-onr-provider or configure models.yaml",
			)
			return
		}

		kname := ""
		kval := ""
		kbase := ""
		kcredFile := ""
		klocation := ""
		kawsAK := ""
		kawsSK := ""
		kawsSession := ""
		kawsRegion := ""
		if uk := auth.TokenUpstreamKey(c); uk != "" {
			kname = "byok"
			kval = uk
		} else {
			keys := st.Keys()
			k, ok := keys.NextKey(provider)
			if !ok {
				writeOpenAIError(c, requestIDHeaderKey, "missing_upstream_key", "no upstream key for provider: "+provider)
				return
			}
			kname = k.Name
			kval = k.Value
			kbase = k.BaseURLOverride
			kcredFile = k.CredentialFile
			klocation = k.Location
			kawsAK = k.AWSAccessKeyID
			kawsSK = k.AWSSecretAccessKey
			kawsSession = k.AWSSessionToken
			kawsRegion = k.AWSRegion
		}

		res, perr := pclient.ProxyJSON(c, provider, proxy.ProviderKey{
			Name:               kname,
			Value:              kval,
			BaseURLOverride:    kbase,
			CredentialFile:     kcredFile,
			Location:           klocation,
			AWSAccessKeyID:     kawsAK,
			AWSSecretAccessKey: kawsSK,
			AWSSessionToken:    kawsSession,
			AWSRegion:          kawsRegion,
		}, api, stream)
		if perr != nil {
			writeProxyError(c, requestIDHeaderKey, perr)
			return
		}
		setProxyResultContext(c, res)

		_ = cfg
	}
}

func inspectRequestBody(c *gin.Context, api string) ([]byte, bool, string, error) {
	bodyBytes, err := ioReadAllLimit(c.Request.Body, 16<<20) // 16MB
	if err != nil {
		return nil, false, "", err
	}
	contentType := ""
	if c != nil && c.Request != nil {
		contentType = c.Request.Header.Get("Content-Type")
	}
	info, err := requestcanon.Inspect(bodyBytes, contentType, requestcanon.InspectOptions{
		AllowNonJSON: requestcanon.AllowNonJSONRequestBodyAPI(api),
	})
	if err != nil {
		return bodyBytes, false, "", err
	}
	cacheRequestInspection(c, bodyBytes, info.Root, strings.TrimSpace(info.Model), contentType)
	return bodyBytes, info.Stream, strings.TrimSpace(info.Model), nil
}

func selectProvider(st *state, tokenProvider string, headerProvider string, model string) (provider string, source string) {
	if p := strings.ToLower(strings.TrimSpace(tokenProvider)); p != "" {
		return p, "token"
	}
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
	info, err := requestcanon.Inspect(b, "application/json", requestcanon.InspectOptions{})
	if err != nil {
		return b, false, "", err
	}
	cacheRequestInspection(c, b, info.Root, strings.TrimSpace(info.Model), "application/json")
	return b, info.Stream, strings.TrimSpace(info.Model), nil
}

// cacheRequestInspection requires a non-nil Gin context from the request handler path.
func cacheRequestInspection(c *gin.Context, body []byte, root map[string]any, model, contentType string) {
	c.Set(ctxKeyRequestBody, body)
	c.Set(ctxKeyRequestRoot, root)
	c.Set(ctxKeyRequestModel, model)
	c.Set(ctxKeyRequestContentType, contentType)
}

func writeOpenAIError(c *gin.Context, requestIDHeaderKey string, code, msg string) {
	requestIDHeaderKey = requestid.ResolveHeaderKey(requestIDHeaderKey)
	if c != nil {
		if rid := strings.TrimSpace(c.GetString(requestIDHeaderKey)); rid != "" {
			msg = msg + " (request id: " + rid + ")"
		}
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    openAIInvalidRequestType,
			"code":    code,
		},
	})
}

// writeProxyError maps a ProxyJSON error to a downstream 400 response.
// Request validation failures get a stable code and the failing param path;
// all other proxy errors keep the generic proxy_error code.
func writeProxyError(c *gin.Context, requestIDHeaderKey string, err error) {
	var verr *requestvalidate.RequestValidationError
	if errors.As(err, &verr) {
		writeOpenAIErrorWithParam(c, requestIDHeaderKey, "request_validation_failed", err.Error(), verr.PathOrName)
		return
	}
	writeOpenAIError(c, requestIDHeaderKey, "proxy_error", err.Error())
}

func writeOpenAIErrorWithParam(c *gin.Context, requestIDHeaderKey string, code, msg, param string) {
	requestIDHeaderKey = requestid.ResolveHeaderKey(requestIDHeaderKey)
	if c != nil {
		if rid := strings.TrimSpace(c.GetString(requestIDHeaderKey)); rid != "" {
			msg = msg + " (request id: " + rid + ")"
		}
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    openAIInvalidRequestType,
			"param":   param,
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
