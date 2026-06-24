package proxy

import "github.com/gin-gonic/gin"

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

	resp, cancelUpstream, err := c.doUpstreamRequest(gc, provider, &pf, m, reqBody)
	if err != nil {
		return nil, err
	}
	defer cancelUpstream()
	defer func() {
		_ = resp.Body.Close()
	}()

	// If upstream returns SSE, treat it as streaming regardless of client "stream" flag.
	effectiveStream := isEffectiveStream(stream, resp, respDir)

	if !effectiveStream {
		return c.handleNonStreamResponse(gc, provider, key, api, stream, start, pf, m, model, reqBody, respDir, resp)
	}
	return c.handleStreamResponse(gc, provider, key, api, start, pf, m, model, reqBody, respDir, resp)
}
