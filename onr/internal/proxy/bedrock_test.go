package proxy

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestBedrockRuntimeTargetHTTPPassthroughPaths(t *testing.T) {
	for _, path := range []string{
		"/v1/chat/completions",
		"/openai/v1/chat/completions",
		"/v1/responses",
		"/anthropic/v1/messages",
		"/custom/v1/anything",
	} {
		t.Run(path, func(t *testing.T) {
			op, err := bedrockRuntimeTarget(path)
			if err != nil {
				t.Fatalf("bedrockRuntimeTarget: %v", err)
			}
			if op != "http-passthrough" {
				t.Fatalf("operation=%q", op)
			}
		})
	}
}

func TestWriteBedrockEventStreamAsSSE(t *testing.T) {
	var input bytes.Buffer
	writeBedrockEventStreamMessage(t, &input, []byte(`{"type":"content_block_delta","delta":{"text":"hi"}}`))

	var out bytes.Buffer
	if err := writeBedrockEventStreamAsSSE(&out, &input); err != nil {
		t.Fatalf("writeBedrockEventStreamAsSSE: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `data: {"type":"content_block_delta","delta":{"text":"hi"}}`) {
		t.Fatalf("missing chunk in SSE: %s", got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("missing DONE in SSE: %s", got)
	}
}

func TestDoBedrockInvokeModelStreamUsesHTTPAndDecodesEventStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotPath string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		writeBedrockEventStreamMessage(t, w, []byte(`{"type":"content_block_delta","delta":{"text":"hi"}}`))
	}))
	t.Cleanup(srv.Close)

	gc, _ := gin.CreateTestContext(httptest.NewRecorder())
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c := &Client{HTTP: srv.Client(), WriteTimeout: 5 * time.Second}
	resp, cancel, err := c.doBedrockInvokeModelStream(gc, "aws-bedrock", &dslmeta.Meta{
		BaseURL:            srv.URL,
		AWSAccessKeyID:     "AKID",
		AWSSecretAccessKey: "SECRET",
		AWSRegion:          "us-east-1",
		RequestURLPath:     "/model/anthropic.claude-3-5-sonnet-20241022-v2%3A0/invoke-with-response-stream",
	}, []byte(`{"anthropic_version":"bedrock-2023-05-31","messages":[]}`))
	if err != nil {
		t.Fatalf("doBedrockInvokeModelStream: %v", err)
	}
	defer cancel()
	defer func() { _ = resp.Body.Close() }()

	if gotPath != "/model/anthropic.claude-3-5-sonnet-20241022-v2%3A0/invoke-with-response-stream" {
		t.Fatalf("path=%q", gotPath)
	}
	if !strings.Contains(gotAuth, "AWS4-HMAC-SHA256") ||
		!strings.Contains(gotAuth, "Credential=AKID/") ||
		!strings.Contains(gotAuth, "/us-east-1/bedrock/aws4_request") {
		t.Fatalf("Authorization=%q", gotAuth)
	}
	if strings.Contains(gotAuth, "SECRET") {
		t.Fatalf("Authorization leaked secret: %q", gotAuth)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read SSE body: %v", err)
	}
	if !strings.Contains(string(body), `data: {"type":"content_block_delta","delta":{"text":"hi"}}`) {
		t.Fatalf("SSE body=%s", string(body))
	}
	if !strings.Contains(string(body), "data: [DONE]") {
		t.Fatalf("SSE body missing DONE: %s", string(body))
	}
}

func writeBedrockEventStreamMessage(t *testing.T, w io.Writer, payload []byte) {
	t.Helper()
	err := eventstream.NewEncoder().Encode(w, eventstream.Message{
		Headers: eventstream.Headers{
			{Name: eventstreamapi.MessageTypeHeader, Value: eventstream.StringValue(eventstreamapi.EventMessageType)},
			{Name: eventstreamapi.EventTypeHeader, Value: eventstream.StringValue("chunk")},
			{Name: eventstreamapi.ContentTypeHeader, Value: eventstream.StringValue("application/json")},
		},
		Payload: []byte(`{"bytes":"` + base64.StdEncoding.EncodeToString(payload) + `"}`),
	})
	if err != nil {
		t.Fatalf("encode eventstream message: %v", err)
	}
}
