package apitransform

import (
	"net/http"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Gemini (Nano Banana) image generation mapping. These builtins replicate the
// relay Go adaptor (internal/channel/adaptor/gemini) so OpenAI-compatible image
// generation requests can be routed to Gemini generateContent through the DSL
// config-file pipeline instead of a provider-specific Go adaptor.

// geminiImageAspectRatios maps OpenAI size values (pixel dimensions or direct
// aspect ratios) to Gemini aspect ratios. Unknown values fall back to "1:1".
var geminiImageAspectRatios = map[string]string{
	"1024x1024": "1:1",
	"1792x1024": "16:9",
	"1024x1792": "9:16",
	"1152x864":  "4:3",
	"864x1152":  "3:4",
	"1248x832":  "3:2",
	"832x1248":  "2:3",
	"1280x1024": "5:4",
	"1024x1280": "4:5",
	"1344x576":  "21:9",
	"1:1":       "1:1",
	"2:3":       "2:3",
	"3:2":       "3:2",
	"3:4":       "3:4",
	"4:3":       "4:3",
	"4:5":       "4:5",
	"5:4":       "5:4",
	"9:16":      "9:16",
	"16:9":      "16:9",
	"21:9":      "21:9",
}

func geminiImageIsGemini3Model(model string) bool {
	return strings.HasPrefix(model, "gemini-3")
}

func geminiImageSupports4K(model string) bool {
	return strings.HasPrefix(model, "gemini-3-pro-image")
}

// geminiImageNormalizeSize lowercases, trims and removes spaces from an OpenAI
// size value so it can be looked up in geminiImageAspectRatios.
func geminiImageNormalizeSize(size string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(size)), " ", "")
}

// geminiImageAspectRatio normalizes an OpenAI size string to a Gemini aspect
// ratio, defaulting to "1:1" when empty or unrecognized.
func geminiImageAspectRatio(size string) string {
	if aspect, ok := geminiImageAspectRatios[geminiImageNormalizeSize(size)]; ok {
		return aspect
	}
	return "1:1"
}

// validateGeminiImageOptions replicates the relay Go adaptor's
// validateGeminiImagePrompt, validateGeminiImageCount,
// validateGeminiImageResponseFormat and validateGeminiImageModelOptions:
// prompt is required, n must be <= 1, response_format "url" is rejected (other
// values pass), and size/quality are model-conditional — gemini-3 accepts known
// aspect ratios and standard/hd quality, models below 3.0 accept neither.
//
// response_format and quality are compared case-insensitively. The Go adaptor
// compares them verbatim, which lets "URL" through and silently downgrades the
// client to b64_json; rejecting it here is a deliberate tightening.
// Rejections carry the Go adaptor's error code and the offending parameter, so
// a client sees the same code on both routes.
func validateGeminiImageOptions(model, prompt, size, quality, responseFormat string, n int) error {
	if prompt == "" {
		return newRequestMappingError(CodeRequestPromptMissing, "prompt", "prompt is required")
	}
	if n > 1 {
		return newRequestMappingError(CodeRequestNOutOfRange, "n", "Gemini image generation only supports n=1")
	}
	if strings.ToLower(responseFormat) == "url" {
		return newRequestMappingError(CodeRequestInvalidParameter, "response_format",
			"response_format 'url' is not supported for this channel")
	}
	normalizedQuality := strings.ToLower(quality)
	if geminiImageIsGemini3Model(model) {
		if size != "" {
			if _, ok := geminiImageAspectRatios[geminiImageNormalizeSize(size)]; !ok {
				return newRequestMappingError(CodeRequestSizeNotSupported, "size",
					"invalid size/aspect_ratio for Gemini: %s. Supported: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 5:4, 4:5, 21:9 or equivalent pixel dimensions", size)
			}
		}
		if normalizedQuality != "" && normalizedQuality != "standard" && normalizedQuality != "hd" {
			return newRequestMappingError(CodeRequestInvalidParameter, "quality",
				"invalid quality: %s. Supported: standard, hd", quality)
		}
		return nil
	}
	if size != "" {
		return newRequestMappingError(CodeRequestSizeNotSupported, "size",
			"size/aspect_ratio is not supported for Gemini models below 3.0")
	}
	if quality != "" {
		return newRequestMappingError(CodeRequestInvalidParameter, "quality",
			"quality is not supported for Gemini models below 3.0")
	}
	return nil
}

// geminiImageSize maps OpenAI quality to a Gemini image size: hd -> 4K (only
// gemini-3-pro-image) or 2K, otherwise 1K.
func geminiImageSize(model, quality string) string {
	if strings.ToLower(strings.TrimSpace(quality)) == "hd" {
		if geminiImageSupports4K(model) {
			return "4K"
		}
		return "2K"
	}
	return "1K"
}

