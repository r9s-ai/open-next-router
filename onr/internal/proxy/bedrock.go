package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

func (c *Client) doBedrockRuntimeRequest(gc *gin.Context, provider string, pf *dslconfig.ProviderFile, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	operation, err := bedrockRuntimeTarget(m.RequestURLPath)
	if err != nil {
		return nil, func() {}, err
	}
	switch operation {
	case "invoke":
		return c.doBedrockInvokeModel(gc, provider, m, reqBody)
	case "invoke-with-response-stream":
		return c.doBedrockInvokeModelStream(gc, provider, m, reqBody)
	case "http-passthrough":
		return c.doBedrockHTTPPassthrough(gc, provider, pf, m, reqBody)
	default:
		return nil, func() {}, fmt.Errorf("unsupported bedrock operation: %s", operation)
	}
}

func (c *Client) doBedrockHTTPPassthrough(gc *gin.Context, provider string, pf *dslconfig.ProviderFile, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
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
	if pf != nil {
		pf.Headers.Apply(m, gc.Request.Header, req.Header)
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
	req, err := c.newBedrockRuntimeHTTPRequest(reqCtx, m, reqBody)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Content-Type", contentTypeJSON)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, req.Method, req.URL.String(), req.Header, limited, false, truncated)
	}
	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	return resp, cancel, nil
}

func (c *Client) doBedrockInvokeModelStream(gc *gin.Context, provider string, m *dslmeta.Meta, reqBody []byte) (*http.Response, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(gc.Request.Context(), c.WriteTimeout)
	req, err := c.newBedrockRuntimeHTTPRequest(reqCtx, m, reqBody)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Content-Type", contentTypeJSON)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		limited, truncated := trafficdump.LimitBytes(reqBody, rec.MaxBytes())
		trafficdump.AppendUpstreamRequest(gc, req.Method, req.URL.String(), req.Header, limited, false, truncated)
	}
	httpc, err := c.httpClientForProvider(provider)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	upstreamResp, err := httpc.Do(req)
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	if upstreamResp.StatusCode < http.StatusOK || upstreamResp.StatusCode >= http.StatusMultipleChoices {
		return upstreamResp, cancel, nil
	}
	pr, pw := io.Pipe()
	go func() {
		defer func() {
			_ = upstreamResp.Body.Close()
		}()
		if err := writeBedrockEventStreamAsSSE(pw, upstreamResp.Body); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	resp := &http.Response{
		Status:     upstreamResp.Status,
		StatusCode: upstreamResp.StatusCode,
		Header:     cloneHeader(upstreamResp.Header),
		Body:       pr,
	}
	resp.Header.Set("Content-Type", "text/event-stream")
	return resp, cancel, nil
}

func (c *Client) newBedrockRuntimeHTTPRequest(ctx context.Context, m *dslmeta.Meta, body []byte) (*http.Request, error) {
	baseURL, err := bedrockRuntimeBaseURL(m)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+m.RequestURLPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := signBedrockHTTPRequest(ctx, req, m, body); err != nil {
		return nil, err
	}
	return req, nil
}

func writeBedrockEventStreamAsSSE(w io.Writer, r io.Reader) error {
	decoder := eventstream.NewDecoder()
	var payloadBuf []byte
	for {
		payloadBuf = payloadBuf[:0]
		msg, err := decoder.Decode(r, payloadBuf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				_, err = io.WriteString(w, "data: [DONE]\n\n")
				return err
			}
			return err
		}
		chunk, err := bedrockEventStreamChunkBytes(msg)
		if err != nil {
			return err
		}
		if chunk == nil {
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", string(chunk)); err != nil {
			return err
		}
	}
}

