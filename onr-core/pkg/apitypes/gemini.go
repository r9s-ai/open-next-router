package apitypes

// GeminiGenerateContentRequest is a minimal Gemini request DTO placeholder.
type GeminiGenerateContentRequest struct {
	Contents          []JSONObject `json:"contents,omitempty"`
	SystemInstruction JSONObject   `json:"system_instruction,omitempty"`
	GenerationConfig  JSONObject   `json:"generation_config,omitempty"`
	ToolConfig        JSONObject   `json:"tool_config,omitempty"`
	SafetySettings    []JSONObject `json:"safety_settings,omitempty"`
	CachedContent     string       `json:"cachedContent,omitempty"`
}

// GeminiGenerateContentResponse is a minimal Gemini response DTO placeholder.
type GeminiGenerateContentResponse struct {
	Candidates    []JSONObject `json:"candidates,omitempty"`
	UsageMetadata JSONObject   `json:"usageMetadata,omitempty"`
}
