package apitransform

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Minimax H3 video generation response mapping. Minimax exposes an async
// task API with creation at /v2/video_generation, polling at
// /v2/query/video_generation/{task_id}, and H3 Context IR (draft enriched
// prompt) at /v2/h3_context_ir. Regeneration at /v2/video_regeneration works
// like Sora remix: it upscales or re-renders an existing task's video.
//
// Status vocabulary differs from OpenAI: "running" maps to "in_progress" and
// "succeeded" maps to "completed". The remaining status values ("queued",
// "failed", "cancelled") are identical to OpenAI's.

const (
	minimaxObjectVideo = "video"

	minimaxStatusQueued     = "queued"
	minimaxStatusInProgress = "in_progress"
	minimaxStatusCompleted  = "completed"
	minimaxStatusFailed     = "failed"
	minimaxStatusCancelled  = "cancelled"
)

var minimaxNativeToOpenAIStatus = map[string]string{
	"queued":    minimaxStatusQueued,
	"running":   minimaxStatusInProgress,
	"succeeded": minimaxStatusCompleted,
	"failed":    minimaxStatusFailed,
	"cancelled": minimaxStatusCancelled,
}

// minimaxTaskStatus maps Minimax's status strings to the OpenAI video status
// vocabulary. An unrecognized status is an error: guessing "queued" would keep
// a caller polling a task that will never advance.
func minimaxTaskStatus(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if s, ok := minimaxNativeToOpenAIStatus[trimmed]; ok {
		return s, nil
	}
	return "", &UpstreamResponseError{
		StatusCode: http.StatusBadGateway,
		Type:       "server_error",
		Code:       "upstream_unknown_task_status",
		Message:    "Minimax returned an unknown task status: " + trimmed,
	}
}

// MapMinimaxVideoCreateToOpenAIVideoObject converts Minimax's task-creation
// response into the OpenAI video object shape. Minimax create returns
// {"task_id": "..."}, which we normalise to a queued video object.
func MapMinimaxVideoCreateToOpenAIVideoObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	taskID := strings.TrimSpace(jsonutil.CoerceString(root["task_id"]))
	if taskID == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task_id",
			Message:    "Minimax create response has no task_id",
		}
	}
	return apitypes.JSONObject{
		"object":     minimaxObjectVideo,
		"id":         taskID,
		"status":     minimaxStatusQueued,
		"progress":   0,
		"created_at": time.Now().Unix(),
	}, nil
}

// MapMinimaxVideoResultToOpenAIVideoObject converts Minimax's GET task
// response into the OpenAI video object shape.
//
// The query response wraps the task inside a "task" object:
//
//	{"task": {"id": "...", "status": "...", "content": {"url": "..."}, "usage": {...}, ...}}
//
// For video tasks (modality="video"), content.url becomes video_url.
// For H3 Context IR tasks (modality="text"), content.prompt is returned as
// result_prompt.  Both paths carry usage for downstream billing extraction.
func MapMinimaxVideoResultToOpenAIVideoObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	task, _ := root["task"].(map[string]any)
	if task == nil {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task",
			Message:    "Minimax task-result response has no task",
		}
	}

	taskID := strings.TrimSpace(jsonutil.CoerceString(task["id"]))
	if taskID == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task_id",
			Message:    "Minimax task-result response has no task.id",
		}
	}

	rawStatus := jsonutil.CoerceString(task["status"])
	status, err := minimaxTaskStatus(rawStatus)
	if err != nil {
		return nil, err
	}

	taskType := strings.TrimSpace(jsonutil.CoerceString(task["task_type"]))
	modality := strings.TrimSpace(jsonutil.CoerceString(task["modality"]))

	out := apitypes.JSONObject{
		"object":    minimaxObjectVideo,
		"id":        taskID,
		"status":    status,
		"task_type": taskType,
	}

	terminal := status == minimaxStatusCompleted || status == minimaxStatusFailed || status == minimaxStatusCancelled
	if terminal {
		out["completed_at"] = time.Now().Unix()
	}

	if resolution := strings.TrimSpace(jsonutil.CoerceString(task["resolution"])); resolution != "" {
		out["resolution"] = resolution
	}
	if duration := jsonutil.CoerceInt(task["duration"]); duration > 0 {
		out["duration"] = duration
	}

	if status == minimaxStatusFailed {
		if errObj, _ := task["error"].(map[string]any); errObj != nil {
			if msg := strings.TrimSpace(jsonutil.CoerceString(errObj["message"])); msg != "" {
				out["error"] = apitypes.JSONObject{"code": "upstream_task_failed", "message": msg}
			}
		}
		return out, nil
	}

	if status == minimaxStatusCancelled {
		return out, nil
	}

	if status != minimaxStatusCompleted {
		return out, nil
	}

	out["progress"] = 100

	// Forward usage so the relay billing extractor can read output_seconds or
	// completion_tokens at the root of the stored upstream payload.
	if usage, _ := task["usage"].(map[string]any); usage != nil {
		out["usage"] = usage
	}

	content, _ := task["content"].(map[string]any)
	isTextResult := modality == "text" || taskType == "h3_context_ir"
	if isTextResult {
		// H3 Context IR produces an enriched text prompt, not a video URL.
		if content != nil {
			if prompt := strings.TrimSpace(jsonutil.CoerceString(content["prompt"])); prompt != "" {
				out["result_prompt"] = prompt
			}
		}
		return out, nil
	}

	// Video and regeneration tasks produce a downloadable URL.
	var videoURL string
	if content != nil {
		videoURL = strings.TrimSpace(jsonutil.CoerceString(content["url"]))
	}
	if videoURL == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_video",
			Message:    "Minimax reported a completed task with no content.url",
		}
	}
	out["video_url"] = videoURL
	return out, nil
}

