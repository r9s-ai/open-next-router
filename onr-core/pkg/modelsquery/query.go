package modelsquery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
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

	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSSessionToken    string
	AWSRegion          string

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
	if strings.TrimSpace(p.AWSAccessKeyID) != "" {
		meta.AWSAccessKeyID = strings.TrimSpace(p.AWSAccessKeyID)
	}
	if strings.TrimSpace(p.AWSSecretAccessKey) != "" {
		meta.AWSSecretAccessKey = strings.TrimSpace(p.AWSSecretAccessKey)
	}
	if strings.TrimSpace(p.AWSSessionToken) != "" {
		meta.AWSSessionToken = strings.TrimSpace(p.AWSSessionToken)
	}
	if strings.TrimSpace(p.AWSRegion) != "" {
		meta.AWSRegion = strings.ToLower(strings.TrimSpace(p.AWSRegion))
	}

	cfg, ok := p.File.Models.Select(meta)
	if !ok {
		return nil, fmt.Errorf("provider %q has no models config", provider)
	}

	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = dslquery.ResolveBaseURLFromExpr(p.File.Routing.BaseURLExpr)
	}
	transport := strings.TrimSpace(p.File.Routing.Transport)
	if transport == "aws_sdk" {
		var err error
		baseURL, err = bedrockModelsBaseURL(provider, meta, baseURL, cfg.Path)
		if err != nil {
			return nil, err
		}
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
	body, err := getModelsResponseBody(ctx, client, method, reqURL, headers, p.DebugOut, meta, transport)
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

func getModelsResponseBody(ctx context.Context, client httpclient.HTTPDoer, method, reqURL string, headers http.Header, debugOut io.Writer, meta *dslmeta.Meta, transport string) ([]byte, error) {
	if strings.TrimSpace(transport) != "aws_sdk" {
		return dslquery.GetResponseBody(ctx, client, method, reqURL, headers, debugOut)
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	dslquery.MergeHeaders(req.Header, headers)
	if err := signBedrockModelsRequest(ctx, req, meta); err != nil {
		return nil, err
	}
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

func bedrockModelsBaseURL(provider string, meta *dslmeta.Meta, override string, modelsPath string) (string, error) {
	endpoint := bedrockModelsEndpoint(provider, modelsPath)
	if baseURL := bedrockModelsBaseURLFromOverride(override, endpoint); baseURL != "" {
		return baseURL, nil
	}
	region := strings.ToLower(strings.TrimSpace(meta.AWSRegion))
	if region == "" {
		return "", errors.New("aws region is required")
	}
	if endpoint == "mantle" {
		return "https://bedrock-mantle." + region + ".api.aws", nil
	}
	return "https://bedrock." + region + ".amazonaws.com", nil
}

func bedrockModelsEndpoint(provider string, modelsPath string) string {
	if strings.EqualFold(strings.TrimSpace(provider), "aws-bedrock-mantle") {
		return "mantle"
	}
	_ = modelsPath
	return "control"
}

func bedrockModelsBaseURLFromOverride(raw string, endpoint string) string {
	baseURL := strings.TrimSpace(raw)
	if baseURL == "" {
		return ""
	}
	if endpoint == "mantle" {
		baseURL = strings.Replace(baseURL, "://bedrock-runtime.", "://bedrock-mantle.", 1)
		baseURL = strings.Replace(baseURL, "://bedrock.", "://bedrock-mantle.", 1)
		baseURL = strings.Replace(baseURL, ".amazonaws.com", ".api.aws", 1)
		return baseURL
	}
	baseURL = strings.Replace(baseURL, "://bedrock-runtime.", "://bedrock.", 1)
	baseURL = strings.Replace(baseURL, "://bedrock-mantle.", "://bedrock.", 1)
	baseURL = strings.Replace(baseURL, ".api.aws", ".amazonaws.com", 1)
	baseURL = strings.Replace(baseURL, "://bedrock-runtime-fips.", "://bedrock-fips.", 1)
	return baseURL
}

func signBedrockModelsRequest(ctx context.Context, req *http.Request, meta *dslmeta.Meta) error {
	region := strings.ToLower(strings.TrimSpace(meta.AWSRegion))
	if region == "" {
		return errors.New("aws region is required")
	}
	ak := strings.TrimSpace(meta.AWSAccessKeyID)
	sk := strings.TrimSpace(meta.AWSSecretAccessKey)
	if ak == "" || sk == "" {
		return errors.New("aws access key id and secret access key are required")
	}
	sum := sha256.Sum256(nil)
	payloadHash := hex.EncodeToString(sum[:])
	creds := aws.Credentials{
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		SessionToken:    strings.TrimSpace(meta.AWSSessionToken),
		Source:          "onr-modelsquery",
	}
	return v4.NewSigner().SignHTTP(ctx, creds, req, payloadHash, "bedrock", region, time.Now().UTC())
}
