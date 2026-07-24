package apitransform

import (
	"fmt"
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
// validateGeminiImageResponseFormat and validateGeminiImageModelOptions:
// response_format "url" is rejected (other values pass), and size/quality are
// model-conditional — gemini-3 accepts known aspect ratios and
// standard/hd quality, models below 3.0 accept neither.
func validateGeminiImageOptions(model, size, quality, responseFormat string) error {
	if responseFormat == "url" {
		return fmt.Errorf("response_format 'url' is not supported for this channel")
	}
	if geminiImageIsGemini3Model(model) {
		if size != "" {
			if _, ok := geminiImageAspectRatios[geminiImageNormalizeSize(size)]; !ok {
				return fmt.Errorf("invalid size/aspect_ratio for Gemini: %s. Supported: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 5:4, 4:5, 21:9 or equivalent pixel dimensions", size)
			}
		}
		if quality != "" && quality != "standard" && quality != "hd" {
			return fmt.Errorf("invalid quality: %s. Supported: standard, hd", quality)
		}
		return nil
	}
	if size != "" {
		return fmt.Errorf("size/aspect_ratio is not supported for Gemini models below 3.0")
	}
	if quality != "" {
		return fmt.Errorf("quality is not supported for Gemini models below 3.0")
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
	prompt := jsonutil.CoerceString(root["prompt"])
	size := strings.TrimSpace(jsonutil.CoerceString(root["size"]))
	quality := strings.TrimSpace(jsonutil.CoerceString(root["quality"]))
	responseFormat := strings.TrimSpace(jsonutil.CoerceString(root["response_format"]))
	if err := validateGeminiImageOptions(model, size, quality, responseFormat); err != nil {
		return nil, err
	}

	req := &apitypes.ChatRequest{
		Contents: []apitypes.ChatContent{
			{Parts: []apitypes.Part{{Text: prompt}}},
		},
	}
	if n := jsonutil.CoerceInt(root["n"]); n > 0 {
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
// response handling.
func MapGeminiGenerateContentToOpenAIImagesResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	data := make([]any, 0, 2)
	candidates, _ := root["candidates"].([]any)
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

	out := apitypes.JSONObject{
		"created": time.Now().Unix(),
		"data":    data,
	}
	if usageRaw, _ := root["usageMetadata"].(map[string]any); usageRaw != nil {
		if usage, err := mapGeminiUsageToOpenAI(usageRaw); err == nil && usage != nil {
			out["usage"] = usage
		}
	}
	return out, nil
}
