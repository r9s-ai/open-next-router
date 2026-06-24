package modelsquery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslquery"
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

// Query requires a non-nil ctx and p.Meta.
// It returns a non-nil Result on success.
func Query(ctx context.Context, p Params) (*Result, error) {
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if provider == "" {
		return nil, errors.New("provider is empty")
	}

	meta := dslmeta.Clone(p.Meta)
	meta.API = strings.TrimSpace(meta.API)
	if meta.API == "" {
		meta.API = "chat.completions"
	}
	if strings.TrimSpace(p.APIKey) != "" {
		meta.APIKey = strings.TrimSpace(p.APIKey)
	}

	cfg, ok := p.File.Models.Select(meta)
	if !ok {
		return nil, fmt.Errorf("provider %q has no models config", provider)
	}

	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = dslquery.ResolveBaseURLFromExpr(p.File.Routing.BaseURLExpr)
	}
	if baseURL == "" {
		return nil, errors.New("base url is empty")
	}
	meta.BaseURL = baseURL

	headers := make(http.Header)
	p.File.Headers.Apply(meta, nil, headers)
	dslquery.ApplyHeaderOps(headers, cfg.Headers, meta)

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	reqURL, err := dslquery.BuildRequestURL(baseURL, "", dslconfig.EvalStringExpr(cfg.Path, meta), "models")
	if err != nil {
		return nil, err
	}
	body, err := dslquery.GetResponseBody(ctx, client, method, reqURL, headers, p.DebugOut)
	if err != nil {
		return nil, err
	}
	ids, err := dslconfig.ExtractModelIDs(cfg, body)
	if err != nil {
		return nil, err
	}
	return &Result{
		Provider: provider,
		IDs:      ids,
	}, nil
}
