package modelsquery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/httpclient"
)

type Params struct {
	Provider string
	File     dslconfig.ProviderFile
	Meta     *dslmeta.Meta
	BaseURL  string
	APIKey   string

	// HTTPClient allows next-router to inject a fake client for tests or to
	// override the default http.Client behavior.
	HTTPClient httpclient.HTTPDoer
	DebugOut   io.Writer
}

type Result struct {
	Provider string
	IDs      []string
}

func Query(ctx context.Context, p Params) (Result, error) {
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if provider == "" {
		return Result{}, errors.New("provider is empty")
	}

	meta := cloneMetaForQuery(p.Meta)
	meta.API = strings.TrimSpace(meta.API)
	if meta.API == "" {
		meta.API = "chat.completions"
	}
	if strings.TrimSpace(p.APIKey) != "" {
		meta.APIKey = strings.TrimSpace(p.APIKey)
	}

	cfg, ok := p.File.Models.Select(&meta)
	if !ok {
		return Result{}, fmt.Errorf("provider %q has no models config", provider)
	}

	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = resolveBaseURLFromExpr(p.File.Routing.BaseURLExpr)
	}
	if baseURL == "" {
		return Result{}, errors.New("base url is empty")
	}
	meta.BaseURL = baseURL

	headers := make(http.Header)
	p.File.Headers.Apply(&meta, nil, headers)
	applyHeaderOps(headers, cfg.Headers, &meta)

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	if ctx == nil {
		ctx = context.Background()
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	reqURL, err := buildModelsRequestURL(baseURL, cfg.Path)
	if err != nil {
		return Result{}, err
	}
	body, err := getResponseBody(ctx, client, method, reqURL, headers, p.DebugOut)
	if err != nil {
		return Result{}, err
	}
	ids, err := dslconfig.ExtractModelIDs(cfg, body)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Provider: provider,
		IDs:      ids,
	}, nil
}

func cloneMetaForQuery(src *dslmeta.Meta) dslmeta.Meta {
	if src == nil {
		return dslmeta.Meta{}
	}
	return dslmeta.Meta{
		API:              src.API,
		IsStream:         src.IsStream,
		BaseURL:          src.BaseURL,
		APIKey:           src.APIKey,
		OAuthAccessToken: src.OAuthAccessToken,
		OAuthCacheKey:    src.OAuthCacheKey,
		ActualModelName:  src.ActualModelName,
		DSLModelMapped:   src.DSLModelMapped,
		RequestURLPath:   src.RequestURLPath,
		StartTime:        src.StartTime,
	}
}

func getResponseBody(ctx context.Context, client httpclient.HTTPDoer, method, reqURL string, headers http.Header, debugOut io.Writer) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	mergeHeaders(req.Header, headers)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debugOut != nil {
		_, _ = fmt.Fprintf(debugOut, "debug upstream_response method=%s url=%s status=%d body=%s\n", method, reqURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request %s %s failed: status=%d body=%s", method, reqURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func mergeHeaders(dst, src http.Header) {
	for k, vals := range src {
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

func buildModelsRequestURL(baseURL, path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", errors.New("models path is empty")
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p, nil
	}
	b := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if b == "" {
		return "", errors.New("models baseURL is empty")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u, err := url.Parse(b + p)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func applyHeaderOps(h http.Header, ops []dslconfig.HeaderOp, meta *dslmeta.Meta) {
	for _, op := range ops {
		name := strings.TrimSpace(evalHeaderExpr(op.NameExpr, meta))
		if name == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(op.Op)) {
		case "header_set":
			h.Set(name, evalHeaderExpr(op.ValueExpr, meta))
		case "header_del":
			h.Del(name)
		}
	}
}

func evalHeaderExpr(expr string, meta *dslmeta.Meta) string {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")")
		parts := splitTopLevelArgs(inner)
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(evalHeaderExpr(p, meta))
		}
		return b.String()
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		v, err := strconv.Unquote(raw)
		if err == nil {
			return v
		}
		return raw
	}
	switch raw {
	case "$channel.base_url":
		if meta != nil {
			return meta.BaseURL
		}
	case "$channel.key":
		if meta != nil {
			return meta.APIKey
		}
	case "$oauth.access_token":
		if meta != nil {
			return meta.OAuthAccessToken
		}
	case "$request.model":
		if meta != nil {
			return meta.ActualModelName
		}
	case "$request.model_mapped":
		if meta != nil {
			if meta.DSLModelMapped != "" {
				return meta.DSLModelMapped
			}
			return meta.ActualModelName
		}
	}
	return raw
}

func splitTopLevelArgs(s string) []string {
	var parts []string
	var b strings.Builder
	depth := 0
	inString := false
	escaped := false
	flush := func() {
		part := strings.TrimSpace(b.String())
		if part != "" {
			parts = append(parts, part)
		}
		b.Reset()
	}
	for _, r := range s {
		ch := byte(r)
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			b.WriteByte(ch)
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			b.WriteByte(ch)
			continue
		}
		if !inString {
			switch ch {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case ',':
				if depth == 0 {
					flush()
					continue
				}
			}
		}
		b.WriteByte(ch)
	}
	flush()
	return parts
}

func resolveBaseURLFromExpr(expr string) string {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		v, err := strconv.Unquote(raw)
		if err == nil {
			return strings.TrimSpace(v)
		}
	}
	return raw
}
