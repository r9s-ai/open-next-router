package onrserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
