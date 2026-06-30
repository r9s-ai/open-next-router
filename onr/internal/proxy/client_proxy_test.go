package proxy

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestHTTPClientForProvider_UsesProxy(t *testing.T) {
	c := &Client{
		HTTP: &http.Client{Timeout: 3 * time.Second},
		ProxyByProvider: map[string]string{
			"openai": "http://127.0.0.1:7890",
		},
	}

	hc, err := c.httpClientForProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc == nil || hc.Transport == nil {
		t.Fatalf("expected http client with transport")
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", hc.Transport)
	}
	if tr.Proxy == nil {
		t.Fatalf("expected proxy function to be set")
	}
	pu, err := tr.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}})
	if err != nil {
		t.Fatalf("unexpected proxy error: %v", err)
	}
	if pu == nil || pu.Scheme != "http" || pu.Host != "127.0.0.1:7890" {
		t.Fatalf("unexpected proxy url: %#v", pu)
	}
}

func TestHTTPClientForProvider_NoProxyReturnsBase(t *testing.T) {
	base := &http.Client{Timeout: 3 * time.Second}
	c := &Client{
		HTTP:            base,
		ProxyByProvider: map[string]string{},
	}
	hc, err := c.httpClientForProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc != base {
		t.Fatalf("expected base client, got different instance")
	}
}

func TestHTTPClientForProvider_NilProxyMapReturnsBase(t *testing.T) {
	base := &http.Client{Timeout: 3 * time.Second}
	c := &Client{
		HTTP: base,
	}
	hc, err := c.httpClientForProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc != base {
		t.Fatalf("expected base client, got different instance")
	}
}

func TestHTTPClientForProvider_NilHTTPReturnsDefaultClient(t *testing.T) {
	c := &Client{}
	hc, err := c.httpClientForProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc != http.DefaultClient {
		t.Fatalf("expected default client, got %p want %p", hc, http.DefaultClient)
	}
}

func TestHTTPClientForProvider_InvalidProxyURL(t *testing.T) {
	c := &Client{
		HTTP: &http.Client{Timeout: 3 * time.Second},
		ProxyByProvider: map[string]string{
			"openai": "socks4://127.0.0.1:7890",
		},
	}
	_, err := c.httpClientForProvider("openai")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHTTPClientForProvider_SOCKS5(t *testing.T) {
	c := &Client{
		HTTP: &http.Client{Timeout: 3 * time.Second},
		ProxyByProvider: map[string]string{
			"openai": "socks5://127.0.0.1:7890",
		},
	}

	hc, err := c.httpClientForProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", hc.Transport)
	}
	if tr.DialContext == nil {
		t.Fatalf("expected DialContext to be set for socks5")
	}
	if tr.Proxy != nil {
		t.Fatalf("expected Proxy func to be nil for socks5 (use dialer instead)")
	}
}

func TestBedrockClientForProvider_UsesProviderProxy(t *testing.T) {
	c := &Client{
		HTTP: &http.Client{Timeout: 3 * time.Second},
		ProxyByProvider: map[string]string{
			"aws-bedrock": "http://127.0.0.1:1083",
		},
	}

	bc, err := c.bedrockClient("aws-bedrock", &dslmeta.Meta{
		AWSAccessKeyID:     "AKID",
		AWSSecretAccessKey: "SECRET",
		AWSRegion:          "us-east-1",
	})
	if err != nil {
		t.Fatalf("bedrockClient: %v", err)
	}
	opts := bc.Options()
	hc, ok := opts.HTTPClient.(*http.Client)
	if !ok {
		t.Fatalf("expected *http.Client, got %T", opts.HTTPClient)
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", hc.Transport)
	}
	if tr.Proxy == nil {
		t.Fatalf("expected proxy function to be set")
	}
	pu, err := tr.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "bedrock-runtime.us-east-1.amazonaws.com"}})
	if err != nil {
		t.Fatalf("unexpected proxy error: %v", err)
	}
	if pu == nil || pu.Scheme != "http" || pu.Host != "127.0.0.1:1083" {
		t.Fatalf("unexpected proxy url: %#v", pu)
	}
}
