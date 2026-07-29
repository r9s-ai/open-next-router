package apitransform

import (
	"errors"
	"net/http"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

func TestMapOpenAIImagesToMinimaxImageRequest_Defaults(t *testing.T) {
	out, err := MapOpenAIImagesToMinimaxImageRequest(apitypes.JSONObject{
		"model":  "image-01",
		"prompt": "a red fox",
	})
	if err != nil {
		t.Fatalf("map request: %v", err)
	}
	// 缺省:n=1、response_format=url、aspect_ratio=1:1(与 Go convertImageRequest 一致)
	if out["n"] != 1 {
		t.Fatalf("n got %v want 1", out["n"])
	}
	if out["response_format"] != "url" {
		t.Fatalf("response_format got %v want url", out["response_format"])
	}
	if out["aspect_ratio"] != "1:1" {
		t.Fatalf("aspect_ratio got %v want 1:1", out["aspect_ratio"])
	}
	// aspect_ratio 与 width/height 互斥,不能同时下发
	if _, ok := out["width"]; ok {
		t.Fatalf("width must be absent when aspect_ratio is set: %#v", out)
	}
}

func TestMapOpenAIImagesToMinimaxImageRequest_Size(t *testing.T) {
	cases := []struct {
		name       string
		size       string
		wantRatio  string
		wantWidth  int
		wantHeight int
	}{
		{"pixel_maps_to_ratio", "1280x720", "16:9", 0, 0},
		// Go 用原始串比较,这三种写法在 Go 侧会被拒;归一化后接受是有意修正。
		{"pixel_uppercase_x", "1280X720", "16:9", 0, 0},
		{"pixel_padded", "  1024 x 1024  ", "1:1", 0, 0},
		// aspect_ratio 是 minimax 原生参数,直接给比例应当可用。
		{"bare_ratio", "21:9", "21:9", 0, 0},
		{"free_dimensions", "1536x1024", "", 1536, 1024},
	}
	for _, tc := range cases {
		out, err := MapOpenAIImagesToMinimaxImageRequest(apitypes.JSONObject{
			"model": "image-01", "prompt": "x", "size": tc.size,
		})
		if err != nil {
			t.Fatalf("%s: map request: %v", tc.name, err)
		}
		if tc.wantRatio != "" {
			if out["aspect_ratio"] != tc.wantRatio {
				t.Fatalf("%s: aspect_ratio got %v want %v", tc.name, out["aspect_ratio"], tc.wantRatio)
			}
			continue
		}
		if out["width"] != tc.wantWidth || out["height"] != tc.wantHeight {
			t.Fatalf("%s: got %vx%v want %dx%d", tc.name, out["width"], out["height"], tc.wantWidth, tc.wantHeight)
		}
		if _, ok := out["aspect_ratio"]; ok {
			t.Fatalf("%s: aspect_ratio must be absent when width/height are set", tc.name)
		}
	}
}

func TestMapOpenAIImagesToMinimaxImageRequest_SizeRejected(t *testing.T) {
	cases := map[string]string{
		"not_a_size":    "huge",
		"below_min":     "256x256",
		"above_max":     "4096x4096",
		"not_multiple8": "1000x1001",
	}
	for name, size := range cases {
		_, err := MapOpenAIImagesToMinimaxImageRequest(apitypes.JSONObject{
			"model": "image-01", "prompt": "x", "size": size,
		})
		var merr *RequestMappingError
		if !errors.As(err, &merr) {
			t.Fatalf("%s: expected *RequestMappingError, got %v", name, err)
		}
		if merr.Code != CodeRequestSizeNotSupported || merr.Param != "size" {
			t.Fatalf("%s: got code=%q param=%q", name, merr.Code, merr.Param)
		}
	}
}

func TestMapOpenAIImagesToMinimaxImageRequest_ResponseFormatAndPassthrough(t *testing.T) {
	// b64_json 是 OpenAI 的叫法,minimax 叫 base64;大小写归一化后再比较,
	// 避免 Go 侧 "B64_JSON" 原样下发被上游拒的问题。
	for _, in := range []string{"b64_json", "B64_JSON", "base64"} {
		out, err := MapOpenAIImagesToMinimaxImageRequest(apitypes.JSONObject{
			"model": "image-01", "prompt": "x", "response_format": in,
		})
		if err != nil {
			t.Fatalf("%s: map request: %v", in, err)
		}
		if out["response_format"] != "base64" {
			t.Fatalf("%s: response_format got %v want base64", in, out["response_format"])
		}
	}

	out, err := MapOpenAIImagesToMinimaxImageRequest(apitypes.JSONObject{
		"model": "image-01", "prompt": "x",
		"seed": float64(42), "watermark": true,
	})
	if err != nil {
		t.Fatalf("map request: %v", err)
	}
	if out["seed"] != 42 {
		t.Fatalf("seed got %v want 42", out["seed"])
	}
	if out["aigc_watermark"] != true {
		t.Fatalf("aigc_watermark got %v want true", out["aigc_watermark"])
	}
}

func TestMapMinimaxImageToOpenAIImagesResponse(t *testing.T) {
	// minimax 的 data 是对象(两个平行数组),OpenAI 期望对象数组。
	out, err := MapMinimaxImageToOpenAIImagesResponseObject(apitypes.JSONObject{
		"id": "abc",
		"data": map[string]any{
			"image_urls": []any{"https://example.com/a.png", "https://example.com/b.png"},
		},
		"metadata":  map[string]any{"success_count": float64(2)},
		"base_resp": map[string]any{"status_code": float64(0), "status_msg": "success"},
	})
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	data, _ := out["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data len got %d want 2 (%#v)", len(data), out["data"])
	}
	first, _ := data[0].(apitypes.JSONObject)
	if first["url"] != "https://example.com/a.png" {
		t.Fatalf("url got %v", first["url"])
	}
	if _, ok := out["created"]; !ok {
		t.Fatalf("missing created")
	}
}