func bedrockEventStreamChunkBytes(msg eventstream.Message) ([]byte, error) {
	messageType := msg.Headers.Get(eventstreamapi.MessageTypeHeader)
	if messageType == nil {
		return nil, fmt.Errorf("%s event header not present", eventstreamapi.MessageTypeHeader)
	}
	switch messageType.String() {
	case eventstreamapi.EventMessageType:
		eventType := msg.Headers.Get(eventstreamapi.EventTypeHeader)
		if eventType == nil {
			return nil, fmt.Errorf("%s event header not present", eventstreamapi.EventTypeHeader)
		}
		if !strings.EqualFold(eventType.String(), "chunk") {
			return nil, fmt.Errorf("unsupported bedrock eventstream event: %s", eventType.String())
		}
		var payload struct {
			Bytes []byte `json:"bytes"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode bedrock chunk payload: %w", err)
		}
		if payload.Bytes == nil {
			var raw struct {
				Bytes string `json:"bytes"`
			}
			if err := json.Unmarshal(msg.Payload, &raw); err != nil {
				return nil, fmt.Errorf("decode bedrock chunk bytes: %w", err)
			}
			decoded, err := base64.StdEncoding.DecodeString(raw.Bytes)
			if err != nil {
				return nil, fmt.Errorf("decode bedrock chunk bytes: %w", err)
			}
			payload.Bytes = decoded
		}
		return payload.Bytes, nil
	case eventstreamapi.ErrorMessageType:
		return nil, bedrockEventStreamHeaderError(msg, eventstreamapi.ErrorCodeHeader, eventstreamapi.ErrorMessageHeader)
	case eventstreamapi.ExceptionMessageType:
		return nil, bedrockEventStreamHeaderError(msg, eventstreamapi.ExceptionTypeHeader, eventstreamapi.ErrorMessageHeader)
	default:
		return nil, fmt.Errorf("unsupported bedrock eventstream message type: %s", messageType.String())
	}
}

func bedrockEventStreamHeaderError(msg eventstream.Message, codeHeader string, messageHeader string) error {
	code := "UnknownError"
	message := code
	if header := msg.Headers.Get(codeHeader); header != nil {
		code = strings.TrimSpace(header.String())
	}
	if header := msg.Headers.Get(messageHeader); header != nil {
		message = strings.TrimSpace(header.String())
	}
	if message == "" && len(msg.Payload) > 0 {
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err == nil {
			message = strings.TrimSpace(payload.Message)
		}
	}
	if message == "" {
		message = code
	}
	return fmt.Errorf("%s: %s", code, message)
}

func cloneHeader(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for k, values := range in {
		out[k] = append([]string(nil), values...)
	}
	return out
}

func bedrockRuntimeBaseURL(m *dslmeta.Meta) (string, error) {
	if baseURL := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/"); baseURL != "" {
		return baseURL, nil
	}
	region := strings.ToLower(strings.TrimSpace(m.AWSRegion))
	if region == "" {
		return "", errors.New("aws region is required")
	}
	return "https://bedrock-runtime." + region + ".amazonaws.com", nil
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

func bedrockRuntimeTarget(requestPath string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(requestPath))
	if err != nil {
		return "", fmt.Errorf("parse bedrock runtime path: %w", err)
	}
	path := u.EscapedPath()
	if path == "" {
		path = u.Path
	}
	if !strings.HasPrefix(path, "/model/") {
		if strings.HasPrefix(path, "/") {
			return "http-passthrough", nil
		}
		return "", fmt.Errorf("bedrock runtime path must be an absolute path: %s", requestPath)
	}
	rest := strings.TrimPrefix(path, "/model/")
	for _, suffix := range []string{"/invoke-with-response-stream", "/invoke"} {
		if !strings.HasSuffix(rest, suffix) {
			continue
		}
		encodedModelID := strings.TrimSuffix(rest, suffix)
		modelID, err := url.PathUnescape(encodedModelID)
		if err != nil {
			return "", fmt.Errorf("decode bedrock model id: %w", err)
		}
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return "", errors.New("bedrock model id is empty")
		}
		return strings.TrimPrefix(suffix, "/"), nil
	}
	return "", fmt.Errorf("unsupported bedrock runtime path: %s", requestPath)
}
