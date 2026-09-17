package proxy

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
)

func (c *Client) ProxyJSON(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	stream bool,
) (*Result, error) {
	for attempt := 0; attempt < 2; attempt++ {
		res, err, retry := c.proxyJSAttempt(gc, provider, key, api, stream, attempt == 0)
		if !retry {
			return res, err
		}
	}
	panic("unreachable attempt limit")
}

func (c *Client) proxyJSAttempt(gc *gin.Context, provider string, key ProviderKey, api string, stream bool, allowRetry bool) (result *Result, err error, retry bool) {
	gc.Set(jsSessionKey, (*jsext.Session)(nil))
	gc.Set("onr.js.headers", nil)
	gc.Set("onr.js.final_headers", nil)
	defer func() { c.finishJS(gc, result, err) }()
	bctx, err := c.buildProxyCtx(gc, provider, key, api, stream)
	if err != nil {
		return nil, err, false
	}
	start := bctx.start
	pf := bctx.pf
	m := bctx.meta
	model := bctx.model
	reqBody := bctx.reqBody
	respDir := bctx.respDir

	resp, cancelUpstream, err := c.doUpstreamRequest(gc, provider, &pf, m, reqBody, bctx.reqContentType)
	if err != nil {
		return nil, err, false
	}
	defer cancelUpstream()
	defer func() {
		_ = resp.Body.Close()
	}()

	if allowRetry && jsSession(gc) != nil && resp.StatusCode == http.StatusUnauthorized && strings.TrimSpace(m.OAuthCacheKey) != "" {
		c.invalidateOAuthCache(m.OAuthCacheKey)
		return &Result{Provider: provider, Status: resp.StatusCode}, nil, true
	}
	// If upstream returns SSE, treat it as streaming regardless of client "stream" flag.
	effectiveStream := isEffectiveStream(stream, resp, respDir)
	if jsSession(gc) != nil && !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		effectiveStream = false
	}

	if !effectiveStream {
		result, err = c.handleNonStreamResponse(gc, provider, key, api, stream, start, pf, m, model, reqBody, respDir, resp)
		return result, err, false
	}
	result, err = c.handleStreamResponse(gc, provider, key, api, start, pf, m, model, reqBody, respDir, resp)
	return result, err, false
}
