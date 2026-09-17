package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
	"github.com/r9s-ai/open-next-router/onr/internal/auth"
)

type proxyCtx struct {
	start    time.Time
	provider string
	key      ProviderKey
	api      string
	stream   bool
	pf       dslconfig.ProviderFile
	meta     *dslmeta.Meta
	model    string
	reqBody  []byte
	// reqContentType is the content type of reqBody after the request
	// transform, which is not the client's when req_map rewrote a multipart
	// upload into JSON.
	reqContentType string
	respDir        *dslconfig.ResponseDirective
	reqTransform   *dslconfig.RequestTransform
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
	// Pin a snapshot only when extensions are enabled; ordinary requests do
	// not allocate a snapshot map or create a JavaScript VM.
	snapshot := c.Registry.Snapshot()
	if pinned, exists := gc.Get("onr.js.snapshot"); exists {
		snapshot = pinned.(dslconfig.RegistrySnapshot)
	}
	pf, ok := snapshot.GetProvider(provider)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", provider)
	}
	hasJS := pf.JS.Select(api, stream).Enabled()
	if hasJS {
		gc.Set("onr.js.snapshot", snapshot)
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
	if hasJS {
		gc.Set("onr.request_body", bodyBytes)
		gc.Set("onr.request_model", model)
		gc.Set("onr.request_content_type", gc.Request.Header.Get("Content-Type"))
		if root != nil {
			original, _ := json.Marshal(root)
			root = nil
			_ = json.Unmarshal(original, &root)
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
		AWSAccessKeyID:     strings.TrimSpace(key.AWSAccessKeyID),
		AWSSecretAccessKey: strings.TrimSpace(key.AWSSecretAccessKey),
		AWSSessionToken:    strings.TrimSpace(key.AWSSessionToken),
		AWSRegion:          normalizeProviderLocation(firstNonEmpty(key.AWSRegion, key.Location), false),
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

	if !pf.Routing.HasMatch(m) {
		return nil, fmt.Errorf("dsl provider no match (provider=%s api=%s stream=%v)", provider, api, stream)
	}

	respDir, _ := pf.Response.Select(m)

	reqTransform, hasReqTransform := selectRequestTransform(pf, m)
	// Capture the client's original query string before routing rewrites it.
	originalRawQuery := gc.Request.URL.RawQuery
	if err := pf.Routing.Apply(m); err != nil {
		return nil, err
	}
	m.BaseURL = normalizeUpstreamBaseURL(m.BaseURL)
	applyGeminiModelRewrite(api, m)

	session, err := c.startJS(gc, pf, m)
	if err != nil {
		return nil, &jsext.Error{Stage: jsext.Request, Cause: err}
	}
	var preDelta http.Header
	if session != nil {
		m.RequestHeaders = gc.Request.Header.Clone()
		if session.Has(jsext.Request) && gc.GetHeader("Content-Encoding") != "" && gc.GetHeader("Content-Encoding") != "identity" {
			return nil, &jsext.Error{Stage: jsext.Request, Cause: fmt.Errorf("encoded request is not supported")}
		}
		changedBody, changedRoot, hookErr := runRequestJS(gc, session, jsext.Request, m, bodyBytes, gc.GetHeader("Content-Type"), m.RequestHeaders)
		if hookErr != nil {
			return nil, hookErr
		}
		bodyBytes = changedBody
		if changedRoot != nil {
			root = changedRoot
			m.RequestBody = bodyBytes
			m.SetRequestRoot(root)
		}
		preDelta = headerDelta(gc.Request.Header, m.RequestHeaders)
	}
	reqResult, err := applyRequestTransform(m, gc.Request.Header.Get("Content-Type"), gc.GetHeader("Content-Encoding"), originalRawQuery, bodyBytes, root, reqTransform, hasReqTransform)
	if err != nil {
		return nil, err
	}
	reqBody := reqResult.Body
	if session != nil {
		headers := http.Header{}
		headers.Set("Content-Type", reqResult.ContentType)
		pf.Headers.Apply(m, m.RequestHeaders, headers)
		baseline := headers.Clone()
		for k, v := range preDelta {
			headers[k] = v
		}
		body, newRoot, hookErr := runRequestJS(gc, session, jsext.AfterRequestMap, m, reqBody, reqResult.ContentType, headers)
		if hookErr != nil {
			return nil, hookErr
		}
		reqBody = body
		m.RequestBody = body
		if newRoot != nil {
			m.SetRequestRoot(newRoot)
		} else if reqResult.Root != nil {
			m.SetRequestRoot(reqResult.Root)
		}
		gc.Set("onr.js.headers", headerDelta(baseline, headers))
		gc.Set("onr.js.final_headers", headers.Clone())
	}

	return &proxyCtx{
		start:          start,
		provider:       provider,
		key:            key,
		api:            api,
		stream:         stream,
		pf:             pf,
		meta:           m,
		model:          model,
		reqBody:        reqBody,
		reqContentType: reqResult.ContentType,
		respDir:        respDir,
		reqTransform:   reqTransform,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
