package apitypes

// ClaudeMessageRequest is a minimal Claude messages request DTO placeholder.
type ClaudeMessageRequest struct {
	Model    string       `json:"model"`
	Messages []JSONObject `json:"messages,omitempty"`
	System   any          `json:"system,omitempty"`
	Stream   bool         `json:"stream,omitempty"`
}

// ClaudeMessageResponse is a minimal Claude messages response DTO placeholder.
type ClaudeMessageResponse struct {
	ID         string       `json:"id,omitempty"`
	Type       string       `json:"type,omitempty"`
	Role       string       `json:"role,omitempty"`
	Content    []JSONObject `json:"content,omitempty"`
	StopReason string       `json:"stop_reason,omitempty"`
}