// MapOpenAIVideosToMinimaxVideoRequest converts an OpenAI videos.generations
// request into Minimax H3 video generation format.
//
// The OpenAI "size" ("WxH") is mapped to Minimax resolution ("480P"/"768P"/"2K")
// and "seconds" (float) is converted to Minimax duration (integer). The prompt
// is wrapped in the content array Minimax requires.
func MapOpenAIVideosToMinimaxVideoRequest(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	prompt := strings.TrimSpace(jsonutil.CoerceString(root["prompt"]))
	if prompt == "" {
		return nil, newRequestMappingError(CodeRequestPromptMissing, "prompt", "prompt is required")
	}

	// Reference images (first_frame etc.) precede the text prompt in the content
	// array so Minimax reads the visual context before the creative instruction.
	content := make([]apitypes.JSONObject, 0, 2)
	for _, entry := range inlineFilesAt(root, "input_reference") {
		content = append(content, apitypes.JSONObject{
			"type":      "image_url",
			"image_url": apitypes.JSONObject{"url": "data:" + entry.ContentType + ";base64," + entry.Data},
			"role":      "first_frame",
		})
	}
	content = append(content, apitypes.JSONObject{"type": "text", "text": prompt})

	out := apitypes.JSONObject{"content": content}

	// seconds is a float in OpenAI; Minimax accepts an integer duration.
	if rawSeconds := strings.TrimSpace(jsonutil.CoerceString(root["seconds"])); rawSeconds != "" {
		if v, err := strconv.ParseFloat(rawSeconds, 64); err == nil && v > 0 {
			out["duration"] = int(v)
		}
	}

	if rawSize := strings.TrimSpace(jsonutil.CoerceString(root["size"])); rawSize != "" {
		res := openAISizeToMinimaxResolution(rawSize)
		if res == "" {
			return nil, newRequestMappingError(CodeRequestSizeNotSupported, "size",
				"unsupported resolution %q; accepted: 480P, 768P, 2K", rawSize)
		}
		out["resolution"] = res
	}

	if model := strings.TrimSpace(jsonutil.CoerceString(root["model"])); model != "" {
		out["model"] = model
	}
	return out, nil
}

// minimaxSizeToResolution maps known size strings (lowercase) to Minimax
// resolution labels. Covers normalised labels and common OpenAI WxH values;
// openAISizeToMinimaxResolution falls back to short-edge math for anything
// not in this table.
var minimaxSizeToResolution = map[string]string{
	// Minimax resolution labels (pass-through normalisation)
	"480p": "480P",
	"768p": "768P",
	"2k":   "2K",
	// Common OpenAI landscape WxH → Minimax tier
	"854x480":   "480P",
	"1280x720":  "480P",
	"1360x768":  "768P",
	"1920x1080": "768P",
	"2560x1440": "2K",
	"3840x2160": "2K",
	// Common OpenAI portrait WxH → Minimax tier
	"480x854":   "480P",
	"720x1280":  "480P",
	"768x1360":  "768P",
	"1080x1920": "768P",
	"1440x2560": "2K",
	"2160x3840": "2K",
}

// openAISizeToMinimaxResolution converts an OpenAI size value ("WxH") to the
// Minimax resolution label. Already-normalised Minimax labels are passed through.
// Returns "" for unrecognised values so the caller can reject them explicitly.
func openAISizeToMinimaxResolution(size string) string {
	return minimaxSizeToResolution[strings.ToLower(strings.TrimSpace(size))]
}