func TestMapMinimaxImageToOpenAIImagesResponse_Base64Preferred(t *testing.T) {
	out, err := MapMinimaxImageToOpenAIImagesResponseObject(apitypes.JSONObject{
		"data": map[string]any{
			"image_base64": []any{"AAAA"},
			"image_urls":   []any{"https://example.com/a.png"},
		},
	})
	if err != nil {
		t.Fatalf("map response: %v", err)
	}
	data, _ := out["data"].([]any)
	item, _ := data[0].(apitypes.JSONObject)
	if item["b64_json"] != "AAAA" {
		t.Fatalf("b64_json got %v", item["b64_json"])
	}
	if _, ok := item["url"]; ok {
		t.Fatalf("url must not be set alongside b64_json: %#v", item)
	}
}

// 没出图必须报错,而不是像 Go 的 ConvertToOpenAIFormat 那样回一个空 data。
func TestMapMinimaxImageToOpenAIImagesResponse_NoImage(t *testing.T) {
	cases := map[string]apitypes.JSONObject{
		"empty_arrays": {"data": map[string]any{"image_urls": []any{}, "image_base64": []any{}}},
		"missing_data": {"id": "abc"},
		"blank_string": {"data": map[string]any{"image_urls": []any{"  "}}},
	}
	for name, root := range cases {
		out, err := MapMinimaxImageToOpenAIImagesResponseObject(root)
		if out != nil {
			t.Fatalf("%s: expected no body, got %#v", name, out)
		}
		var uerr *UpstreamResponseError
		if !errors.As(err, &uerr) {
			t.Fatalf("%s: expected *UpstreamResponseError, got %v", name, err)
		}
		if uerr.StatusCode != http.StatusInternalServerError || uerr.Code != "upstream_no_image" {
			t.Fatalf("%s: got status=%d code=%q", name, uerr.StatusCode, uerr.Code)
		}
	}
}
