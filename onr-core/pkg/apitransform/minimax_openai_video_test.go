package apitransform

import (
	"errors"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

// --- minimaxTaskStatus ---

func TestMinimaxTaskStatus_KnownStatuses(t *testing.T) {
	for native, want := range map[string]string{
		"queued":    minimaxStatusQueued,
		"running":   minimaxStatusInProgress,
		"succeeded": minimaxStatusCompleted,
		"failed":    minimaxStatusFailed,
		"cancelled": minimaxStatusCancelled,
	} {
		got, err := minimaxTaskStatus(native)
		if err != nil {
			t.Fatalf("minimaxTaskStatus(%q): unexpected error: %v", native, err)
		}
		if got != want {
			t.Fatalf("minimaxTaskStatus(%q) = %q, want %q", native, got, want)
		}
	}
}

func TestMinimaxTaskStatus_UnknownReturnsError(t *testing.T) {
	_, err := minimaxTaskStatus("pending")
	if err == nil {
		t.Fatal("expected error for unknown status, got nil")
	}
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("error type = %T, want *UpstreamResponseError", err)
	}
	if upErr.Code != "upstream_unknown_task_status" {
		t.Fatalf("code = %q, want upstream_unknown_task_status", upErr.Code)
	}
}

// --- MapMinimaxVideoCreateToOpenAIVideoObject ---

func TestMapMinimaxVideoCreateToOpenAIVideoObject_Success(t *testing.T) {
	out, err := MapMinimaxVideoCreateToOpenAIVideoObject(apitypes.JSONObject{
		"task_id": "minimax-upstream-task-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "minimax-upstream-task-001" {
		t.Errorf("id = %v, want minimax-upstream-task-001", out["id"])
	}
	if out["status"] != minimaxStatusQueued {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusQueued)
	}
	if out["object"] != minimaxObjectVideo {
		t.Errorf("object = %v, want %q", out["object"], minimaxObjectVideo)
	}
	if out["progress"] != 0 {
		t.Errorf("progress = %v, want 0", out["progress"])
	}
}

func TestMapMinimaxVideoCreateToOpenAIVideoObject_MissingTaskID(t *testing.T) {
	_, err := MapMinimaxVideoCreateToOpenAIVideoObject(apitypes.JSONObject{})
	if err == nil {
		t.Fatal("expected error for missing task_id, got nil")
	}
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("error type = %T, want *UpstreamResponseError", err)
	}
	if upErr.Code != "upstream_missing_task_id" {
		t.Fatalf("code = %q, want upstream_missing_task_id", upErr.Code)
	}
}

// --- MapMinimaxVideoResultToOpenAIVideoObject ---

