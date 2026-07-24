package apitransform

import (
	"encoding/json"
	"net/http"
	"strings"
)

const contentTypeJSON = "application/json"

// NormalizeResponseMapMode canonicalizes resp_map mode names so runtimes can
// share one supported-mode registry.
func NormalizeResponseMapMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

// SupportsResponseMapMode reports whether onr-core has a shared non-stream
// resp_map transform for the given mode.
func SupportsResponseMapMode(mode string) bool {
	switch NormalizeResponseMapMode(mode) {
	case "openai_responses_to_openai_chat",
		"anthropic_to_openai_chat",
		"gemini_to_openai_chat",
		"gemini_to_openai_images",
		"openai_to_anthropic_messages",
		"openai_to_gemini_chat",
		"openai_to_gemini_generate_content":
		return true
	default:
		return false
	}
}

// MapResponseBodyByMode runs the shared non-stream resp_map transform selected
// by mode and returns the transformed response object plus its downstream
// content type.
func MapResponseBodyByMode(mode string, body []byte) (map[string]any, string, error) {
	root, err := unmarshalResponseBodyObject(body)
	if err != nil {
		return nil, "", err
	}
	out, err := MapResponseObjectByMode(mode, root)
	if err != nil {
		return nil, "", err
	}
	return out, contentTypeJSON, nil
}

func MapResponseObjectByMode(mode string, root map[string]any) (map[string]any, error) {
	switch NormalizeResponseMapMode(mode) {
	case "openai_responses_to_openai_chat":
		return MapOpenAIResponsesToChatCompletionsObject(root)
	case "anthropic_to_openai_chat":
		return MapClaudeMessagesResponseToOpenAIChatCompletionsObject(root)
	case "gemini_to_openai_chat":
		return MapGeminiGenerateContentToOpenAIChatCompletionsResponseObject(root)
	case "gemini_to_openai_images":
		return MapGeminiGenerateContentToOpenAIImagesResponseObject(root)
	case "openai_to_anthropic_messages":
		return MapOpenAIChatCompletionsToClaudeMessagesResponseObject(root)
	case "openai_to_gemini_chat", "openai_to_gemini_generate_content":
		return MapOpenAIChatCompletionsToGeminiGenerateContentResponseObject(root)
	default:
		return nil, unsupportedModeError("resp_map", mode)
	}
}

func unmarshalResponseBodyObject(body []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	return root, nil
}

// TransformNonStreamResponseBody applies the shared non-stream resp_map flow:
// skip on upstream errors, dispatch by mode, and return whether a transform
// was actually applied.
func TransformNonStreamResponseBody(
	statusCode int,
	mode string,
	body map[string]any,
	contentType string,
) (map[string]any, string, bool, error) {
	if statusCode >= http.StatusBadRequest {
		return nil, contentType, false, nil
	}
	if !SupportsResponseMapMode(mode) {
		return nil, contentType, false, nil
	}
	outObj, err := MapResponseObjectByMode(mode, body)
	if err != nil {
		return nil, "", false, err
	}
	return outObj, contentTypeJSON, true, nil
}
