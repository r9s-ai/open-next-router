package oauthclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Token struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
}

type AcquireInput struct {
	CacheKey string

	TokenURL    string
	Method      string
	ContentType string // form|json
	Form        map[string]string

	BasicAuthUsername string
	BasicAuthPassword string

	TokenPath     string
	ExpiresInPath string
	TokenTypePath string
	Timeout       time.Duration
	RefreshSkew   time.Duration
	FallbackTTL   time.Duration
}

type Client struct {
	httpClient *http.Client

	persistEnabled bool
	persistDir     string

	mu       sync.Mutex
	cache    map[string]Token
	inFlight map[string]*flight
}

type flight struct {
	done  chan struct{}
	token Token
	err   error
}

func New(httpClient *http.Client, persistEnabled bool, persistDir string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient:     httpClient,
		persistEnabled: persistEnabled,
		persistDir:     strings.TrimSpace(persistDir),
		cache:          map[string]Token{},
		inFlight:       map[string]*flight{},
	}
}

func (c *Client) Invalidate(cacheKey string) {
	key := strings.TrimSpace(cacheKey)
	if key == "" {
		return
	}
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

func (c *Client) GetToken(ctx context.Context, in AcquireInput) (Token, error) {
	key := strings.TrimSpace(in.CacheKey)
	if key == "" {
		return Token{}, errors.New("oauth cache key is empty")
	}
	if strings.TrimSpace(in.TokenURL) == "" {
		return Token{}, errors.New("oauth token url is empty")
	}
	if strings.TrimSpace(in.Method) == "" {
		in.Method = http.MethodPost
	}
	if in.Timeout <= 0 {
		in.Timeout = 5 * time.Second
	}
	if in.RefreshSkew < 0 {
		in.RefreshSkew = 0
	}
	if in.FallbackTTL <= 0 {
		in.FallbackTTL = 30 * time.Minute
	}

	if tok, ok := c.getCachedValid(key, in.RefreshSkew); ok {
		return tok, nil
	}
	if c.persistEnabled {
		if tok, ok := c.loadPersistedValid(key, in.RefreshSkew); ok {
			c.mu.Lock()
			c.cache[key] = tok
			c.mu.Unlock()
			return tok, nil
		}
	}

	f, owner := c.beginFlight(key)
	if !owner {
		<-f.done
		return f.token, f.err
	}
	defer c.endFlight(key, f)

	token, err := c.requestToken(ctx, in)
	if err != nil {
		f.err = err
		return Token{}, err
	}
	c.mu.Lock()
	c.cache[key] = token
	c.mu.Unlock()
	if c.persistEnabled {
		_ = c.savePersisted(key, token)
	}
	f.token = token
	return token, nil
}

func (c *Client) getCachedValid(cacheKey string, skew time.Duration) (Token, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tok, ok := c.cache[cacheKey]
	if !ok || strings.TrimSpace(tok.AccessToken) == "" {
		return Token{}, false
	}
	if time.Now().Add(skew).Before(tok.ExpiresAt) {
		return tok, true
	}
	return Token{}, false
}

func (c *Client) beginFlight(cacheKey string) (*flight, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f, ok := c.inFlight[cacheKey]; ok && f != nil {
		return f, false
	}
	f := &flight{done: make(chan struct{})}
	c.inFlight[cacheKey] = f
	return f, true
}

func (c *Client) endFlight(cacheKey string, f *flight) {
	c.mu.Lock()
	if cur, ok := c.inFlight[cacheKey]; ok && cur == f {
		delete(c.inFlight, cacheKey)
	}
	c.mu.Unlock()
	close(f.done)
}

func (c *Client) requestToken(ctx context.Context, in AcquireInput) (Token, error) {
	reqCtx, cancel := context.WithTimeout(ctx, in.Timeout)
	defer cancel()

	method := strings.ToUpper(strings.TrimSpace(in.Method))
	target := strings.TrimSpace(in.TokenURL)

	body, contentType, values, err := buildBody(method, strings.ToLower(strings.TrimSpace(in.ContentType)), in.Form)
	if err != nil {
		return Token{}, err
	}
	if method == http.MethodGet && len(values) > 0 {
		u, err := url.Parse(target)
		if err != nil {
			return Token{}, err
		}
		q := u.Query()
		for k, vs := range values {
			if len(vs) > 0 {
				q.Set(k, vs[0])
			}
		}
		u.RawQuery = q.Encode()
		target = u.String()
	}

	req, err := http.NewRequestWithContext(reqCtx, method, target, body)
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.TrimSpace(in.BasicAuthUsername) != "" || strings.TrimSpace(in.BasicAuthPassword) != "" {
		req.SetBasicAuth(in.BasicAuthUsername, in.BasicAuthPassword)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Token{}, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Token{}, fmt.Errorf("oauth token endpoint failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(bodyBytes))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return Token{}, fmt.Errorf("oauth token endpoint non-json response: %w", err)
	}

	tokenPath := firstNonEmpty(strings.TrimSpace(in.TokenPath), "$.access_token")
	expiresPath := firstNonEmpty(strings.TrimSpace(in.ExpiresInPath), "$.expires_in")
	typePath := firstNonEmpty(strings.TrimSpace(in.TokenTypePath), "$.token_type")

	access := strings.TrimSpace(getStringByPath(root, tokenPath))
	if access == "" {
		return Token{}, fmt.Errorf("oauth token not found at %s", tokenPath)
	}
	tokenType := strings.TrimSpace(getStringByPath(root, typePath))
	if tokenType == "" {
		tokenType = "Bearer"
	}
	expiresIn := int(getFloatByPath(root, expiresPath))
	if expiresIn <= 0 {
		expiresIn = int(in.FallbackTTL.Seconds())
	}
	if expiresIn <= 0 {
		expiresIn = 1800
	}
	return Token{
		AccessToken: access,
		TokenType:   tokenType,
		ExpiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func buildBody(method string, contentType string, form map[string]string) (io.Reader, string, url.Values, error) {
	m := strings.ToUpper(strings.TrimSpace(method))
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		ct = "form"
	}
	values := url.Values{}
	for k, v := range form {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		values.Set(key, v)
	}

	switch m {
	case http.MethodGet:
		return nil, "", values, nil
	case http.MethodPost:
		switch ct {
		case "json":
			raw, err := json.Marshal(valuesToMap(values))
			if err != nil {
				return nil, "", nil, err
			}
			return bytes.NewReader(raw), "application/json", values, nil
		default:
			return strings.NewReader(values.Encode()), "application/x-www-form-urlencoded", values, nil
		}
	default:
		return nil, "", nil, fmt.Errorf("unsupported oauth method %q", method)
	}
}

func valuesToMap(v url.Values) map[string]string {
	out := map[string]string{}
	for k, vals := range v {
		if len(vals) == 0 {
			continue
		}
		out[k] = vals[0]
	}
	return out
}

type persistedToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresAt   int64  `json:"expires_at"`
}

func (c *Client) loadPersistedValid(cacheKey string, skew time.Duration) (Token, bool) {
	path := c.persistPath(cacheKey)
	if path == "" {
		return Token{}, false
	}
	// #nosec G304 -- persistence path is generated by hash under configured directory.
	b, err := os.ReadFile(path)
	if err != nil {
		return Token{}, false
	}
	var p persistedToken
	if err := json.Unmarshal(b, &p); err != nil {
		return Token{}, false
	}
	if strings.TrimSpace(p.AccessToken) == "" || p.ExpiresAt <= 0 {
		return Token{}, false
	}
	tok := Token{
		AccessToken: p.AccessToken,
		TokenType:   p.TokenType,
		ExpiresAt:   time.Unix(p.ExpiresAt, 0),
	}
	if time.Now().Add(skew).Before(tok.ExpiresAt) {
		return tok, true
	}
	return Token{}, false
}

func (c *Client) savePersisted(cacheKey string, tok Token) error {
	path := c.persistPath(cacheKey)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	p := persistedToken{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		ExpiresAt:   tok.ExpiresAt.Unix(),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *Client) persistPath(cacheKey string) string {
	dir := strings.TrimSpace(c.persistDir)
	if dir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(cacheKey)))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dir, name)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
