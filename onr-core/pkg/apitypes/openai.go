package apitypes

// OpenAIChatCompletionsRequest is a minimal OpenAI chat.completions request DTO.
type OpenAIChatCompletionsRequest struct {
	Model               string          `json:"model"`
	Messages            []OpenAIMessage `json:"messages,omitempty"`
	N                   int             `json:"n,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	User                string          `json:"user,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Tools               []JSONObject    `json:"tools,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	ParallelToolCalls   any             `json:"parallel_tool_calls,omitempty"`
	ResponseFormat      any             `json:"response_format,omitempty"`
	Store               *bool           `json:"store,omitempty"`
	Metadata            map[string]any  `json:"metadata,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
}

// OpenAIMessage is a minimal chat message DTO used across compatibility mapping.
type OpenAIMessage struct {
	Role       string       `json:"role"`
	Content    any          `json:"content,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	ToolCalls  []JSONObject `json:"tool_calls,omitempty"`
}

// OpenAIResponsesRequest is a minimal OpenAI responses request DTO.
type OpenAIResponsesRequest struct {
	Model             string       `json:"model"`
	Input             any          `json:"input,omitempty"`
	Instructions      string       `json:"instructions,omitempty"`
	Stream            bool         `json:"stream,omitempty"`
	MaxOutputTokens   int          `json:"max_output_tokens,omitempty"`
	Temperature       *float64     `json:"temperature,omitempty"`
	TopP              *float64     `json:"top_p,omitempty"`
	User              string       `json:"user,omitempty"`
	Tools             []JSONObject `json:"tools,omitempty"`
	ToolChoice        any          `json:"tool_choice,omitempty"`
	ParallelToolCalls any          `json:"parallel_tool_calls,omitempty"`
	Text              JSONObject   `json:"text,omitempty"`
	Store             *bool        `json:"store,omitempty"`
	Metadata          JSONObject   `json:"metadata,omitempty"`
	Reasoning         JSONObject   `json:"reasoning,omitempty"`
}

// OpenAIResponsesResponse is a minimal OpenAI responses response DTO.
type OpenAIResponsesResponse struct {
	ID         string       `json:"id,omitempty"`
	CreatedAt  int64        `json:"created_at,omitempty"`
	Model      string       `json:"model,omitempty"`
	Status     string       `json:"status,omitempty"`
	OutputText string       `json:"output_text,omitempty"`
	Output     []JSONObject `json:"output,omitempty"`
	Usage      JSONObject   `json:"usage,omitempty"`
}

// OpenAIChatCompletionsResponse is a minimal OpenAI chat.completions response DTO.
type OpenAIChatCompletionsResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model,omitempty"`
	Choices []JSONObject `json:"choices"`
	Usage   JSONObject   `json:"usage,omitempty"`
}