// mapOpenAIImagesToGeminiGenerateContentRequest builds a Gemini generateContent
// request from an OpenAI images.generations request root. It mirrors the relay
// Go adaptor's convertImageRequest: prompt -> contents[].parts[].text,
// n -> candidateCount, and (gemini-3 only) size/quality -> imageConfig plus
// TEXT+IMAGE response modalities. Safety settings are left unset (matching the
// Go default when GeminiSafetySetting is empty); operators can inject them via
// json_ops when needed. It also replicates the adaptor's model-conditional
// validation of size/quality/response_format and returns an error on violation.
func MapOpenAIImagesToGeminiGenerateContentRequest(root apitypes.JSONObject) (*apitypes.ChatRequest, error) {
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))
	// The prompt is validated trimmed but forwarded verbatim, so leading or
	// trailing whitespace the caller intended is preserved.
	prompt := jsonutil.CoerceString(root["prompt"])
	size := strings.TrimSpace(jsonutil.CoerceString(root["size"]))
	quality := strings.TrimSpace(jsonutil.CoerceString(root["quality"]))
	responseFormat := strings.TrimSpace(jsonutil.CoerceString(root["response_format"]))
	n := jsonutil.CoerceInt(root["n"])
	if err := validateGeminiImageOptions(model, strings.TrimSpace(prompt), size, quality, responseFormat, n); err != nil {
		return nil, err
	}

	req := &apitypes.ChatRequest{
		Contents: []apitypes.ChatContent{
			{Parts: []apitypes.Part{{Text: prompt}}},
		},
	}
	if n > 0 {
		req.GenerationConfig.CandidateCount = n
	}
	if geminiImageIsGemini3Model(model) {
		req.GenerationConfig.ResponseModalities = []string{"TEXT", "IMAGE"}
		req.GenerationConfig.ImageConfig = &apitypes.ImageConfig{
			AspectRatio: geminiImageAspectRatio(size),
			ImageSize:   geminiImageSize(model, quality),
		}
	}
	return req, nil
}

// MapGeminiGenerateContentToOpenAIImagesResponseObject converts a Gemini
// generateContent response object into an OpenAI images response object:
// candidates[].content.parts[].inlineData.data -> data[].b64_json, plus a usage
// block derived from usageMetadata. It mirrors the relay Go adaptor's image
// response handling, including its two "no image produced" failure modes: a
// response with no candidates (typically a safety block) and a response whose
// candidates carry no inline image data both return an *UpstreamResponseError
// rather than a success body with an empty data array.
func MapGeminiGenerateContentToOpenAIImagesResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	candidates, _ := root["candidates"].([]any)
	if len(candidates) == 0 {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusInternalServerError,
			Type:       "server_error",
			Code:       "upstream_no_candidates",
			Message:    "No candidates returned",
		}
	}

	data := make([]any, 0, 2)
	for _, rawCandidate := range candidates {
		candidate, _ := rawCandidate.(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]any)
			inline, _ := part["inlineData"].(map[string]any)
			if inline == nil {
				continue
			}
			b64 := jsonutil.CoerceString(inline["data"])
			if b64 == "" {
				continue
			}
			item := apitypes.JSONObject{"b64_json": b64}
			if revised := strings.TrimSpace(jsonutil.CoerceString(part["text"])); revised != "" {
				item["revised_prompt"] = revised
			}
			data = append(data, item)
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

	out := apitypes.JSONObject{
		"created": time.Now().Unix(),
		"data":    data,
	}
	if usageRaw, _ := root["usageMetadata"].(map[string]any); usageRaw != nil {
		if usage, err := mapGeminiImageUsageToOpenAI(usageRaw); err == nil && usage != nil {
			out["usage"] = usage
		}
	}
	return out, nil
}

// mapGeminiImageUsageToOpenAI derives the OpenAI images usage block from Gemini
// usageMetadata. It deliberately does not reuse mapGeminiUsageToOpenAI:
//
//   - completion tokens follow the relay Go adaptor's image accounting
//     (totalTokenCount - promptTokenCount, floored at 0) rather than preferring
//     candidatesTokenCount, so DSL and Go routes bill the same amount for
//     gemini-3 responses that carry thoughts tokens.
//   - the IMAGE modality entry of candidatesTokensDetails is surfaced as
//     output_tokens_details.image_tokens. Metrics are extracted after resp_map,
//     so anything not copied here is unavailable to usage_fact rules; this is
//     the field the openai_images_generations usage_mode preset reads.
func mapGeminiImageUsageToOpenAI(raw map[string]any) (apitypes.JSONObject, error) {
	if raw == nil {
		return nil, nil
	}
	var usage apitypes.UsageMetadata
	if err := usage.FromMap(raw); err != nil {
		return nil, err
	}
	completionTokens := usage.TotalTokenCount - usage.PromptTokenCount
	if completionTokens < 0 {
		completionTokens = 0
	}
	out := apitypes.JSONObject{
		"prompt_tokens":     usage.PromptTokenCount,
		"completion_tokens": completionTokens,
		"total_tokens":      usage.TotalTokenCount,
	}
	details := apitypes.JSONObject{}
	if imageTokens := modalityTokenCount(usage.CandidatesTokensDetails, "IMAGE"); imageTokens > 0 {
		details["image_tokens"] = imageTokens
	}
	if usage.ThoughtsTokenCount > 0 {
		details["reasoning_tokens"] = usage.ThoughtsTokenCount
	}
	if len(details) > 0 {
		out["output_tokens_details"] = details
	}
	return out, nil
}

// modalityTokenCount returns the token count of the first entry matching the
// given modality, or 0 when absent.
func modalityTokenCount(details []apitypes.ModalityTokenCount, modality string) int {
	for _, detail := range details {
		if strings.EqualFold(detail.Modality, modality) {
			return detail.TokenCount
		}
	}
	return 0
}
