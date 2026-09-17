package jsext

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type HTTPPolicy struct {
	Enabled              bool          `yaml:"enabled"`
	AllowedOrigins       []string      `yaml:"allowed_origins"`
	Timeout              time.Duration `yaml:"timeout"`
	MaxTimeout           time.Duration `yaml:"max_timeout"`
	MaxCallsPerHook      int           `yaml:"max_calls_per_hook"`
	MaxRequestBodyBytes  int64         `yaml:"max_request_body_bytes"`
	MaxResponseBodyBytes int64         `yaml:"max_response_body_bytes"`
}

func (p HTTPPolicy) clone() HTTPPolicy {
	p.AllowedOrigins = append([]string(nil), p.AllowedOrigins...)
	if p.Timeout == 0 {
		p.Timeout = 500 * time.Millisecond
	}
	if p.MaxTimeout == 0 {
		p.MaxTimeout = time.Second
	}
	if p.MaxCallsPerHook == 0 {
		p.MaxCallsPerHook = 3
	}
	if p.MaxRequestBodyBytes == 0 {
		p.MaxRequestBodyBytes = 1 << 20
	}
	if p.MaxResponseBodyBytes == 0 {
		p.MaxResponseBodyBytes = 1 << 20
	}
	return p
}
func (p HTTPPolicy) validate() error {
	if p.Timeout < 0 || p.MaxTimeout < 0 || p.MaxCallsPerHook < 0 || p.MaxRequestBodyBytes < 0 || p.MaxResponseBodyBytes < 0 {
		return fmt.Errorf("HTTP limits must be positive")
	}
	for _, raw := range p.AllowedOrigins {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || strings.ContainsAny(u.Hostname(), "*?%") || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid allowed origin %q", raw)
		}
	}
	return nil
}

type httpRequest struct {
	URL       string              `json:"url"`
	Method    string              `json:"method"`
	Headers   map[string][]string `json:"headers"`
	Body      string              `json:"body"`
	TimeoutMs int64               `json:"timeoutMs"`
}

func (s *Session) httpRequest(r httpRequest) (map[string]any, error) {
	p := s.policy
	if s.stage != Request && s.stage != AfterRequestMap && s.stage != Response {
		return nil, fmt.Errorf("HTTP is not allowed in %s", s.stage)
	}
	if !p.Enabled {
		return nil, fmt.Errorf("HTTP is not authorized")
	}
	s.httpCalls++
	if s.httpCalls > p.MaxCallsPerHook {
		return nil, fmt.Errorf("HTTP call limit exceeded")
	}
	u, err := url.Parse(r.URL)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("invalid HTTP URL")
	}
	origin := canonicalOrigin(u)
	allowed := false
	for _, o := range p.AllowedOrigins {
		allowedURL, _ := url.Parse(o)
		if canonicalOrigin(allowedURL) == origin {
			allowed = true
		}
	}
	if !allowed {
		return nil, fmt.Errorf("HTTP origin is not authorized")
	}
	if int64(len(r.Body)) > p.MaxRequestBodyBytes {
		return nil, fmt.Errorf("HTTP request body exceeds limit")
	}
	timeout := p.Timeout
	if r.TimeoutMs != 0 {
		if r.TimeoutMs < 0 {
			return nil, fmt.Errorf("invalid HTTP timeout")
		}
		timeout = time.Duration(r.TimeoutMs) * time.Millisecond
	}
	if timeout > p.MaxTimeout {
		timeout = p.MaxTimeout
	}
	ctx, cancel := context.WithTimeout(s.hookContext, timeout)
	defer cancel()
	transport := &http.Transport{Proxy: nil, DialContext: publicDialContext, DisableKeepAlives: true, ResponseHeaderTimeout: timeout, MaxResponseHeaderBytes: 64 << 10}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: rejectRedirect}
	return executeHTTPRequest(ctx, p, r, client)
}

func executeHTTPRequest(ctx context.Context, p HTTPPolicy, r httpRequest, client *http.Client) (map[string]any, error) {
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, strings.NewReader(r.Body))
	if err != nil {
		return nil, err
	}
	for k, values := range r.Headers {
		if protectedHeader(k) {
			return nil, fmt.Errorf("protected HTTP header: %s", k)
		}
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, p.MaxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > p.MaxResponseBodyBytes {
		return nil, fmt.Errorf("HTTP response body exceeds limit")
	}
	return map[string]any{"status": resp.StatusCode, "headers": visibleHeaders(resp.Header), "body": string(body)}, nil
}

// Resolve and validate every address, then dial a literal IP. No second DNS
// lookup, environment proxy, redirects or pooled connections can bypass it.
func publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("HTTP destination has no addresses")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, fmt.Errorf("HTTP destination is not public")
		}
	}
	var last error
	for _, ip := range ips {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			remote, parseErr := netip.ParseAddrPort(conn.RemoteAddr().String())
			if parseErr != nil || !publicIP(remote.Addr()) || remote.Addr().Unmap() != ip.Unmap() {
				_ = conn.Close()
				return nil, fmt.Errorf("HTTP connection address failed validation")
			}
			return conn, nil
		}
		last = err
	}
	return nil, last
}
func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/23", "3fff::/20", "64:ff9b::/96", "2002::/16"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

func rejectRedirect(*http.Request, []*http.Request) error {
	return fmt.Errorf("HTTP redirects are forbidden")
}
func canonicalOrigin(u *url.URL) string {
	if u == nil {
		return ""
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}