func TestMapMinimaxVideoResultToOpenAIVideoObject_Queued(t *testing.T) {
	// Matches get_queued.json shape (status=queued, no content).
	out, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":        "task-001",
			"status":    "queued",
			"task_type": "generation",
			"modality":  "video",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != minimaxStatusQueued {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusQueued)
	}
	if _, ok := out["video_url"]; ok {
		t.Error("video_url must be absent for queued task")
	}
	if _, ok := out["usage"]; ok {
		t.Error("usage must be absent for non-terminal task")
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_Running(t *testing.T) {
	// Matches get_running.json shape.
	out, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":        "task-001",
			"status":    "running",
			"task_type": "generation",
			"modality":  "video",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != minimaxStatusInProgress {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusInProgress)
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_Succeeded(t *testing.T) {
	// Matches get_succeeded.json shape (generation, video modality).
	out, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":         "task-gen-001",
			"status":     "succeeded",
			"task_type":  "generation",
			"modality":   "video",
			"resolution": "768P",
			"duration":   float64(5),
			"content":    map[string]any{"url": "https://cdn.example.com/video.mp4"},
			"usage": map[string]any{
				"total_seconds":     float64(10),
				"input_seconds":     float64(5),
				"output_seconds":    float64(5),
				"input_image_count": float64(2),
				"total_tokens":      float64(273890),
				"prompt_tokens":     float64(13500),
				"completion_tokens": float64(260390),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != minimaxStatusCompleted {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusCompleted)
	}
	if out["video_url"] != "https://cdn.example.com/video.mp4" {
		t.Errorf("video_url = %v, want the content URL", out["video_url"])
	}
	if out["resolution"] != "768P" {
		t.Errorf("resolution = %v, want 768P", out["resolution"])
	}
	if out["duration"] != 5 {
		t.Errorf("duration = %v, want 5", out["duration"])
	}
	if out["task_type"] != "generation" {
		t.Errorf("task_type = %v, want generation", out["task_type"])
	}
	if out["progress"] != 100 {
		t.Errorf("progress = %v, want 100", out["progress"])
	}
	usage, _ := out["usage"].(map[string]any)
	if usage == nil {
		t.Fatal("usage must be forwarded for completed task")
	}
	if usage["output_seconds"] != float64(5) {
		t.Errorf("usage.output_seconds = %v, want 5", usage["output_seconds"])
	}
	if usage["input_image_count"] != float64(2) {
		t.Errorf("usage.input_image_count = %v, want 2", usage["input_image_count"])
	}
	if usage["total_seconds"] != float64(10) {
		t.Errorf("usage.total_seconds = %v, want 10", usage["total_seconds"])
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_Regeneration(t *testing.T) {
	// Matches get_succeeded_regeneration.json shape (task_type=regeneration, 2K).
	out, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":         "task-regen-001",
			"status":     "succeeded",
			"task_type":  "regeneration",
			"modality":   "video",
			"resolution": "2K",
			"duration":   float64(5),
			"content":    map[string]any{"url": "https://cdn.example.com/regen.mp4"},
			"usage": map[string]any{
				"total_seconds":     float64(10),
				"input_seconds":     float64(5),
				"output_seconds":    float64(5),
				"input_image_count": float64(2),
				"total_tokens":      float64(97645),
				"prompt_tokens":     float64(0),
				"completion_tokens": float64(97645),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != minimaxStatusCompleted {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusCompleted)
	}
	if out["task_type"] != "regeneration" {
		t.Errorf("task_type = %v, want regeneration", out["task_type"])
	}
	if out["resolution"] != "2K" {
		t.Errorf("resolution = %v, want 2K", out["resolution"])
	}
	if out["video_url"] != "https://cdn.example.com/regen.mp4" {
		t.Errorf("video_url = %v, want regen URL", out["video_url"])
	}
	usage, _ := out["usage"].(map[string]any)
	if usage == nil {
		t.Fatal("usage must be forwarded for completed regeneration task")
	}
	if usage["input_seconds"] != float64(5) {
		t.Errorf("usage.input_seconds = %v, want 5", usage["input_seconds"])
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_H3Succeeded(t *testing.T) {
	// Matches get_succeeded_h3.json shape (task_type=h3_context_ir, modality=text).
	out, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":        "task-h3-001",
			"status":    "succeeded",
			"task_type": "h3_context_ir",
			"modality":  "text",
			"duration":  float64(5),
			"content":   map[string]any{"prompt": "enriched prompt text"},
			"usage": map[string]any{
				"total_tokens":      float64(6974),
				"prompt_tokens":     float64(6526),
				"completion_tokens": float64(448),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != minimaxStatusCompleted {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusCompleted)
	}
	if out["result_prompt"] != "enriched prompt text" {
		t.Errorf("result_prompt = %v, want enriched prompt text", out["result_prompt"])
	}
	if _, ok := out["video_url"]; ok {
		t.Error("video_url must be absent for H3 text-modality task")
	}
	usage, _ := out["usage"].(map[string]any)
	if usage == nil {
		t.Fatal("usage must be forwarded for completed H3 task")
	}
	if usage["completion_tokens"] != float64(448) {
		t.Errorf("usage.completion_tokens = %v, want 448", usage["completion_tokens"])
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_Failed(t *testing.T) {
	// Matches get_failed.json shape.
	out, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":        "task-fail-001",
			"status":    "failed",
			"task_type": "generation",
			"modality":  "video",
			"error":     map[string]any{"message": "Content policy violation: the prompt contains restricted material."},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status"] != minimaxStatusFailed {
		t.Errorf("status = %v, want %q", out["status"], minimaxStatusFailed)
	}
	errObj, _ := out["error"].(apitypes.JSONObject)
	if errObj == nil {
		t.Fatal("error object must be present for failed task")
	}
	if errObj["message"] != "Content policy violation: the prompt contains restricted material." {
		t.Errorf("error.message = %v", errObj["message"])
	}
	if _, ok := out["video_url"]; ok {
		t.Error("video_url must be absent for failed task")
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_MissingTaskObject(t *testing.T) {
	_, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{})
	if err == nil {
		t.Fatal("expected error for missing task object, got nil")
	}
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("error type = %T, want *UpstreamResponseError", err)
	}
	if upErr.Code != "upstream_missing_task" {
		t.Fatalf("code = %q, want upstream_missing_task", upErr.Code)
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_MissingTaskID(t *testing.T) {
	_, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{"status": "queued"},
	})
	if err == nil {
		t.Fatal("expected error for missing task.id, got nil")
	}
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("error type = %T, want *UpstreamResponseError", err)
	}
	if upErr.Code != "upstream_missing_task_id" {
		t.Fatalf("code = %q, want upstream_missing_task_id", upErr.Code)
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_UnknownStatus(t *testing.T) {
	_, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{"id": "task-001", "status": "pending"},
	})
	if err == nil {
		t.Fatal("expected error for unknown status, got nil")
	}
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("error type = %T, want *UpstreamResponseError", err)
	}
	if upErr.Code != "upstream_unknown_task_status" {
		t.Fatalf("code = %q, want upstream_unknown_task_status", upErr.Code)
	}
}

func TestMapMinimaxVideoResultToOpenAIVideoObject_SucceededMissingVideoURL(t *testing.T) {
	_, err := MapMinimaxVideoResultToOpenAIVideoObject(apitypes.JSONObject{
		"task": map[string]any{
			"id":        "task-001",
			"status":    "succeeded",
			"task_type": "generation",
			"modality":  "video",
			"content":   map[string]any{},
		},
	})
	if err == nil {
		t.Fatal("expected error for succeeded task with no content.url, got nil")
	}
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("error type = %T, want *UpstreamResponseError", err)
	}
	if upErr.Code != "upstream_missing_video" {
		t.Fatalf("code = %q, want upstream_missing_video", upErr.Code)
	}
}

// --- MapOpenAIVideosToMinimaxVideoRequest ---

func TestMapOpenAIVideosToMinimaxVideoRequest_Basic(t *testing.T) {
	out, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{
		"prompt": "a cat jumping",
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
	if _, ok := out["duration"]; ok {
		t.Error("duration must be absent when seconds not provided")
	}
	if _, ok := out["resolution"]; ok {
		t.Error("resolution must be absent when size not provided")
	}
}

func TestMapOpenAIVideosToMinimaxVideoRequest_WithSecondsAndSize(t *testing.T) {
	out, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{
		"prompt":  "ocean waves",
		"seconds": "6",
		"size":    "768P",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["duration"] != 6 {
		t.Errorf("duration = %v, want 6", out["duration"])
	}
	if out["resolution"] != "768P" {
		t.Errorf("resolution = %v, want 768P", out["resolution"])
	}
}

func TestMapOpenAIVideosToMinimaxVideoRequest_SizeNormalization(t *testing.T) {
	for size, wantRes := range map[string]string{
		"1920x1080": "768P",
		"854x480":   "480P",
		"2560x1440": "2K",
		"768P":      "768P",
		"2K":        "2K",
	} {
		out, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{
			"prompt": "test", "size": size,
		})
		if err != nil {
			t.Fatalf("size=%q: unexpected error: %v", size, err)
		}
		if out["resolution"] != wantRes {
			t.Errorf("size=%q: resolution = %v, want %q", size, out["resolution"], wantRes)
		}
	}
}

func TestMapOpenAIVideosToMinimaxVideoRequest_WithInputReference(t *testing.T) {
	out, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{
		"prompt": "a river in autumn",
		"input_reference": []any{map[string]any{
			"filename":     "ref.jpg",
			"content_type": "image/jpeg",
			"b64":          "aGVsbG8=",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, ok := out["content"].([]apitypes.JSONObject)
	if !ok || len(content) != 2 {
		t.Fatalf("content len = %d, want 2 (image_url + text)", len(content))
	}
	// Image reference must come first.
	if content[0]["type"] != "image_url" {
		t.Errorf("content[0].type = %v, want image_url", content[0]["type"])
	}
	if content[0]["role"] != "first_frame" {
		t.Errorf("content[0].role = %v, want first_frame", content[0]["role"])
	}
	if content[1]["type"] != "text" {
		t.Errorf("content[1].type = %v, want text", content[1]["type"])
	}
}

func TestMapOpenAIVideosToMinimaxVideoRequest_PreservesModel(t *testing.T) {
	out, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{
		"prompt": "a cat jumping",
		"model":  "MiniMax-H3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["model"] != "MiniMax-H3" {
		t.Errorf("model = %v, want MiniMax-H3", out["model"])
	}
}

func TestMapOpenAIVideosToMinimaxVideoRequest_MissingPromptReturnsError(t *testing.T) {
	_, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{})
	if err == nil {
		t.Fatal("expected error for missing prompt, got nil")
	}
	var mapErr *RequestMappingError
	if !errors.As(err, &mapErr) {
		t.Fatalf("error type = %T, want *RequestMappingError", err)
	}
	if mapErr.Code != CodeRequestPromptMissing {
		t.Fatalf("code = %q, want %q", mapErr.Code, CodeRequestPromptMissing)
	}
}

func TestMapOpenAIVideosToMinimaxVideoRequest_UnsupportedSizeReturnsError(t *testing.T) {
	_, err := MapOpenAIVideosToMinimaxVideoRequest(apitypes.JSONObject{
		"prompt": "a cat jumping",
		"size":   "9999x9999",
	})
	if err == nil {
		t.Fatal("expected error for unsupported size, got nil")
	}
	var mapErr *RequestMappingError
	if !errors.As(err, &mapErr) {
		t.Fatalf("error type = %T, want *RequestMappingError", err)
	}
	if mapErr.Code != CodeRequestSizeNotSupported {
		t.Fatalf("code = %q, want %q", mapErr.Code, CodeRequestSizeNotSupported)
	}
}

// --- openAISizeToMinimaxResolution ---

func TestOpenAISizeToMinimaxResolution(t *testing.T) {
	for size, want := range map[string]string{
		// WxH landscape
		"854x480":   "480P",
		"1280x720":  "480P",
		"1920x1080": "768P",
		"2560x1440": "2K",
		"3840x2160": "2K",
		// WxH portrait
		"480x854":   "480P",
		"720x1280":  "480P",
		"1080x1920": "768P",
		"2160x3840": "2K",
		// Pass-through labels (case-insensitive)
		"480p": "480P",
		"768p": "768P",
		"2k":   "2K",
		// Unrecognised → empty
		"1024x768": "",
		"":         "",
	} {
		got := openAISizeToMinimaxResolution(size)
		if got != want {
			t.Errorf("openAISizeToMinimaxResolution(%q) = %q, want %q", size, got, want)
		}
	}
}
