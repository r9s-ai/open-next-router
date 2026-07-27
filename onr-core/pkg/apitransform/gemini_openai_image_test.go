package apitransform

import (
	"errors"
	"net/http"
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
		// 大小写归一化后再比较,挡住 Go 侧会静默放行的 "URL"(客户端要 url 却收到 b64_json)。
		{"url_uppercase_rejected", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "response_format": "URL"}, true},
		{"unknown_format_passes", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "response_format": "webp"}, false},
		{"quality_uppercase_ok", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "quality": "HD"}, false},
		// prompt / n 对齐 Go validateGeminiImagePrompt / validateGeminiImageCount。
		{"prompt_missing", apitypes.JSONObject{"model": "gemini-3-pro-image"}, true},
		{"prompt_blank", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "   "}, true},
		{"n_gt_1_rejected", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "n": float64(4)}, true},
		{"n_1_ok", apitypes.JSONObject{"model": "gemini-3-pro-image", "prompt": "x", "n": float64(1)}, false},
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

// 无图输出必须报错而不是回 200 空 data,否则客户端拿不到失败信号却照样被计费。
// 两种成因分别对齐 Go handler 的 "No candidates returned" / "No image data found in response"。
func TestMapGeminiGenerateContentToOpenAIImagesResponse_NoImage(t *testing.T) {
	cases := []struct {
		name     string
		root     apitypes.JSONObject
		wantCode string
	}{
		{
			name:     "no_candidates",
			root:     apitypes.JSONObject{"promptFeedback": map[string]any{"blockReason": "SAFETY"}},
			wantCode: "upstream_no_candidates",
		},
		{
			name: "candidates_without_inline_data",
			root: apitypes.JSONObject{
				"candidates": []any{
					map[string]any{"finishReason": "IMAGE_SAFETY", "content": map[string]any{"parts": []any{map[string]any{"text": "sorry"}}}},
				},
			},
			wantCode: "upstream_no_image",
		},
	}
	for _, tc := range cases {
		out, err := MapGeminiGenerateContentToOpenAIImagesResponseObject(tc.root)
		if out != nil {
			t.Fatalf("%s: expected no body, got %#v", tc.name, out)
		}
		var uerr *UpstreamResponseError
		if !errors.As(err, &uerr) {
			t.Fatalf("%s: expected *UpstreamResponseError, got %v", tc.name, err)
		}
		if uerr.Code != tc.wantCode {
			t.Fatalf("%s: code got %q want %q", tc.name, uerr.Code, tc.wantCode)
		}
		if uerr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("%s: status got %d want 500", tc.name, uerr.StatusCode)
		}
	}
}

// 计费维度:completion 用 Go 口径(total-prompt,含 thoughts),IMAGE modality 明细
// 必须落到 output_tokens_details.image_tokens —— metrics 在 resp_map 之后提取,
// 没搬过来的字段 usage_fact 就再也读不到。
func TestMapGeminiGenerateContentToOpenAIImagesResponse_UsageDetails(t *testing.T) {
	root := apitypes.JSONObject{
		"candidates": []any{
			map[string]any{"content": map[string]any{"parts": []any{
				map[string]any{"inlineData": map[string]any{"data": "AAAA"}},
			}}},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     float64(12),
			"candidatesTokenCount": float64(1290),
			"thoughtsTokenCount":   float64(64),
			"totalTokenCount":      float64(1366),
			"candidatesTokensDetails": []any{
				map[string]any{"modality": "IMAGE", "tokenCount": float64(1290)},
			},
		},
	}
	out, err := MapGeminiGenerateContentToOpenAIImagesResponseObject(root)
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	usage, _ := out["usage"].(apitypes.JSONObject)
	if usage == nil {
		t.Fatalf("missing usage: %#v", out)
	}
	// Go 口径:1366-12=1354,而不是 candidatesTokenCount 的 1290。
	if usage["completion_tokens"] != 1354 {
		t.Fatalf("completion_tokens got %v want 1354", usage["completion_tokens"])
	}
	details, _ := usage["output_tokens_details"].(apitypes.JSONObject)
	if details == nil || details["image_tokens"] != 1290 {
		t.Fatalf("output_tokens_details got %#v", usage["output_tokens_details"])
	}
	if details["reasoning_tokens"] != 64 {
		t.Fatalf("reasoning_tokens got %v want 64", details["reasoning_tokens"])
	}
}
