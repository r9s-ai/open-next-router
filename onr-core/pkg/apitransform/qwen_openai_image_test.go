package apitransform

import (
	"errors"
	"net/http"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

func qwenParams(t *testing.T, out apitypes.JSONObject) apitypes.JSONObject {
	t.Helper()
	params, _ := out["parameters"].(apitypes.JSONObject)
	if params == nil {
		t.Fatalf("missing parameters: %#v", out)
	}
	return params
}

func TestMapOpenAIImagesToQwenImageRequest_Envelope(t *testing.T) {
	out, err := MapOpenAIImagesToQwenImageRequest(apitypes.JSONObject{
		"model":  "qwen-image",
		"prompt": "a red fox",
	})
	if err != nil {
		t.Fatalf("map request: %v", err)
	}
	if out["model"] != "qwen-image" {
		t.Fatalf("model got %v", out["model"])
	}
	// DashScope takes the prompt as a single user message, not a flat field.
	input, _ := out["input"].(apitypes.JSONObject)
	messages, _ := input["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages got %#v", input["messages"])
	}
	msg, _ := messages[0].(apitypes.JSONObject)
	if msg["role"] != "user" {
		t.Fatalf("role got %v", msg["role"])
	}
	content, _ := msg["content"].([]any)
	first, _ := content[0].(apitypes.JSONObject)
	if first["text"] != "a red fox" {
		t.Fatalf("prompt got %v", first["text"])
	}
	// OpenAI field names must not leak into the DashScope envelope.
	for _, k := range []string{"prompt", "size", "n"} {
		if _, exists := out[k]; exists {
			t.Fatalf("OpenAI field %q leaked to top level: %#v", k, out)
		}
	}
}

func TestMapOpenAIImagesToQwenImageRequest_Defaults(t *testing.T) {
	params := qwenParams(t, mustQwenRequest(t, apitypes.JSONObject{"model": "qwen-image", "prompt": "x"}))
	if params["size"] != qwenImageDefaultSize {
		t.Fatalf("size got %v want %v", params["size"], qwenImageDefaultSize)
	}
	if params["n"] != 1 {
		t.Fatalf("n got %v want 1", params["n"])
	}
	// Both flags default to something other than the Go zero value, so an
	// absent field must not read as false.
	if params["prompt_extend"] != true {
		t.Fatalf("prompt_extend got %v want true", params["prompt_extend"])
	}
	if params["watermark"] != false {
		t.Fatalf("watermark got %v want false", params["watermark"])
	}
	if _, exists := params["seed"]; exists {
		t.Fatalf("seed must stay absent when the caller omits it: %#v", params)
	}
}

func TestMapOpenAIImagesToQwenImageRequest_Size(t *testing.T) {
	cases := map[string]string{
		"1024x1024": "1024*1024",
		// The Go adaptor only replaces a lowercase x, so these reach DashScope
		// unconverted there and are rejected; normalizing is a deliberate fix.
		"1024X1024":     "1024*1024",
		"  1024 x 768 ": "1024*768",
		"1328*1328":     "1328*1328",
		"":              qwenImageDefaultSize,
	}
	for size, want := range cases {
		params := qwenParams(t, mustQwenRequest(t, apitypes.JSONObject{
			"model": "qwen-image", "prompt": "x", "size": size,
		}))
		if params["size"] != want {
			t.Fatalf("size %q -> %v want %v", size, params["size"], want)
		}
	}
}

func TestMapOpenAIImagesToQwenImageRequest_OptionalPassthrough(t *testing.T) {
	params := qwenParams(t, mustQwenRequest(t, apitypes.JSONObject{
		"model": "qwen-image", "prompt": "x",
		"negative_prompt": "blurry",
		"prompt_extend":   false,
		"watermark":       true,
		"seed":            float64(42),
	}))
	if params["negative_prompt"] != "blurry" {
		t.Fatalf("negative_prompt got %v", params["negative_prompt"])
	}
	if params["prompt_extend"] != false || params["watermark"] != true {
		t.Fatalf("explicit flags not honoured: %#v", params)
	}
	if params["seed"] != 42 {
		t.Fatalf("seed got %v", params["seed"])
	}
}

func mustQwenRequest(t *testing.T, root apitypes.JSONObject) apitypes.JSONObject {
	t.Helper()
	out, err := MapOpenAIImagesToQwenImageRequest(root)
	if err != nil {
		t.Fatalf("map request: %v", err)
	}
	return out
}

func TestMapQwenImageToOpenAIImagesResponse(t *testing.T) {
	out, err := MapQwenImageToOpenAIImagesResponseObject(apitypes.JSONObject{
		"output": map[string]any{
			"choices": []any{
				map[string]any{
					"finish_reason": "stop",
					"message": map[string]any{
						"content": []any{map[string]any{"image": "https://example.invalid/a.png"}},
					},
				},
			},
		},
		"usage":      map[string]any{"width": float64(1328), "height": float64(1328), "image_count": float64(1)},
		"request_id": "abc",
	})
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	data, _ := out["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data got %#v", out["data"])
	}
	item, _ := data[0].(apitypes.JSONObject)
	if item["url"] != "https://example.invalid/a.png" {
		t.Fatalf("url got %v", item["url"])
	}
	if _, ok := out["created"]; !ok {
		t.Fatalf("missing created")
	}
}

// DashScope reports failures inside an HTTP 200 by setting a top-level code.
func TestMapQwenImageToOpenAIImagesResponse_BusinessError(t *testing.T) {
	out, err := MapQwenImageToOpenAIImagesResponseObject(apitypes.JSONObject{
		"code":       "InvalidParameter",
		"message":    "size is invalid",
		"request_id": "abc",
	})
	if out != nil {
		t.Fatalf("expected no body, got %#v", out)
	}
	var uerr *UpstreamResponseError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected *UpstreamResponseError, got %v", err)
	}
	if uerr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status got %d want 400", uerr.StatusCode)
	}
	if uerr.Code != "qwen_InvalidParameter" {
		t.Fatalf("code got %q", uerr.Code)
	}
	// The upstream message is the only description of the cause.
	if uerr.Message != "size is invalid" {
		t.Fatalf("message got %q", uerr.Message)
	}
}

func TestMapQwenImageToOpenAIImagesResponse_NoImage(t *testing.T) {
	cases := map[string]apitypes.JSONObject{
		"no output":    {"request_id": "abc"},
		"no choices":   {"output": map[string]any{"choices": []any{}}},
		"empty image":  {"output": map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{map[string]any{"image": "  "}}}}}}},
		"text content": {"output": map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{map[string]any{"text": "sorry"}}}}}}},
	}
	for name, root := range cases {
		out, err := MapQwenImageToOpenAIImagesResponseObject(root)
		if out != nil {
			t.Fatalf("%s: expected no body, got %#v", name, out)
		}
		var uerr *UpstreamResponseError
		if !errors.As(err, &uerr) {
			t.Fatalf("%s: expected *UpstreamResponseError, got %v", name, err)
		}
		if uerr.Code != "upstream_no_image" || uerr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("%s: got code=%q status=%d", name, uerr.Code, uerr.StatusCode)
		}
	}
}
