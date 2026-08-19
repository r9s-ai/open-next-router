package apitransform

import (
	"errors"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

func ppioVideoRoot(extra map[string]any) apitypes.JSONObject {
	root := apitypes.JSONObject{
		"model":   "sora-2",
		"prompt":  "a fox running through snow",
		"seconds": "8",
		"size":    "1280x720",
	}
	for k, v := range extra {
		root[k] = v
	}
	return root
}

// Text-to-video carries an exact size in PPIO's star form; there is no image.
func TestMapOpenAIVideosToPPIO_TextToVideo(t *testing.T) {
	out, err := MapOpenAIVideosToPPIORequest(ppioVideoRoot(nil))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if out["prompt"] != "a fox running through snow" {
		t.Fatalf("prompt=%v", out["prompt"])
	}
	if out["duration"] != 8 {
		t.Fatalf("duration=%v want 8", out["duration"])
	}
	if out["size"] != "1280*720" {
		t.Fatalf("size=%v want the star form", out["size"])
	}
	for _, k := range []string{"image", "resolution"} {
		if _, exists := out[k]; exists {
			t.Fatalf("text-to-video must not carry %q", k)
		}
	}
}

// Image-to-video swaps the exact size for a coarse resolution and sends the
// reference image as a data URL, which is what the Go adaptor produces.
func TestMapOpenAIVideosToPPIO_ImageToVideo(t *testing.T) {
	out, err := MapOpenAIVideosToPPIORequest(ppioVideoRoot(map[string]any{
		"input_reference": []any{map[string]any{
			"filename": "ref.png", "content_type": "image/png", "b64": "UE5H",
		}},
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if out["image"] != "data:image/png;base64,UE5H" {
		t.Fatalf("image=%v want a data URL", out["image"])
	}
	if out["resolution"] != "720p" {
		t.Fatalf("resolution=%v want 720p", out["resolution"])
	}
	if _, exists := out["size"]; exists {
		t.Fatal("image-to-video takes resolution, not size")
	}
}

func TestMapOpenAIVideosToPPIO_ResolutionFromSize(t *testing.T) {
	for size, want := range map[string]string{
		"1920x1080": "1080p",
		"1280x720":  "720p",
		"720x1280":  "720p",
	} {
		out, err := MapOpenAIVideosToPPIORequest(ppioVideoRoot(map[string]any{
			"size": size,
			"input_reference": []any{map[string]any{
				"content_type": "image/png", "b64": "UE5H",
			}},
		}))
		if err != nil {
			t.Fatalf("map %s: %v", size, err)
		}
		if out["resolution"] != want {
			t.Fatalf("size=%s resolution=%v want %v", size, out["resolution"], want)
		}
	}
}

// The Go adaptor rejects a missing seconds or size as one opaque
// "ppio_video_field_missing"; naming the field is a deliberate improvement.
func TestMapOpenAIVideosToPPIO_RejectsMissingFields(t *testing.T) {
	cases := map[string]struct {
		root  apitypes.JSONObject
		param string
	}{
		"no prompt":  {ppioVideoRoot(map[string]any{"prompt": "  "}), "prompt"},
		"no seconds": {ppioVideoRoot(map[string]any{"seconds": ""}), "seconds"},
		"no size":    {ppioVideoRoot(map[string]any{"size": ""}), "size"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := MapOpenAIVideosToPPIORequest(tc.root)
			var mapErr *RequestMappingError
			if !errors.As(err, &mapErr) {
				t.Fatalf("err=%v want *RequestMappingError", err)
			}
			if mapErr.Param != tc.param {
				t.Fatalf("param=%q want %q", mapErr.Param, tc.param)
			}
		})
	}
}

func TestMapPPIOVideoCreate_ToVideoObject(t *testing.T) {
	out, err := MapPPIOVideoCreateToOpenAIVideoObject(apitypes.JSONObject{"task_id": "tsk_123"})
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if out["id"] != "tsk_123" || out["object"] != "video" || out["status"] != "queued" {
		t.Fatalf("out=%v", out)
	}
	if out["progress"] != 0 {
		t.Fatalf("progress=%v want 0", out["progress"])
	}
}

// A create response with no task id leaves nothing to poll, so it must fail
// rather than produce a task handle that can never resolve.
func TestMapPPIOVideoCreate_RejectsMissingTaskID(t *testing.T) {
	_, err := MapPPIOVideoCreateToOpenAIVideoObject(apitypes.JSONObject{})
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("err=%v want *UpstreamResponseError", err)
	}
}

func ppioResult(status string, extra map[string]any) apitypes.JSONObject {
	task := map[string]any{"task_id": "tsk_123", "status": status, "progress_percent": float64(42)}
	root := apitypes.JSONObject{"task": task}
	for k, v := range extra {
		if k == "progress_percent" || k == "reason" {
			task[k] = v
			continue
		}
		root[k] = v
	}
	return root
}

func TestMapPPIOVideoResult_StatusMapping(t *testing.T) {
	for upstream, want := range map[string]string{
		"TASK_STATUS_QUEUED":     "queued",
		"TASK_STATUS_PROCESSING": "in_progress",
		"TASK_STATUS_FAILED":     "failed",
	} {
		out, err := MapPPIOVideoResultToOpenAIVideoObject(ppioResult(upstream, nil))
		if err != nil {
			t.Fatalf("%s: %v", upstream, err)
		}
		if out["status"] != want {
			t.Fatalf("%s -> %v want %v", upstream, out["status"], want)
		}
	}
}

// An unrecognized status must fail loudly. Defaulting it to queued would leave
// the caller polling a task that will never advance.
func TestMapPPIOVideoResult_UnknownStatusIsAnError(t *testing.T) {
	_, err := MapPPIOVideoResultToOpenAIVideoObject(ppioResult("TASK_STATUS_WAT", nil))
	var upErr *UpstreamResponseError
	if !errors.As(err, &upErr) {
		t.Fatalf("err=%v want *UpstreamResponseError", err)
	}
}

func TestMapPPIOVideoResult_CompletedCarriesVideo(t *testing.T) {
	out, err := MapPPIOVideoResultToOpenAIVideoObject(ppioResult("TASK_STATUS_SUCCEED", map[string]any{
		"videos": []any{map[string]any{
			"video_url": "https://example.invalid/v.mp4", "video_url_ttl": "3600",
		}},
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if out["video_url"] != "https://example.invalid/v.mp4" {
		t.Fatalf("video_url=%v", out["video_url"])
	}
	// A completed task is 100% regardless of what the upstream last reported.
	if out["progress"] != 100 {
		t.Fatalf("progress=%v want 100", out["progress"])
	}
	completed, _ := out["completed_at"].(int64)
	expires, _ := out["expires_at"].(int64)
	if expires != completed+3600 {
		t.Fatalf("expires_at=%v completed_at=%v want +3600", expires, completed)
	}
}

// completed with no video is a contract violation: the caller would be told to
// download something that does not exist.
func TestMapPPIOVideoResult_CompletedWithoutVideoIsAnError(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"no videos":  {},
		"empty list": {"videos": []any{}},
		"blank url":  {"videos": []any{map[string]any{"video_url": "  "}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MapPPIOVideoResultToOpenAIVideoObject(ppioResult("TASK_STATUS_SUCCEED", extra))
			var upErr *UpstreamResponseError
			if !errors.As(err, &upErr) {
				t.Fatalf("err=%v want *UpstreamResponseError", err)
			}
		})
	}
}

// A failed task surfaces the upstream reason so the caller learns why, instead
// of a bare status with no explanation.
func TestMapPPIOVideoResult_FailedCarriesReason(t *testing.T) {
	out, err := MapPPIOVideoResultToOpenAIVideoObject(ppioResult("TASK_STATUS_FAILED", map[string]any{
		"reason": "content policy",
	}))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	errObj, _ := out["error"].(apitypes.JSONObject)
	if errObj == nil || errObj["message"] != "content policy" {
		t.Fatalf("error=%v want the upstream reason", out["error"])
	}
}

func TestMapPPIOVideoResult_ProgressClamped(t *testing.T) {
	for raw, want := range map[float64]int{-5: 0, 42: 42, 250: 100} {
		out, err := MapPPIOVideoResultToOpenAIVideoObject(ppioResult("TASK_STATUS_PROCESSING",
			map[string]any{"progress_percent": raw}))
		if err != nil {
			t.Fatalf("progress %v: %v", raw, err)
		}
		if out["progress"] != want {
			t.Fatalf("progress %v -> %v want %v", raw, out["progress"], want)
		}
	}
}
