package apitransform

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Volcengine Seedance (Ark) video response mapping. Seedance exposes an async
// task API at /api/v3/contents/generations/tasks with a nested content.video_url
// in the terminal-state response.
//
// Status vocabulary differs from OpenAI: "running" maps to "in_progress" and
// "succeeded" maps to "completed". The remaining status values ("queued",
// "failed", "cancelled") are identical to OpenAI's.

const (
	seedanceObjectVideo = "video"

	seedanceStatusQueued     = "queued"
	seedanceStatusInProgress = "in_progress"
	seedanceStatusCompleted  = "completed"
	seedanceStatusFailed     = "failed"
	seedanceStatusCancelled  = "cancelled"
)

// seedanceTaskStatus maps Seedance's status strings to the OpenAI video status
// vocabulary. An unrecognized status is an error: guessing "queued" would keep
// a caller polling a task that will never advance.
func seedanceTaskStatus(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "queued":
		return seedanceStatusQueued, nil
	case "running":
		return seedanceStatusInProgress, nil
	case "succeeded":
		return seedanceStatusCompleted, nil
	case "failed":
		return seedanceStatusFailed, nil
	case "cancelled":
		return seedanceStatusCancelled, nil
	default:
		return "", &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_unknown_task_status",
			Message:    "Seedance returned an unknown task status: " + strings.TrimSpace(raw),
		}
	}
}

// MapOpenAIVideosToSeedanceRequest builds a Seedance async video request from an
// OpenAI videos.generations request root.
//
// Seedance wraps the prompt inside a content array rather than accepting it as
// a flat field, and uses the same "WxH" size format as OpenAI but calls it
// resolution. seconds is converted to an integer duration the same way PPIO
// does it.
func MapOpenAIVideosToSeedanceRequest(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	prompt := strings.TrimSpace(jsonutil.CoerceString(root["prompt"]))
	if prompt == "" {
		return nil, newRequestMappingError(CodeRequestPromptMissing, "prompt", "prompt is required")
	}
	out := apitypes.JSONObject{
		"content": []apitypes.JSONObject{{"type": "text", "text": prompt}},
	}
	if seconds := strings.TrimSpace(jsonutil.CoerceString(root["seconds"])); seconds != "" {
		if v, err := strconv.Atoi(seconds); err == nil && v > 0 {
			out["duration"] = v
		}
	}
	if size := strings.TrimSpace(jsonutil.CoerceString(root["size"])); size != "" {
		out["resolution"] = size
	}
	return out, nil
}

// MapSeedanceVideoCreateToOpenAIVideoObject converts Seedance's task-creation
// response into the OpenAI video object shape.
//
// Only the task ID is available at creation time; the host fills prompt/seconds/
// size from the client's original request for later poll responses.
func MapSeedanceVideoCreateToOpenAIVideoObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	taskID := strings.TrimSpace(jsonutil.CoerceString(root["id"]))
	if taskID == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task_id",
			Message:    "Seedance create response has no id",
		}
	}
	return apitypes.JSONObject{
		"object":     seedanceObjectVideo,
		"id":         taskID,
		"status":     seedanceStatusQueued,
		"progress":   0,
		"created_at": time.Now().Unix(),
	}, nil
}

// MapSeedanceVideoResultToOpenAIVideoObject converts Seedance's GET task
// response into the OpenAI video object shape.
//
// The video URL lives at content.video_url in the upstream response. A
// completed task without a video_url is a contract violation.
func MapSeedanceVideoResultToOpenAIVideoObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	taskID := strings.TrimSpace(jsonutil.CoerceString(root["id"]))
	if taskID == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_task_id",
			Message:    "Seedance task-result response has no id",
		}
	}

	rawStatus := jsonutil.CoerceString(root["status"])
	status, err := seedanceTaskStatus(rawStatus)
	if err != nil {
		return nil, err
	}

	out := apitypes.JSONObject{
		"object": seedanceObjectVideo,
		"status": status,
	}

	terminal := status == seedanceStatusCompleted || status == seedanceStatusFailed || status == seedanceStatusCancelled
	completedAt := time.Now().Unix()
	if terminal {
		out["completed_at"] = completedAt
	}

	if status == seedanceStatusFailed {
		if errMsg, _ := root["error"].(map[string]any); errMsg != nil {
			if msg := strings.TrimSpace(jsonutil.CoerceString(errMsg["message"])); msg != "" {
				out["error"] = apitypes.JSONObject{"code": "upstream_task_failed", "message": msg}
			}
		}
		return out, nil
	}

	if status == seedanceStatusCancelled {
		return out, nil
	}

	if status != seedanceStatusCompleted {
		return out, nil
	}

	content, _ := root["content"].(map[string]any)
	var videoURL string
	if content != nil {
		videoURL = strings.TrimSpace(jsonutil.CoerceString(content["video_url"]))
	}
	if videoURL == "" {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusBadGateway,
			Type:       "server_error",
			Code:       "upstream_missing_video",
			Message:    "Seedance reported a completed task with no video_url",
		}
	}

	out["progress"] = 100
	out["video_url"] = videoURL
	return out, nil
}
