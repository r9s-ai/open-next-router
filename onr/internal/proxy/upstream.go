package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	xproxy "golang.org/x/net/proxy"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

// doUpstreamRequest requires a non-nil provider file and request meta from buildProxyCtx.
func (c *Client) doUpstreamRequest(gc *gin.Context, provider string, pf *dslconfig.ProviderFile, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	baseURL := m.BaseURL
	if baseURL == "" {
		return nil, func() {}, errors.New("upstream base_url is empty")
	}
	upstreamURL := baseURL + m.RequestURLPath

	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}

	var lastResp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		req, reqErr := http.NewRequestWithContext(reqCtx, gc.Request.Method, upstreamURL, bytes.NewReader(reqBody))
		if reqErr != nil {
			cancel()
			return nil, func() {}, reqErr
		}
		if ct := strings.TrimSpace(gc.Request.Header.Get("Content-Type")); ct != "" {
			req.Header.Set("Content-Type", ct)
		} else {
			req.Header.Set("Content-Type", "application/json")
		}

		if oauthErr := c.prepareOAuthForUpstream(reqCtx, provider, *pf, m); oauthErr != nil {
			cancel()
			return nil, func() {}, oauthErr
		}
		pf.Headers.Apply(m, gc.Request.Header, req.Header)

		if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
			limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
			trafficdump.AppendUpstreamRequest(gc, req.Method, upstreamURL, req.Header, limited, false, truncated)
		}

		resp, doErr := httpc.Do(req)
		if doErr != nil {
			cancel()
			return nil, func() {}, doErr
		}
		lastResp = resp
		if attempt == 0 && resp.StatusCode == http.StatusUnauthorized && strings.TrimSpace(m.OAuthCacheKey) != "" {
			_ = resp.Body.Close()
			c.invalidateOAuthCache(m.OAuthCacheKey)
			m.OAuthAccessToken = ""
			continue
		}
		return resp, cancel, nil
	}
	return lastResp, cancel, nil
}

func isEffectiveStream(clientStream bool, resp *http.Response, respDir *dslconfig.ResponseDirective) bool {
	if clientStream {
		return true
	}
	if shouldCollectSSE(respDir, resp) {
		return false
	}
	upstreamCT := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(upstreamCT, "text/event-stream")
}

// httpClientForProvider selects the upstream HTTP client for one normalized provider name.
// The receiver is expected to be non-nil. A nil base client falls back to http.DefaultClient.
// provider is expected to be pre-normalized without leading or trailing spaces.
// c.ProxyByProvider keys and values are expected to be pre-normalized without leading or trailing spaces.
func (c *Client) httpClientForProvider(provider string) (*http.Client, error) {
	base := c.HTTP
	if base == nil {
		base = http.DefaultClient
	}
	raw := c.ProxyByProvider[strings.ToLower(provider)]
	if raw == "" {
		return base, nil
	}

	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid upstream proxy url for provider=%s: %q", provider, raw)
	}
	scheme := strings.ToLower(u.Scheme)

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
			return nil, fmt.Errorf("init socks5 dialer for provider=%s: %w", provider, err)
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
		return nil, fmt.Errorf("unsupported upstream proxy scheme for provider=%s: %q", provider, u.Scheme)
	}

	hc := &http.Client{
		Timeout:   base.Timeout,
		Transport: rt,
	}
	c.httpByProxy[u.String()] = hc
	return hc, nil
}
