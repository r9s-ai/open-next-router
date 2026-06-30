package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

func (c *Client) doBedrockRuntimeRequest(gc *gin.Context, provider string, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	operation, _, err := bedrockRuntimeTarget(m.RequestURLPath)
	if err != nil {
		return nil, func() {}, err
	}
	switch operation {
	case "invoke":
		return c.doBedrockInvokeModel(gc, provider, m, reqBody)
	case "invoke-with-response-stream":
		return c.doBedrockInvokeModelStream(gc, provider, m, reqBody)
	case "http-passthrough":
		return c.doBedrockHTTPPassthrough(gc, provider, m, reqBody)
	default:
		return nil, func() {}, fmt.Errorf("unsupported bedrock operation: %s", operation)
	}
}

func (c *Client) doBedrockHTTPPassthrough(gc *gin.Context, provider string, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	baseURL, err := bedrockHTTPPassthroughBaseURL(provider, m)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+m.RequestURLPath, bytes.NewReader(reqBody))
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	if ct := strings.TrimSpace(gc.Request.Header.Get("Content-Type")); ct != "" {
		req.Header.Set("Content-Type", ct)
	} else {
		req.Header.Set("Content-Type", contentTypeJSON)
	}
	if accept := strings.TrimSpace(gc.Request.Header.Get("Accept")); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if err := signBedrockHTTPRequest(reqCtx, req, m, reqBody); err != nil {
		cancel()
		return nil, func() {}, err
	}
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, req.Method, req.URL.String(), req.Header, limited, false, truncated)
	}
	resp, err := httpc.Do(req)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	return resp, cancel, nil
}

func (c *Client) doBedrockInvokeModel(gc *gin.Context, provider string, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	client, err := c.bedrockClient(provider, m)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	_, modelID, err := bedrockRuntimeTarget(m.RequestURLPath)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, http.MethodPost, "bedrock-runtime:"+modelID+":invoke_model", http.Header{}, limited, false, truncated)
	}
	out, err := client.InvokeModel(reqCtx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Accept:      aws.String(contentTypeJSON),
		ContentType: aws.String(contentTypeJSON),
		Body:        reqBody,
	})
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	resp := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentTypeJSON}},
		Body:       io.NopCloser(bytes.NewReader(out.Body)),
	}
	return resp, cancel, nil
}

func (c *Client) doBedrockInvokeModelStream(gc *gin.Context, provider string, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	client, err := c.bedrockClient(provider, m)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	_, modelID, err := bedrockRuntimeTarget(m.RequestURLPath)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	out, err := client.InvokeModelWithResponseStream(reqCtx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(modelID),
		Accept:      aws.String(contentTypeJSON),
		ContentType: aws.String(contentTypeJSON),
		Body:        reqBody,
	})
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	pr, pw := io.Pipe()
	stream := out.GetStream()
	go func() {
		defer func() {
			_ = stream.Close()
		}()
		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				if _, err := fmt.Fprintf(pw, "data: %s\n\n", string(v.Value.Bytes)); err != nil {
					_ = pw.CloseWithError(err)
					return
				}
			case *types.UnknownUnionMember:
				_ = pw.CloseWithError(fmt.Errorf("unknown bedrock eventstream tag: %s", v.Tag))
				return
			default:
				_ = pw.CloseWithError(fmt.Errorf("unsupported bedrock eventstream event: %T", event))
				return
			}
		}
		if _, err := io.WriteString(pw, "data: [DONE]\n\n"); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	resp := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       pr,
	}
	return resp, cancel, nil
}

func (c *Client) bedrockClient(provider string, m *dslmeta.Meta) (*bedrockruntime.Client, error) {
	region := strings.ToLower(strings.TrimSpace(m.AWSRegion))
	if region == "" {
		return nil, errors.New("aws region is required")
	}
	ak := strings.TrimSpace(m.AWSAccessKeyID)
	sk := strings.TrimSpace(m.AWSSecretAccessKey)
	if ak == "" || sk == "" {
		return nil, errors.New("aws access key id and secret access key are required")
	}
	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	opts := bedrockruntime.Options{
		Region:      region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(ak, sk, strings.TrimSpace(m.AWSSessionToken))),
		HTTPClient:  httpc,
	}
	if endpoint := strings.TrimSpace(m.BaseURL); endpoint != "" {
		opts.BaseEndpoint = aws.String(endpoint)
	}
	return bedrockruntime.New(opts), nil
}

func bedrockHTTPPassthroughBaseURL(provider string, m *dslmeta.Meta) (string, error) {
	if baseURL := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/"); baseURL != "" {
		return baseURL, nil
	}
	region := strings.ToLower(strings.TrimSpace(m.AWSRegion))
	if region == "" {
		return "", errors.New("aws region is required")
	}
	if strings.EqualFold(strings.TrimSpace(provider), "aws-bedrock-mantle") {
		return "https://bedrock-mantle." + region + ".api.aws", nil
	}
	return "https://bedrock-runtime." + region + ".amazonaws.com", nil
}

func signBedrockHTTPRequest(ctx context.Context, req *http.Request, m *dslmeta.Meta, body []byte) error {
	region := strings.ToLower(strings.TrimSpace(m.AWSRegion))
	if region == "" {
		return errors.New("aws region is required")
	}
	ak := strings.TrimSpace(m.AWSAccessKeyID)
	sk := strings.TrimSpace(m.AWSSecretAccessKey)
	if ak == "" || sk == "" {
		return errors.New("aws access key id and secret access key are required")
	}
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])
	creds := aws.Credentials{
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		SessionToken:    strings.TrimSpace(m.AWSSessionToken),
		Source:          "onr-bedrock-runtime",
	}
	return v4.NewSigner().SignHTTP(ctx, creds, req, payloadHash, "bedrock", region, time.Now().UTC())
}

func bedrockRuntimeTarget(requestPath string) (operation string, modelID string, err error) {
	u, err := url.Parse(strings.TrimSpace(requestPath))
	if err != nil {
		return "", "", fmt.Errorf("parse bedrock runtime path: %w", err)
	}
	path := u.EscapedPath()
	if path == "" {
		path = u.Path
	}
	if !strings.HasPrefix(path, "/model/") {
		if strings.HasPrefix(path, "/") {
			return "http-passthrough", "", nil
		}
		return "", "", fmt.Errorf("bedrock runtime path must be an absolute path: %s", requestPath)
	}
	rest := strings.TrimPrefix(path, "/model/")
	for _, suffix := range []string{"/invoke-with-response-stream", "/invoke"} {
		if !strings.HasSuffix(rest, suffix) {
			continue
		}
		encodedModelID := strings.TrimSuffix(rest, suffix)
		modelID, err := url.PathUnescape(encodedModelID)
		if err != nil {
			return "", "", fmt.Errorf("decode bedrock model id: %w", err)
		}
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return "", "", errors.New("bedrock model id is empty")
		}
		return strings.TrimPrefix(suffix, "/"), modelID, nil
	}
	return "", "", fmt.Errorf("unsupported bedrock runtime path: %s", requestPath)
}
