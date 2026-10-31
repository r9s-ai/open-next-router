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

type responseMapFn func(map[string]any) (map[string]any, error)

var responseMapModes map[string]responseMapFn

func init() {
	geminiChat := MapOpenAIChatCompletionsToGeminiGenerateContentResponseObject
	responseMapModes = map[string]responseMapFn{
		"openai_responses_to_openai_chat":        MapOpenAIResponsesToChatCompletionsObject,
		"anthropic_to_openai_chat":               MapClaudeMessagesResponseToOpenAIChatCompletionsObject,
		"gemini_to_openai_chat":                  MapGeminiGenerateContentToOpenAIChatCompletionsResponseObject,
		"gemini_to_openai_images":                MapGeminiGenerateContentToOpenAIImagesResponseObject,
		"minimax_image_to_openai_images":         MapMinimaxImageToOpenAIImagesResponseObject,
		"qwen_image_to_openai_images":            MapQwenImageToOpenAIImagesResponseObject,
		"ppio_video_create_to_openai_videos":     MapPPIOVideoCreateToOpenAIVideoObject,
		"ppio_video_result_to_openai_videos":     MapPPIOVideoResultToOpenAIVideoObject,
		"seedance_video_create_to_openai_videos": MapSeedanceVideoCreateToOpenAIVideoObject,
		"seedance_video_result_to_openai_videos": MapSeedanceVideoResultToOpenAIVideoObject,
		"minimax_video_create_to_openai_videos":  MapMinimaxVideoCreateToOpenAIVideoObject,
		"minimax_video_result_to_openai_videos":  MapMinimaxVideoResultToOpenAIVideoObject,
		"openai_to_anthropic_messages":           MapOpenAIChatCompletionsToClaudeMessagesResponseObject,
		"openai_to_gemini_chat":                  geminiChat,
		"openai_to_gemini_generate_content":      geminiChat,
	}
}

// SupportsResponseMapMode reports whether onr-core has a shared non-stream
// resp_map transform for the given mode.
func SupportsResponseMapMode(mode string) bool {
	_, ok := responseMapModes[NormalizeResponseMapMode(mode)]
	return ok
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
	fn, ok := responseMapModes[NormalizeResponseMapMode(mode)]
	if !ok {
		return nil, unsupportedModeError("resp_map", mode)
	}
	return fn(root)
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
