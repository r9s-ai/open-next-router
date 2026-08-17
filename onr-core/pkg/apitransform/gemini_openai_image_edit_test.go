package apitransform

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

// inlineFile builds the shape req_inline_file writes into the request root.
func inlineFile(contentType, b64 string) map[string]any {
	return map[string]any{"filename": "f.png", "content_type": contentType, "b64": b64}
}

func editRoot(extra map[string]any) apitypes.JSONObject {
	root := apitypes.JSONObject{
		"model":  "gemini-3-pro-image",
		"prompt": "make the sky blue",
		"image":  []any{inlineFile("image/png", "SU1H")},
	}
	for k, v := range extra {
		root[k] = v
	}
	return root
}

// The Go adaptor emits source images, then mask plus its instruction, then the
// prompt. A multimodal model reads parts in order, so reordering them changes
// the edit — this pins the order rather than just the set of parts.
func TestMapOpenAIImagesEditsToGemini_PartOrderMatchesGoAdaptor(t *testing.T) {
	req, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(editRoot(map[string]any{
		"image": []any{inlineFile("image/png", "SU1H"), inlineFile("image/jpeg", "Sk1H")},
		"mask":  []any{inlineFile("image/png", "TUFTSw==")},
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(req.Contents) != 1 || req.Contents[0].Role != "user" {
		t.Fatalf("contents=%+v want a single user content", req.Contents)
	}

	parts := req.Contents[0].Parts
	if len(parts) != 5 {
		t.Fatalf("parts=%d want 5 (2 images + mask + mask instruction + prompt)", len(parts))
	}
	for i, want := range []struct {
		mime string
		data string
	}{{"image/png", "SU1H"}, {"image/jpeg", "Sk1H"}, {"image/png", "TUFTSw=="}} {
		if parts[i].InlineData == nil {
			t.Fatalf("parts[%d] is not inline data: %+v", i, parts[i])
		}
		if parts[i].InlineData.MimeType != want.mime || parts[i].InlineData.Data != want.data {
			t.Fatalf("parts[%d]=%+v want %+v", i, *parts[i].InlineData, want)
		}
	}
	if parts[3].Text != geminiImageEditMaskInstruction {
		t.Fatalf("parts[3]=%q want the mask instruction", parts[3].Text)
	}
	if parts[4].Text != "make the sky blue" {
		t.Fatalf("parts[4]=%q want the prompt last", parts[4].Text)
	}
}

// Without a mask there is no mask instruction: the sentence describes an image
// that was never sent, and would push the model toward an edit nobody asked for.
func TestMapOpenAIImagesEditsToGemini_NoMaskNoMaskInstruction(t *testing.T) {
	req, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(editRoot(nil))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	parts := req.Contents[0].Parts
	if len(parts) != 2 {
		t.Fatalf("parts=%d want 2 (image + prompt)", len(parts))
	}
	for _, part := range parts {
		if part.Text == geminiImageEditMaskInstruction {
			t.Fatal("mask instruction present without a mask")
		}
	}
}

func TestMapOpenAIImagesEditsToGemini_MissingImageIsRejected(t *testing.T) {
	for name, root := range map[string]apitypes.JSONObject{
		"absent": {"model": "gemini-3-pro-image", "prompt": "p"},
		"empty":  {"model": "gemini-3-pro-image", "prompt": "p", "image": []any{}},
		// A part that arrived with no bytes is dropped while reading, which
		// must land on the same rejection rather than an empty inlineData part
		// that Gemini answers with an opaque error.
		"blank": {"model": "gemini-3-pro-image", "prompt": "p", "image": []any{inlineFile("image/png", "")}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(root)
			var mapErr *RequestMappingError
			if !errors.As(err, &mapErr) {
				t.Fatalf("err=%v want *RequestMappingError", err)
			}
			if mapErr.Code != CodeRequestMissingRequiredField || mapErr.Param != "image" {
				t.Fatalf("code=%q param=%q want %q/image", mapErr.Code, mapErr.Param, CodeRequestMissingRequiredField)
			}
		})
	}
}

// Edits share the generation validator, so the model-conditional rules apply
// identically on both routes.
func TestMapOpenAIImagesEditsToGemini_SharesGenerationValidation(t *testing.T) {
	cases := map[string]struct {
		root  apitypes.JSONObject
		code  string
		param string
	}{
		"prompt required": {editRoot(map[string]any{"prompt": "   "}), CodeRequestPromptMissing, "prompt"},
		"n above one":     {editRoot(map[string]any{"n": 2}), CodeRequestNOutOfRange, "n"},
		"url unsupported": {editRoot(map[string]any{"response_format": "url"}), CodeRequestInvalidParameter, "response_format"},
		"size below 3.0": {editRoot(map[string]any{
			"model": "gemini-2.5-flash-image", "size": "1024x1024",
		}), CodeRequestSizeNotSupported, "size"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(tc.root)
			var mapErr *RequestMappingError
			if !errors.As(err, &mapErr) {
				t.Fatalf("err=%v want *RequestMappingError", err)
			}
			if mapErr.Code != tc.code || mapErr.Param != tc.param {
				t.Fatalf("code=%q param=%q want %q/%q", mapErr.Code, mapErr.Param, tc.code, tc.param)
			}
		})
	}
}

// The image check runs before the option checks: a request with no image is
// missing the one thing an edit cannot proceed without, and reporting a size
// complaint first would send the caller after the wrong problem.
func TestMapOpenAIImagesEditsToGemini_MissingImageReportedBeforeOptions(t *testing.T) {
	_, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(apitypes.JSONObject{
		"model": "gemini-2.5-flash-image",
		"size":  "1024x1024",
	})
	var mapErr *RequestMappingError
	if !errors.As(err, &mapErr) || mapErr.Param != "image" {
		t.Fatalf("err=%v want the image rejection", err)
	}
}

func TestMapOpenAIImagesEditsToGemini_Gemini3CarriesImageConfig(t *testing.T) {
	req, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(editRoot(map[string]any{
		"size": "16:9", "quality": "hd", "n": 1,
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if req.GenerationConfig.ImageConfig == nil {
		t.Fatal("gemini-3 edit lost its imageConfig")
	}
	if got := req.GenerationConfig.ImageConfig.AspectRatio; got != "16:9" {
		t.Fatalf("aspectRatio=%q want 16:9", got)
	}
	// gemini-3-pro-image is the 4K-capable model, so hd resolves to 4K.
	if got := req.GenerationConfig.ImageConfig.ImageSize; got != "4K" {
		t.Fatalf("imageSize=%q want 4K", got)
	}
	if got := req.GenerationConfig.CandidateCount; got != 1 {
		t.Fatalf("candidateCount=%d want 1", got)
	}
}

// Models below 3.0 take no imageConfig, matching the Go adaptor.
func TestMapOpenAIImagesEditsToGemini_PreGemini3OmitsImageConfig(t *testing.T) {
	req, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(apitypes.JSONObject{
		"model":  "gemini-2.5-flash-image",
		"prompt": "p",
		"image":  []any{inlineFile("image/png", "SU1H")},
	})
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if req.GenerationConfig.ImageConfig != nil {
		t.Fatalf("imageConfig=%+v want none below 3.0", req.GenerationConfig.ImageConfig)
	}
	if len(req.GenerationConfig.ResponseModalities) != 0 {
		t.Fatalf("responseModalities=%v want none below 3.0", req.GenerationConfig.ResponseModalities)
	}
}

// The wire form is what Gemini actually receives, so pin the JSON field names.
func TestMapOpenAIImagesEditsToGemini_WireShape(t *testing.T) {
	req, err := MapOpenAIImagesEditsToGeminiGenerateContentRequest(editRoot(nil))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	contents, _ := got["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents=%v", got["contents"])
	}
	first, _ := contents[0].(map[string]any)
	parts, _ := first["parts"].([]any)
	inline, _ := parts[0].(map[string]any)["inlineData"].(map[string]any)
	if inline["mimeType"] != "image/png" || inline["data"] != "SU1H" {
		t.Fatalf("inlineData=%v want mimeType/data on the wire (%s)", inline, encoded)
	}
}
