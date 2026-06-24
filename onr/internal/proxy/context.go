package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr/internal/auth"
)

type proxyCtx struct {
	start        time.Time
	provider     string
	key          ProviderKey
	api          string
	stream       bool
	pf           dslconfig.ProviderFile
	meta         *dslmeta.Meta
	model        string
	reqBody      []byte
	respDir      *dslconfig.ResponseDirective
	reqTransform *dslconfig.RequestTransform
}

func normalizeUpstreamBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func normalizeProviderLocation(raw string, defaultGlobal bool) string {
	location := strings.ToLower(strings.TrimSpace(raw))
	if location == "" && defaultGlobal {
		return "global"
	}
	return location
}

func credentialProjectIDFromFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	// #nosec G304 -- credential file path is supplied by trusted local ONR configuration.
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read credential file: %w", err)
	}
	var doc struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", errors.New("credential file is not valid JSON")
	}
	projectID := strings.TrimSpace(doc.ProjectID)
	if projectID == "" {
		return "", errors.New("credential file missing project_id")
	}
	return projectID, nil
}

func (c *Client) buildProxyCtx(gc *gin.Context, provider string, key ProviderKey, api string, stream bool) (*proxyCtx, error) {
	start := time.Now()
	pf, ok := c.Registry.GetProvider(provider)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", provider)
	}

	bodyBytes, root, model, _, err := readRequestBody(gc, api)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(model) == "" {
		if v, ok := gc.Get("onr.model"); ok {
			model = strings.TrimSpace(fmt.Sprintf("%v", v))
		}
	}
	if mo := auth.TokenModelOverride(gc); mo != "" {
		model = mo
		if root != nil {
			if _, exists := root["model"]; exists {
				root["model"] = mo
			}
		}
	}

	m := &dslmeta.Meta{
		API:                strings.TrimSpace(api),
		IsStream:           stream,
		OriginModelName:    strings.TrimSpace(model),
		APIKey:             strings.TrimSpace(key.Value),
		BaseURL:            normalizeUpstreamBaseURL(key.BaseURLOverride),
		CredentialFile:     strings.TrimSpace(key.CredentialFile),
		ChannelLocation:    normalizeProviderLocation(key.Location, strings.TrimSpace(key.CredentialFile) != ""),
		RequestURLPath:     gc.Request.URL.RequestURI(),
		RequestContentType: gc.Request.Header.Get("Content-Type"),
		RequestHeaders:     gc.Request.Header,
		RequestBody:        bodyBytes,
		StartTime:          time.Now(),
	}
	if projectID, err := credentialProjectIDFromFile(m.CredentialFile); err != nil {
		return nil, err
	} else {
		m.CredentialProjectID = projectID
	}
	m.SetRequestRoot(root)
	if mo := strings.TrimSpace(model); mo != "" {
		if newPath, ok := replaceGeminiModelInPath(m.RequestURLPath, mo); ok {
			m.RequestURLPath = newPath
		}
	}

	if err := pf.Routing.Apply(m); err != nil {
		return nil, err
	}
	m.BaseURL = normalizeUpstreamBaseURL(m.BaseURL)
	if !pf.Routing.HasMatch(m) {
		return nil, fmt.Errorf("dsl provider no match (provider=%s api=%s stream=%v)", provider, api, stream)
	}

	respDir, _ := pf.Response.Select(m)

	reqTransform, hasReqTransform := selectRequestTransform(pf, m)
	applyGeminiModelRewrite(api, m)

	reqResult, err := applyRequestTransform(m, gc.Request.Header.Get("Content-Type"), gc.GetHeader("Content-Encoding"), bodyBytes, root, reqTransform, hasReqTransform)
	if err != nil {
		return nil, err
	}
	reqBody := reqResult.Body

	return &proxyCtx{
		start:        start,
		provider:     provider,
		key:          key,
		api:          api,
		stream:       stream,
		pf:           pf,
		meta:         m,
		model:        model,
		reqBody:      reqBody,
		respDir:      respDir,
		reqTransform: reqTransform,
	}, nil
}
