package apitransform

import (
	"errors"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

// --- MapOpenAIVideosToSeedanceRequest ---

func TestMapOpenAIVideosToSeedanceRequest_TextToVideo(t *testing.T) {
	out, err := MapOpenAIVideosToSeedanceRequest(apitypes.JSONObject{
		"prompt":  "a cat jumping",
		"seconds": "5",
		"size":    "1280x720",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, ok := out["content"].([]apitypes.JSONObject)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %v, want single-element array", out["content"])
	}
	if content[0]["type"] != "text" || content[0]["text"] != "a cat jumping" {
		t.Fatalf("content[0] = %v, want {type:text text:a cat jumping}", content[0])
	}
	if out["duration"] != 5 {
		t.Errorf("duration = %v, want 5", out["duration"])
	}
	if out["resolution"] != "1280x720" {
		t.Errorf("resolution = %v, want 1280x720", out["resolution"])
	}
}

func TestMapOpenAIVideosToSeedanceRequest_OmitsOptionalFields(t *testing.T) {
	out, err := MapOpenAIVideosToSeedanceRequest(apitypes.JSONObject{
		"prompt": "a dog running",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out["duration"]; ok {
		t.Error("duration should be absent when seconds not provided")
	}
	if _, ok := out["resolution"]; ok {
		t.Error("resolution should be absent when size not provided")
	}
}

func TestMapOpenAIVideosToSeedanceRequest_MissingPromptIsError(t *testing.T) {
	_, err := MapOpenAIVideosToSeedanceRequest(apitypes.JSONObject{"seconds": "5"})
	var reqErr *RequestMappingError
	if !errors.As(err, &reqErr) {
		t.Fatalf("err = %v, want *RequestMappingError", err)
	}
	if reqErr.Param != "prompt" {
		t.Errorf("Param = %q, want %q", reqErr.Param, "prompt")
	}
}

func TestMapOpenAIVideosToSeedanceRequest_NonNumericSecondsOmitsDuration(t *testing.T) {
	out, err := MapOpenAIVideosToSeedanceRequest(apitypes.JSONObject{
		"prompt":  "timelapse sunset",
		"seconds": "fast",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out["duration"]; ok {
		t.Error("duration should be absent when seconds is non-numeric")
	}
}

func TestMapSeedanceVideoCreate_ToVideoObject(t *testing.T) {
	out, err := MapSeedanceVideoCreateToOpenAIVideoObject(apitypes.JSONObject{"id": "task-abc123"})
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if out["id"] != "task-abc123" || out["object"] != "video" || out["status"] != "queued" {
		t.Fatalf("out=%v", out)
	}
	if out["progress"] != 0 {
		t.Fatalf("progress=%v want 0", out["progress"])
	}
}

func TestMapSeedanceVideoCreate_RejectsMissingID(t *testing.T) {
	_, err := MapSeedanceVideoCreateToOpenAIVideoObject(apitypes.JSONObject{})
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("err=%v want *UpstreamResponseError", err)
	}
}

func seedanceResult(status string, extra map[string]any) apitypes.JSONObject {
	root := apitypes.JSONObject{"id": "task-abc123", "status": status}
	for k, v := range extra {
		root[k] = v
	}
	return root
}

func TestMapSeedanceVideoResult_StatusMapping(t *testing.T) {
	for upstream, want := range map[string]string{
		"queued":    "queued",
		"running":   "in_progress",
		"failed":    "failed",
		"cancelled": "cancelled",
	} {
		out, err := MapSeedanceVideoResultToOpenAIVideoObject(seedanceResult(upstream, nil))
		if err != nil {
			t.Fatalf("%s: %v", upstream, err)
		}
		if out["status"] != want {
			t.Fatalf("%s -> %v want %v", upstream, out["status"], want)
		}
	}
}

func TestMapSeedanceVideoResult_UnknownStatusIsAnError(t *testing.T) {
	_, err := MapSeedanceVideoResultToOpenAIVideoObject(seedanceResult("pending", nil))
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("err=%v want *UpstreamResponseError", err)
	}
}

func TestMapSeedanceVideoResult_CompletedCarriesVideo(t *testing.T) {
	out, err := MapSeedanceVideoResultToOpenAIVideoObject(seedanceResult("succeeded", map[string]any{
		"content": map[string]any{"video_url": "https://example.invalid/v.mp4"},
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if out["video_url"] != "https://example.invalid/v.mp4" {
		t.Fatalf("video_url=%v", out["video_url"])
	}
	if out["progress"] != 100 {
		t.Fatalf("progress=%v want 100", out["progress"])
	}
	if out["status"] != "completed" {
		t.Fatalf("status=%v want completed", out["status"])
	}
}

func TestMapSeedanceVideoResult_CompletedWithoutVideoIsAnError(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"no content":    {},
		"null content":  {"content": nil},
		"blank url":     {"content": map[string]any{"video_url": "  "}},
		"missing field": {"content": map[string]any{}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MapSeedanceVideoResultToOpenAIVideoObject(seedanceResult("succeeded", extra))
			var upErr *UpstreamResponseError
			if !errors.As(err, &upErr) {
				t.Fatalf("err=%v want *UpstreamResponseError", err)
			}
		})
	}
}

func TestMapSeedanceVideoResult_RejectsMissingID(t *testing.T) {
	_, err := MapSeedanceVideoResultToOpenAIVideoObject(apitypes.JSONObject{"status": "running"})
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("err=%v want *UpstreamResponseError", err)
	}
}

func TestMapSeedanceVideoResult_FailedSurfacesError(t *testing.T) {
	out, err := MapSeedanceVideoResultToOpenAIVideoObject(seedanceResult("failed", map[string]any{
		"error": map[string]any{"message": "content policy violation"},
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	errObj, _ := out["error"].(apitypes.JSONObject)
	if errObj == nil || errObj["message"] != "content policy violation" {
		t.Fatalf("error=%v want the upstream message", out["error"])
	}
}
