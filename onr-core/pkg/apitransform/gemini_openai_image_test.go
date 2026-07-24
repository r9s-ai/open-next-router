package apitransform

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

func TestMapOpenAIImagesToGeminiGenerateContentRequest_Gemini3(t *testing.T) {
	root := apitypes.JSONObject{
		"model":   "gemini-3-pro-image",
		"prompt":  "a red fox",
		"n":       float64(1),
		"size":    "1792x1024",
		"quality": "hd",
	}
	req, err := MapOpenAIImagesToGeminiGenerateContentRequest(root)
	if err != nil {
		t.Fatalf("map request: %v", err)
	}

	if len(req.Contents) != 1 || len(req.Contents[0].Parts) != 1 || req.Contents[0].Parts[0].Text != "a red fox" {
		t.Fatalf("prompt not mapped to contents[].parts[].text: %#v", req.Contents)
	}
	if req.GenerationConfig.CandidateCount != 1 {
		t.Fatalf("candidateCount got %d want 1", req.GenerationConfig.CandidateCount)
	}
	if got := req.GenerationConfig.ResponseModalities; len(got) != 2 || got[0] != "TEXT" || got[1] != "IMAGE" {
		t.Fatalf("responseModalities got %#v want [TEXT IMAGE]", got)
	}
	if req.GenerationConfig.ImageConfig == nil {
		t.Fatalf("imageConfig nil for gemini-3")
	}
	// 1792x1024 -> 16:9; hd + gemini-3-pro-image -> 4K
	if req.GenerationConfig.ImageConfig.AspectRatio != "16:9" {
		t.Fatalf("aspectRatio got %q want 16:9", req.GenerationConfig.ImageConfig.AspectRatio)
	}
	if req.GenerationConfig.ImageConfig.ImageSize != "4K" {
		t.Fatalf("imageSize got %q want 4K", req.GenerationConfig.ImageConfig.ImageSize)
	}

	// ToMap 产出 Gemini camelCase 键
	m, err := req.ToMap()
	if err != nil {
		t.Fatalf("ToMap: %v", err)
	}
	cfg, _ := m["generationConfig"].(map[string]any)
	if cfg == nil {
		t.Fatalf("expected generationConfig key, got %#v", m)
	}
	ic, _ := cfg["imageConfig"].(map[string]any)
	if ic == nil || ic["aspectRatio"] != "16:9" || ic["imageSize"] != "4K" {
		t.Fatalf("imageConfig json shape wrong: %#v", cfg["imageConfig"])
	}
}

func TestMapOpenAIImagesToGeminiGenerateContentRequest_NonGemini3(t *testing.T) {
	root := apitypes.JSONObject{
		"model":  "gemini-2.5-flash-image",
		"prompt": "a blue whale",
	}
	req, err := MapOpenAIImagesToGeminiGenerateContentRequest(root)
	if err != nil {
		t.Fatalf("map request: %v", err)
	}
	// 非 gemini-3:不注入 imageConfig / responseModalities
	if req.GenerationConfig.ImageConfig != nil {
		t.Fatalf("non-gemini3 should not set imageConfig, got %#v", req.GenerationConfig.ImageConfig)
	}
	if len(req.GenerationConfig.ResponseModalities) != 0 {
		t.Fatalf("non-gemini3 should not set responseModalities, got %#v", req.GenerationConfig.ResponseModalities)
	}
	// n 缺省不设置 candidateCount
	if req.GenerationConfig.CandidateCount != 0 {
		t.Fatalf("candidateCount got %d want 0", req.GenerationConfig.CandidateCount)
	}
}

