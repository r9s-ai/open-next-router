package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/oauthclient"
)

func (c *Client) prepareOAuthForUpstream(ctx context.Context, provider string, pf dslconfig.ProviderFile, meta *dslmeta.Meta) error {
	if c == nil || meta == nil {
		return nil
	}

	phase, ok := pf.Headers.Effective(meta)
	if !ok {
		return nil
	}
	resolved, ok := phase.OAuth.Resolve(meta)
	if !ok {
		meta.OAuthAccessToken = ""
		meta.OAuthCacheKey = ""
		return nil
	}

	cacheKey := buildOAuthCacheKey(provider, resolved.CacheIdentity(), meta.APIKey)
	meta.OAuthCacheKey = cacheKey

	client := c.oauthTokenClient()
	if client == nil {
		return fmt.Errorf("oauth client is nil")
	}
	tok, err := client.GetToken(ctx, oauthclient.AcquireInput{
		CacheKey:          cacheKey,
		TokenURL:          resolved.TokenURL,
		Method:            resolved.Method,
		ContentType:       resolved.ContentType,
		Form:              resolved.Form,
		BasicAuthUsername: resolved.BasicAuthUsername,
		BasicAuthPassword: resolved.BasicAuthPassword,
		TokenPath:         resolved.TokenPath,
		ExpiresInPath:     resolved.ExpiresInPath,
		TokenTypePath:     resolved.TokenTypePath,
		Timeout:           time.Duration(resolved.TimeoutMs) * time.Millisecond,
		RefreshSkew:       time.Duration(resolved.RefreshSkewSec) * time.Second,
		FallbackTTL:       time.Duration(resolved.FallbackTTLSec) * time.Second,
	})
	if err != nil {
		return err
	}
	meta.OAuthAccessToken = strings.TrimSpace(tok.AccessToken)
	return nil
}

func (c *Client) oauthTokenClient() *oauthclient.Client {
	if c == nil {
		return nil
	}
	c.oauthMu.Lock()
	defer c.oauthMu.Unlock()
	if c.oauthClient == nil {
		c.oauthClient = oauthclient.New(c.HTTP, c.OAuthTokenPersistEnabled, c.OAuthTokenPersistDir)
	}
	return c.oauthClient
}

func (c *Client) invalidateOAuthCache(cacheKey string) {
	if c == nil {
		return
	}
	client := c.oauthTokenClient()
	if client == nil {
		return
	}
	client.Invalidate(cacheKey)
}

func buildOAuthCacheKey(provider string, identity string, apiKey string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	id := strings.TrimSpace(identity)
	h := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return p + "|" + id + "|" + hex.EncodeToString(h[:])
}
