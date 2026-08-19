package apitransform

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// PPIO (Novita) video generation mapping. These builtins replicate the relay Go
// adaptor (internal/channel/adaptor/ppio) so OpenAI-compatible video requests
// can be routed to PPIO's async task API through the DSL config-file pipeline.
//
// Video generation is asynchronous: creating a task returns a handle, the
// caller polls, and the content arrives at the end. Only the parts an upstream
// payload can determine live here. Everything that needs stored task state —
// the router-local task id, the prompt/size echoed back from the create call,
// progress monotonicity across polls — stays with the host, which is the only
// side that has the task row.

const (
	// ppioObjectVideo matches model.TaskTypeVideo on the relay side, which is
	// what the OpenAI video object carries in its "object" field.
	ppioObjectVideo = "video"

	ppioStatusQueued     = "queued"
	ppioStatusInProgress = "in_progress"
	ppioStatusCompleted  = "completed"
	ppioStatusFailed     = "failed"
)

// ppioDefaultResolution is what the Go adaptor falls back to when the caller
// sent no size at all.
const ppioDefaultResolution = "720p"

// ppioNormalizeSize converts an OpenAI size ("1280x720") to PPIO's star form
// ("1280*720"), matching the Go adaptor's normalizeSize.
func ppioNormalizeSize(size string) string {
	return strings.ReplaceAll(strings.TrimSpace(size), "x", "*")
}

// ppioResolutionFromSize collapses a size to PPIO's coarse resolution field.
// Image-to-video takes a resolution rather than exact dimensions.
func ppioResolutionFromSize(size string) string {
	trimmed := strings.TrimSpace(size)
	if trimmed == "" {
		return ppioDefaultResolution
	}
	if strings.Contains(trimmed, "1080") {
		return "1080p"
	}
	return ppioDefaultResolution
}

// ppioDurationSeconds parses OpenAI's string "seconds" into PPIO's integer
// duration. A non-numeric value becomes 0, matching the Go adaptor, which lets
// the upstream apply its own default rather than guessing one here.
func ppioDurationSeconds(raw string) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	if v, err := strconv.Atoi(trimmed); err == nil {
		return v
	}
	return 0
}

// MapOpenAIVideosToPPIORequest builds a PPIO async video request from an OpenAI
// videos.generations request root.
//
// PPIO splits the operation across two endpoints and two payload shapes: text
// to video, and image to video when a reference image was uploaded. The path is
// selected in the config with if_present on the same field this reads, so the
// two decisions stay in sync.
//
// The reference image must already have been inlined by req_inline_file — this
// reads the {filename, content_type, b64} shape that directive produces — and
// is passed as a data URL, which is how the Go adaptor encodes it.
func MapOpenAIVideosToPPIORequest(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	prompt := jsonutil.CoerceString(root["prompt"])
	seconds := strings.TrimSpace(jsonutil.CoerceString(root["seconds"]))
	size := strings.TrimSpace(jsonutil.CoerceString(root["size"]))

	if strings.TrimSpace(prompt) == "" {
		return nil, newRequestMappingError(CodeRequestPromptMissing, "prompt", "prompt is required")
	}
	// The Go adaptor rejects both as "ppio_video_field_missing" without saying
	// which one; naming the offending field costs nothing and saves a round trip.
	if seconds == "" {
		return nil, newRequestMappingError(CodeRequestMissingRequiredField, "seconds", "seconds is required")
	}
	if size == "" {
		return nil, newRequestMappingError(CodeRequestMissingRequiredField, "size", "size is required")
	}

	duration := ppioDurationSeconds(seconds)
	out := apitypes.JSONObject{"prompt": prompt}
	if duration > 0 {
		out["duration"] = duration
	}

	reference := inlineFilesAt(root, "input_reference")
	if len(reference) == 0 {
		out["size"] = ppioNormalizeSize(size)
		return out, nil
	}
	out["image"] = "data:" + reference[0].ContentType + ";base64," + reference[0].Data
	out["resolution"] = ppioResolutionFromSize(ppioNormalizeSize(size))
	return out, nil
}

