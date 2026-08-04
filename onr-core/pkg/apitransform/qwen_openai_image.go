package apitransform

import (
	"net/http"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Qwen image generation mapping (DashScope multimodal-generation). These
// builtins replicate the relay Go adaptor (internal/channel/adaptor/ali) so
// OpenAI-compatible image generation requests can be routed to DashScope
// through the DSL config-file pipeline instead of a provider-specific Go
// adaptor.
//
// DashScope wraps the prompt in a chat-shaped envelope and puts every knob
// under parameters, spells sizes with a star, and answers with the image URL
// nested under output.choices[].message.content[].image. It never returns
// inline image content, so a caller asking for b64_json is served by the
// resp_inline_url response rule fetching the link.
//
// Generic bounds — prompt presence and length, n range, seed range — are left
// to the request validation directives, which already express them and report
// the failing path.

// qwenImageDefaultSize is what the Go adaptor sends when the caller omits size.
const qwenImageDefaultSize = "1328*1328"

// qwenImageNormalizeSize converts an OpenAI size to DashScope's spelling.
// DashScope writes dimensions as "<width>*<height>"; OpenAI uses "x". The Go
// adaptor only replaces a lowercase "x", so " 1024X1024 " reaches DashScope
// unconverted and is rejected there; normalizing first is a deliberate fix.
func qwenImageNormalizeSize(size string) string {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(size)), " ", "")
	if normalized == "" {
		return qwenImageDefaultSize
	}
	return strings.ReplaceAll(normalized, "x", "*")
}

// MapOpenAIImagesToQwenImageRequest builds a DashScope multimodal-generation
// request from an OpenAI images.generations request root, mirroring the relay Go
// adaptor's convertImageRequest: the prompt becomes a single user message, size
// is respelled, and n defaults to 1 with prompt_extend on and watermark off.
func MapOpenAIImagesToQwenImageRequest(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	parameters := apitypes.JSONObject{
		"size": qwenImageNormalizeSize(jsonutil.CoerceString(root["size"])),
	}

	// An omitted n means "one image" in the OpenAI API. An explicit out-of-range
	// n is rejected earlier by req_range; the test below only covers the absent
	// field, which decodes to 0.
	n := jsonutil.CoerceInt(root["n"])
	if n <= 0 {
		n = 1
	}
	parameters["n"] = n

	if negative := strings.TrimSpace(jsonutil.CoerceString(root["negative_prompt"])); negative != "" {
		parameters["negative_prompt"] = negative
	}
	// Both flags carry a meaningful default that differs from Go's zero value,
	// so an absent field must not be read as false.
	parameters["prompt_extend"] = boolOrDefault(root["prompt_extend"], true)
	parameters["watermark"] = boolOrDefault(root["watermark"], false)
	if seed, ok := jsonutil.CoerceIntOK(root["seed"]); ok {
		parameters["seed"] = seed
	}

	return apitypes.JSONObject{
		"model": jsonutil.CoerceString(root["model"]),
		"input": apitypes.JSONObject{
			"messages": []any{
				apitypes.JSONObject{
					"role":    "user",
					"content": []any{apitypes.JSONObject{"text": jsonutil.CoerceString(root["prompt"])}},
				},
			},
		},
		"parameters": parameters,
	}, nil
}

// boolOrDefault reads a JSON bool, falling back when the field is absent or of
// another type.
func boolOrDefault(value any, fallback bool) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

// MapQwenImageToOpenAIImagesResponseObject converts a DashScope
// multimodal-generation response into an OpenAI images response object.
//
// DashScope reports business failures inside an HTTP 200 by setting a top-level
// code, which the Go adaptor's parser treats as an upstream error; that is
// replicated here. A response carrying no image is an error too, rather than a
// success body with an empty data array that bills the caller for nothing they
// can detect.
func MapQwenImageToOpenAIImagesResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	if err := qwenImageBusinessError(root); err != nil {
		return nil, err
	}

	data := make([]any, 0, 2)
	output, _ := root["output"].(map[string]any)
	choices, _ := output["choices"].([]any)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		contents, _ := message["content"].([]any)
		for _, rawContent := range contents {
			content, _ := rawContent.(map[string]any)
			if url := strings.TrimSpace(jsonutil.CoerceString(content["image"])); url != "" {
				data = append(data, apitypes.JSONObject{"url": url})
			}
		}
	}

	if len(data) == 0 {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusInternalServerError,
			Type:       "server_error",
			Code:       "upstream_no_image",
			Message:    "No image data found in response",
		}
	}

	return apitypes.JSONObject{
		"created": time.Now().Unix(),
		"data":    data,
	}, nil
}

// qwenImageBusinessError reports a DashScope failure delivered inside an HTTP
// 200. Only a present, non-empty code counts.
func qwenImageBusinessError(root apitypes.JSONObject) error {
	code := strings.TrimSpace(jsonutil.CoerceString(root["code"]))
	if code == "" {
		return nil
	}
	message := strings.TrimSpace(jsonutil.CoerceString(root["message"]))
	if message == "" {
		message = "qwen returned code " + code
	}
	return &UpstreamResponseError{
		// The Go adaptor surfaces these as upstream errors carrying DashScope's
		// own code and message; keeping both makes the cause visible downstream.
		StatusCode: http.StatusBadRequest,
		Type:       "invalid_request_error",
		Code:       "qwen_" + code,
		Message:    message,
	}
}
