package apitransform

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Gemini image editing mapping. Gemini has no dedicated edit endpoint: an edit
// is a generateContent call whose contents carry the source images inline
// alongside the instruction. This builtin replicates the relay Go adaptor's
// convertImageEditRequest (internal/channel/adaptor/gemini) so an OpenAI
// images.edits request can reach Gemini through the DSL pipeline.
//
// The response needs no counterpart: an edit response is an ordinary
// generateContent response, so gemini_to_openai_images handles it unchanged.

// geminiImageEditMaskInstruction is the sentence the Go adaptor appends after a
// mask so the model knows what the extra image means. Gemini has no mask
// parameter, so the meaning has to be carried in the prompt; the wording is
// copied verbatim to keep both routes producing the same edit.
const geminiImageEditMaskInstruction = "Use the provided mask to edit the image. " +
	"Areas with transparency in the mask indicate where edits should be applied."

// inlineFileEntry is one file placed into the request root by req_inline_file:
// {filename, content_type, b64}.
type inlineFileEntry struct {
	ContentType string
	Data        string
}

// inlineFilesAt reads the req_inline_file array a config wrote to root[field].
// Entries without base64 data are skipped rather than forwarded as empty inline
// parts, which Gemini rejects with an opaque error.
func inlineFilesAt(root apitypes.JSONObject, field string) []inlineFileEntry {
	raw, _ := root[field].([]any)
	if len(raw) == 0 {
		return nil
	}
	out := make([]inlineFileEntry, 0, len(raw))
	for _, item := range raw {
		entry, _ := item.(map[string]any)
		if entry == nil {
			continue
		}
		data := strings.TrimSpace(jsonutil.CoerceString(entry["b64"]))
		if data == "" {
			continue
		}
		contentType := strings.TrimSpace(jsonutil.CoerceString(entry["content_type"]))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		out = append(out, inlineFileEntry{ContentType: contentType, Data: data})
	}
	return out
}

// MapOpenAIImagesEditsToGeminiGenerateContentRequest builds a Gemini
// generateContent request from an OpenAI images.edits request root whose
// uploads have already been inlined by req_inline_file.
//
// Part order mirrors the Go adaptor exactly — source images, then the mask plus
// its instruction, then the prompt — because a multimodal model reads its
// parts in order and reordering them changes the edit.
func MapOpenAIImagesEditsToGeminiGenerateContentRequest(root apitypes.JSONObject) (*apitypes.ChatRequest, error) {
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))
	// Forwarded verbatim, validated trimmed: leading or trailing whitespace the
	// caller meant to send is preserved.
	prompt := jsonutil.CoerceString(root["prompt"])
	size := strings.TrimSpace(jsonutil.CoerceString(root["size"]))
	quality := strings.TrimSpace(jsonutil.CoerceString(root["quality"]))
	responseFormat := strings.TrimSpace(jsonutil.CoerceString(root["response_format"]))
	n := jsonutil.CoerceInt(root["n"])

	images := inlineFilesAt(root, "image")
	if len(images) == 0 {
		return nil, newRequestMappingError(CodeRequestMissingRequiredField, "image",
			"at least one image is required for image editing")
	}
	if err := validateGeminiImageOptions(model, strings.TrimSpace(prompt), size, quality, responseFormat, n); err != nil {
		return nil, err
	}

	parts := make([]apitypes.Part, 0, len(images)+3)
	for _, image := range images {
		parts = append(parts, apitypes.Part{
			InlineData: &apitypes.InlineData{MimeType: image.ContentType, Data: image.Data},
		})
	}
	// The Go adaptor accepts a single mask; extra mask files are ignored rather
	// than rejected, matching a client that sent one by mistake.
	if masks := inlineFilesAt(root, "mask"); len(masks) > 0 {
		parts = append(parts,
			apitypes.Part{InlineData: &apitypes.InlineData{MimeType: masks[0].ContentType, Data: masks[0].Data}},
			apitypes.Part{Text: geminiImageEditMaskInstruction},
		)
	}
	if prompt != "" {
		parts = append(parts, apitypes.Part{Text: prompt})
	}

	req := &apitypes.ChatRequest{
		Contents: []apitypes.ChatContent{{Role: "user", Parts: parts}},
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