// MapPPIOVideoCreateToOpenAIVideoObject converts PPIO's task-creation response
// into the OpenAI video object shape.
//
// Only the fields the upstream determined are set. The host replaces id with a
// router-local task id and fills prompt/seconds/size from the client's request,
// because a poll later has to answer with those and PPIO never echoes them back.
func MapPPIOVideoCreateToOpenAIVideoObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	taskID := strings.TrimSpace(jsonutil.CoerceString(root["task_id"]))
	if taskID == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task_id",
			Message:    "PPIO create response has no task_id",
		}
	}
	return apitypes.JSONObject{
		"object":     ppioObjectVideo,
		"id":         taskID,
		"status":     ppioStatusQueued,
		"progress":   0,
		"created_at": time.Now().Unix(),
	}, nil
}

// ppioTaskStatus maps PPIO's task status to the OpenAI video status vocabulary.
// An unrecognized status is an error rather than a guess: reporting an unknown
// state as queued would keep a caller polling a task that will never advance.
func ppioTaskStatus(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "TASK_STATUS_SUCCEED":
		return ppioStatusCompleted, nil
	case "TASK_STATUS_FAILED":
		return ppioStatusFailed, nil
	case "TASK_STATUS_PROCESSING":
		return ppioStatusInProgress, nil
	case "TASK_STATUS_QUEUED":
		return ppioStatusQueued, nil
	default:
		return "", &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_unknown_task_status",
			Message:    "PPIO returned an unknown task status: " + strings.TrimSpace(raw),
		}
	}
}

// MapPPIOVideoResultToOpenAIVideoObject converts PPIO's task-result response
// into the OpenAI video object shape.
//
// Progress is clamped to [0, 100] but not made monotonic here: only the host
// knows what the previous poll reported, and a value that goes backwards
// between polls is corrected there.
func MapPPIOVideoResultToOpenAIVideoObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	task, _ := root["task"].(map[string]any)
	if task == nil {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task",
			Message:    "PPIO task-result response has no task",
		}
	}
	if strings.TrimSpace(jsonutil.CoerceString(task["task_id"])) == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task_id",
			Message:    "PPIO task-result response has no task_id",
		}
	}

	status, err := ppioTaskStatus(jsonutil.CoerceString(task["status"]))
	if err != nil {
		return nil, err
	}

	progress := jsonutil.CoerceInt(task["progress_percent"])
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	out := apitypes.JSONObject{
		"object": ppioObjectVideo,
		"status": status,
	}
	terminal := status == ppioStatusCompleted || status == ppioStatusFailed
	completedAt := time.Now().Unix()
	if terminal {
		out["completed_at"] = completedAt
	}

	if status == ppioStatusFailed {
		out["progress"] = progress
		if reason := strings.TrimSpace(jsonutil.CoerceString(task["reason"])); reason != "" {
			out["error"] = apitypes.JSONObject{"code": "upstream_task_failed", "message": reason}
		}
		return out, nil
	}

	if status != ppioStatusCompleted {
		out["progress"] = progress
		return out, nil
	}

	// A completed task without a video is a contract violation: the caller would
	// see status=completed and then find nothing to download.
	videos, _ := root["videos"].([]any)
	if len(videos) == 0 {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_video",
			Message:    "PPIO reported a completed task with no video",
		}
	}
	first, _ := videos[0].(map[string]any)
	videoURL := strings.TrimSpace(jsonutil.CoerceString(first["video_url"]))
	if videoURL == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_video",
			Message:    "PPIO reported a completed task with an empty video url",
		}
	}

	out["progress"] = 100
	out["video_url"] = videoURL
	// The URL is time limited. Surfacing the absolute expiry lets a caller tell
	// "not fetched yet" from "too late to fetch"; an unparsable TTL is treated
	// as already expired rather than dropped, which is the safer direction.
	if ttl, convErr := strconv.ParseInt(strings.TrimSpace(jsonutil.CoerceString(first["video_url_ttl"])), 10, 64); convErr == nil {
		out["expires_at"] = completedAt + ttl
	} else {
		out["expires_at"] = completedAt
	}
	return out, nil
}