// 校验语义逐条对齐 Go validateGeminiImageResponseFormat / validateGeminiImageModelOptions。
func TestMapOpenAIImagesToGeminiGenerateContentRequest_Validation(t *testing.T) {
	cases := []struct {
		name    string
		root    apitypes.JSONObject
		wantErr bool
	}{
		{"url_rejected", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "response_format": "url"}, true},
		{"unknown_format_passes", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "response_format": "webp"}, false},
		{"gemini3_bad_size", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "size": "999x999"}, true},
		{"gemini3_bad_quality", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "quality": "ultra"}, true},
		{"below3_size_rejected", apitypes.JSONObject{"model": "gemini-2.5-flash-image", "prompt": "x", "size": "1024x1024"}, true},
		{"below3_quality_rejected", apitypes.JSONObject{"model": "gemini-2.5-flash-image", "prompt": "x", "quality": "hd"}, true},
		{"below3_plain_ok", apitypes.JSONObject{"model": "gemini-2.5-flash-image", "prompt": "x"}, false},
	}
	for _, tc := range cases {
		if _, err := MapOpenAIImagesToGeminiGenerateContentRequest(tc.root); (err != nil) != tc.wantErr {
			t.Fatalf("%s: err=%v wantErr=%v", tc.name, err, tc.wantErr)
		}
	}
}

func TestGeminiImageSizeAndAspect(t *testing.T) {
	// hd + 非 pro-image -> 2K
	if got := geminiImageSize("gemini-3-flash-image", "hd"); got != "2K" {
		t.Fatalf("2K case got %q", got)
	}
	// 非 hd -> 1K
	if got := geminiImageSize("gemini-3-pro-image", "standard"); got != "1K" {
		t.Fatalf("1K case got %q", got)
	}
	// 直接比例透传
	if got := geminiImageAspectRatio("21:9"); got != "21:9" {
		t.Fatalf("aspect passthrough got %q", got)
	}
	// 未知 -> 1:1
	if got := geminiImageAspectRatio("weird"); got != "1:1" {
		t.Fatalf("aspect fallback got %q", got)
	}
	// 空 -> 1:1
	if got := geminiImageAspectRatio(""); got != "1:1" {
		t.Fatalf("aspect empty got %q", got)
	}
}

func TestMapGeminiGenerateContentToOpenAIImagesResponse(t *testing.T) {
	root := apitypes.JSONObject{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					// 逐 part 读 text 作为 revised_prompt(与 Go 一致):text 与 inlineData 同 part
					"parts": []any{
						map[string]any{"text": "a red fox running", "inlineData": map[string]any{"mimeType": "image/png", "data": "AAAAbase64"}},
					},
				},
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(12),
			"candidatesTokenCount": float64(1290),
			"totalTokenCount":      float64(1302),
		},
	}
	out, err := MapGeminiGenerateContentToOpenAIImagesResponseObject(root)
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	data, _ := out["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("data len got %d want 1 (%#v)", len(data), out["data"])
	}
	item, _ := data[0].(apitypes.JSONObject)
	if item["b64_json"] != "AAAAbase64" {
		t.Fatalf("b64_json got %v", item["b64_json"])
	}
	if item["revised_prompt"] != "a red fox running" {
		t.Fatalf("revised_prompt got %v", item["revised_prompt"])
	}
	usage, _ := out["usage"].(apitypes.JSONObject)
	if usage == nil || usage["prompt_tokens"] != 12 || usage["completion_tokens"] != 1290 || usage["total_tokens"] != 1302 {
		t.Fatalf("usage mapping wrong: %#v", out["usage"])
	}
	if _, ok := out["created"]; !ok {
		t.Fatalf("missing created")
	}
}

func TestMapGeminiGenerateContentToOpenAIImagesResponse_NoImage(t *testing.T) {
	// 无 inlineData:data 为空数组(运行时/conf 侧据此判错),不 panic
	root := apitypes.JSONObject{
		"candidates": []any{
			map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": "sorry"}}}},
		},
	}
	out, err := MapGeminiGenerateContentToOpenAIImagesResponseObject(root)
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	if data, _ := out["data"].([]any); len(data) != 0 {
		t.Fatalf("expected empty data, got %#v", out["data"])
	}
}
