package onrserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestvalidate"
)

func TestWriteProxyError_RequestValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	verr := &requestvalidate.RequestValidationError{
		Source:     "body",
		PathOrName: "$.messages",
		Rule:       "required",
		Message:    "$.messages is required",
	}
	writeProxyError(gc, "", verr)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "request_validation_failed" {
		t.Fatalf("unexpected code: %#v", errObj["code"])
	}
	if errObj["param"] != "$.messages" {
		t.Fatalf("unexpected param: %#v", errObj["param"])
	}
	if errObj["type"] != openAIInvalidRequestType {
		t.Fatalf("unexpected type: %#v", errObj["type"])
	}
}

// req_map 校验被拒时,客户端应拿到 builtin 给出的具体 code 与 param,
// 而不是被折叠成通用的 proxy_error。code 取值与 relay Go 侧一致。
func TestWriteProxyError_RequestMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	merr := &apitransform.RequestMappingError{
		Code:    apitransform.CodeRequestNOutOfRange,
		Param:   "n",
		Message: "Gemini image generation only supports n=1",
	}
	writeProxyError(gc, "", merr)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "request_n_out_of_range" {
		t.Fatalf("unexpected code: %#v", errObj["code"])
	}
	if errObj["param"] != "n" {
		t.Fatalf("unexpected param: %#v", errObj["param"])
	}
	if errObj["type"] != openAIInvalidRequestType {
		t.Fatalf("unexpected type: %#v", errObj["type"])
	}
	if errObj["message"] != "Gemini image generation only supports n=1" {
		t.Fatalf("unexpected message: %#v", errObj["message"])
	}
}

// 上游 200 但无可用负载时保留 5xx,不能被当成客户端错误。
func TestWriteProxyError_UpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	writeProxyError(gc, "", &apitransform.UpstreamResponseError{
		StatusCode: http.StatusInternalServerError,
		Type:       "server_error",
		Code:       "upstream_no_image",
		Message:    "No image data found in response",
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "upstream_no_image" {
		t.Fatalf("unexpected code: %#v", errObj["code"])
	}
	if errObj["type"] != "server_error" {
		t.Fatalf("unexpected type: %#v", errObj["type"])
	}
}

func TestWriteProxyError_GenericError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	writeProxyError(gc, "", errors.New("upstream exploded"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "proxy_error" {
		t.Fatalf("unexpected code: %#v", errObj["code"])
	}
	if _, hasParam := errObj["param"]; hasParam {
		t.Fatalf("generic proxy error must not carry param: %#v", errObj)
	}
}
