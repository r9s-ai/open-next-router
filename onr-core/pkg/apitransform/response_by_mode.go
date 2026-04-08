package apitransform

import (
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
		"openai_to_anthropic_messages",
		"openai_to_gemini_chat",
		"openai_to_gemini_generate_content":
		return true
	default:
		return false
	}
}

// MapResponseBodyByMode runs the shared non-stream resp_map transform selected
// by mode and returns the transformed body plus its downstream content type.
func MapResponseBodyByMode(mode string, body []byte) ([]byte, string, error) {
	switch NormalizeResponseMapMode(mode) {
	case "openai_responses_to_openai_chat":
		out, err := MapOpenAIResponsesToChatCompletions(body)
		return out, contentTypeJSON, err
	case "anthropic_to_openai_chat":
		out, err := MapClaudeMessagesResponseToOpenAIChatCompletions(body)
		return out, contentTypeJSON, err
	case "gemini_to_openai_chat":
		out, err := MapGeminiGenerateContentToOpenAIChatCompletionsResponse(body)
		return out, contentTypeJSON, err
	case "openai_to_anthropic_messages":
		out, err := MapOpenAIChatCompletionsToClaudeMessagesResponse(body)
		return out, contentTypeJSON, err
	case "openai_to_gemini_chat", "openai_to_gemini_generate_content":
		out, err := MapOpenAIChatCompletionsToGeminiGenerateContentResponse(body)
		return out, contentTypeJSON, err
	default:
		return nil, "", unsupportedModeError("resp_map", mode)
	}
}

// TransformNonStreamResponseBody applies the shared non-stream resp_map flow:
// skip on upstream errors, decode the upstream body when needed, dispatch by
// mode, and return whether a transform was actually applied.
func TransformNonStreamResponseBody(
	statusCode int,
	mode string,
	body []byte,
	contentType string,
	contentEncoding string,
) ([]byte, string, bool, error) {
	if statusCode >= http.StatusBadRequest {
		return body, contentType, false, nil
	}
	decoded, _, err := DecodeResponseBody(body, contentEncoding)
	if err != nil {
		return nil, "", false, err
	}
	if !SupportsResponseMapMode(mode) {
		return body, contentType, false, nil
	}
	out, outCT, err := MapResponseBodyByMode(mode, decoded)
	if err != nil {
		return nil, "", false, err
	}
	return out, outCT, true, nil
}
