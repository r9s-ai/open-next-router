package apitypes

import (
	"strings"
)

type OpenAIChatCompletionsRequest struct {
	Model                string                        `json:"model"`
	Messages             []OpenAIChatMessage           `json:"messages,omitempty"`
	Audio                *OpenAIChatAudioParam         `json:"audio,omitempty"`
	FrequencyPenalty     *float64                      `json:"frequency_penalty,omitempty"`
	FunctionCall         *OpenAIChatFunctionCallOption `json:"function_call,omitempty"`
	Functions            []OpenAIFunctionDefinition    `json:"functions,omitempty"`
	LogitBias            map[string]float64            `json:"logit_bias,omitempty"`
	Logprobs             *bool                         `json:"logprobs,omitempty"`
	MaxCompletionTokens  int                           `json:"max_completion_tokens,omitempty"`
	MaxTokens            int                           `json:"max_tokens,omitempty"`
	Metadata             map[string]string             `json:"metadata,omitempty"`
	Modalities           []string                      `json:"modalities,omitempty"`
	N                    int                           `json:"n,omitempty"`
	ParallelToolCalls    *bool                         `json:"parallel_tool_calls,omitempty"`
	Prediction           *OpenAIChatPrediction         `json:"prediction,omitempty"`
	PresencePenalty      *float64                      `json:"presence_penalty,omitempty"`
	PromptCacheKey       string                        `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string                        `json:"prompt_cache_retention,omitempty"`
	ReasoningEffort      string                        `json:"reasoning_effort,omitempty"`
	ResponseFormat       *OpenAIChatResponseFormat     `json:"response_format,omitempty"`
	SafetyIdentifier     string                        `json:"safety_identifier,omitempty"`
	Seed                 *int                          `json:"seed,omitempty"`
	ServiceTier          string                        `json:"service_tier,omitempty"`
	Stop                 *OpenAIChatStop               `json:"stop,omitempty"`
	Store                *bool                         `json:"store,omitempty"`
	Stream               bool                          `json:"stream,omitempty"`
	StreamOptions        *OpenAIChatStreamOptions      `json:"stream_options,omitempty"`
	Temperature          *float64                      `json:"temperature,omitempty"`
	ToolChoice           *OpenAIChatToolChoice         `json:"tool_choice,omitempty"`
	Tools                []OpenAIChatTool              `json:"tools,omitempty"`
	TopLogprobs          int                           `json:"top_logprobs,omitempty"`
	TopP                 *float64                      `json:"top_p,omitempty"`
	User                 string                        `json:"user,omitempty"`
	Verbosity            string                        `json:"verbosity,omitempty"`
	WebSearchOptions     *OpenAIChatWebSearchOptions   `json:"web_search_options,omitempty"`
}

func (r *OpenAIChatCompletionsRequest) GetPrompt() string {
	var builder strings.Builder
	for i := range r.Messages {
		if strings.ToLower(r.Messages[i].Role) != "user" || r.Messages[i].Content == nil {
			continue
		}
		appendOpenAIChatMessagePrompt(&builder, r.Messages[i].Content)
	}
	return builder.String()
}

func (r *OpenAIChatCompletionsRequest) FromMap(m map[string]any) error {
	if err := r.fromMapPart1(m); err != nil {
		return err
	}
	if err := r.fromMapPart2(m); err != nil {
		return err
	}
	return r.fromMapPart3(m)
}

func (r *OpenAIChatCompletionsRequest) fromMapPart1(m map[string]any) error {
	var err error
	r.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	r.Messages, err = decodeOpenAIChatMessageListFromMapField(m, "messages")
	if err != nil {
		return err
	}
	r.Audio, err = decodeOpenAIChatAudioParamPtrFromMapField(m, "audio")
	if err != nil {
		return err
	}
	r.FrequencyPenalty, err = floatPtrValue(m, "frequency_penalty")
	if err != nil {
		return err
	}
	r.FunctionCall, err = decodeOpenAIChatFunctionCallOptionPtrFromMapField(m, "function_call")
	if err != nil {
		return err
	}
	r.Functions, err = decodeOpenAIFunctionDefinitionListFromMapField(m, "functions")
	if err != nil {
		return err
	}
	r.LogitBias, err = mapStringFloat64Value(m, "logit_bias")
	if err != nil {
		return err
	}
	r.Logprobs, err = boolPtrValue(m, "logprobs")
	if err != nil {
		return err
	}
	r.MaxCompletionTokens, err = intValue(m, "max_completion_tokens")
	if err != nil {
		return err
	}
	r.MaxTokens, err = intValue(m, "max_tokens")
	if err != nil {
		return err
	}
	r.Metadata, err = mapStringStringValue(m, "metadata")
	if err != nil {
		return err
	}
	r.Modalities, err = stringSliceValue(m, "modalities")
	if err != nil {
		return err
	}
	r.N, err = intValue(m, "n")
	if err != nil {
		return err
	}
	r.ParallelToolCalls, err = boolPtrValue(m, "parallel_tool_calls")
	return err
}

func (r *OpenAIChatCompletionsRequest) fromMapPart2(m map[string]any) error {
	var err error
	r.Prediction, err = decodeOpenAIChatPredictionPtrFromMapField(m, "prediction")
	if err != nil {
		return err
	}
	r.PresencePenalty, err = floatPtrValue(m, "presence_penalty")
	if err != nil {
		return err
	}
	r.PromptCacheKey, err = stringValue(m, "prompt_cache_key")
	if err != nil {
		return err
	}
	r.PromptCacheRetention, err = stringValue(m, "prompt_cache_retention")
	if err != nil {
		return err
	}
	r.ReasoningEffort, err = stringValue(m, "reasoning_effort")
	if err != nil {
		return err
	}
	r.ResponseFormat, err = decodeOpenAIChatResponseFormatPtrFromMapField(m, "response_format")
	if err != nil {
		return err
	}
	r.SafetyIdentifier, err = stringValue(m, "safety_identifier")
	if err != nil {
		return err
	}
	r.Seed, err = intPtrValue(m, "seed")
	if err != nil {
		return err
	}
	r.ServiceTier, err = stringValue(m, "service_tier")
	if err != nil {
		return err
	}
	r.Stop, err = decodeOpenAIChatStopPtrFromMapField(m, "stop")
	if err != nil {
		return err
	}
	r.Store, err = boolPtrValue(m, "store")
	return err
}

func (r *OpenAIChatCompletionsRequest) fromMapPart3(m map[string]any) error {
	var err error
	r.Stream, err = boolValue(m, "stream")
	if err != nil {
		return err
	}
	r.StreamOptions, err = decodeOpenAIChatStreamOptionsPtrFromMapField(m, "stream_options")
	if err != nil {
		return err
	}
	r.Temperature, err = floatPtrValue(m, "temperature")
	if err != nil {
		return err
	}
	r.ToolChoice, err = decodeOpenAIChatToolChoicePtrFromMapField(m, "tool_choice")
	if err != nil {
		return err
	}
	r.Tools, err = decodeOpenAIChatToolListFromMapField(m, "tools")
	if err != nil {
		return err
	}
	r.TopLogprobs, err = intValue(m, "top_logprobs")
	if err != nil {
		return err
	}
	r.TopP, err = floatPtrValue(m, "top_p")
	if err != nil {
		return err
	}
	r.User, err = stringValue(m, "user")
	if err != nil {
		return err
	}
	r.Verbosity, err = stringValue(m, "verbosity")
	if err != nil {
		return err
	}
	r.WebSearchOptions, err = decodeOpenAIChatWebSearchOptionsPtrFromMapField(m, "web_search_options")
	return err
}

func (r *OpenAIChatCompletionsRequest) ToMap() (map[string]any, error) {
	out := map[string]any{"model": r.Model}
	if err := r.toMapPart1(out); err != nil {
		return nil, err
	}
	if err := r.toMapPart2(out); err != nil {
		return nil, err
	}
	if err := r.toMapPart3(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OpenAIChatCompletionsRequest) toMapPart1(out map[string]any) error {
	if len(r.Messages) > 0 {
		messages, err := openAIChatMessageListToMaps(r.Messages)
		if err != nil {
			return err
		}
		out["messages"] = messages
	}
	if r.Audio != nil {
		audio, err := r.Audio.ToMap()
		if err != nil {
			return err
		}
		out["audio"] = audio
	}
	if r.FrequencyPenalty != nil {
		out["frequency_penalty"] = *r.FrequencyPenalty
	}
	if r.FunctionCall != nil {
		functionCall, err := r.FunctionCall.ToAny()
		if err != nil {
			return err
		}
		out["function_call"] = functionCall
	}
	if len(r.Functions) > 0 {
		functions, err := openAIFunctionDefinitionListToMaps(r.Functions)
		if err != nil {
			return err
		}
		out["functions"] = functions
	}
	if r.LogitBias != nil {
		logitBias := make(map[string]any, len(r.LogitBias))
		for k, v := range r.LogitBias {
			logitBias[k] = v
		}
		out["logit_bias"] = logitBias
	}
	if r.Logprobs != nil {
		out["logprobs"] = *r.Logprobs
	}
	setMapInt(out, "max_completion_tokens", r.MaxCompletionTokens)
	setMapInt(out, "max_tokens", r.MaxTokens)
	if r.Metadata != nil {
		metadata := make(map[string]any, len(r.Metadata))
		for k, v := range r.Metadata {
			metadata[k] = v
		}
		out["metadata"] = metadata
	}
	setMapStringSlice(out, "modalities", r.Modalities)
	setMapInt(out, "n", r.N)
	if r.ParallelToolCalls != nil {
		out["parallel_tool_calls"] = *r.ParallelToolCalls
	}
	return nil
}

func (r *OpenAIChatCompletionsRequest) toMapPart2(out map[string]any) error {
	if r.Prediction != nil {
		prediction, err := r.Prediction.ToMap()
		if err != nil {
			return err
		}
		out["prediction"] = prediction
	}
	if r.PresencePenalty != nil {
		out["presence_penalty"] = *r.PresencePenalty
	}
	setMapString(out, "prompt_cache_key", r.PromptCacheKey)
	setMapString(out, "prompt_cache_retention", r.PromptCacheRetention)
	setMapString(out, "reasoning_effort", r.ReasoningEffort)
	if r.ResponseFormat != nil {
		responseFormat, err := r.ResponseFormat.ToMap()
		if err != nil {
			return err
		}
		out["response_format"] = responseFormat
	}
	setMapString(out, "safety_identifier", r.SafetyIdentifier)
	if r.Seed != nil {
		out["seed"] = *r.Seed
	}
	setMapString(out, "service_tier", r.ServiceTier)
	if r.Stop != nil {
		stop, err := r.Stop.ToAny()
		if err != nil {
			return err
		}
		out["stop"] = stop
	}
	if r.Store != nil {
		out["store"] = *r.Store
	}
	return nil
}

func (r *OpenAIChatCompletionsRequest) toMapPart3(out map[string]any) error {
	setMapBool(out, "stream", r.Stream)
	if r.StreamOptions != nil {
		streamOptions, err := r.StreamOptions.ToMap()
		if err != nil {
			return err
		}
		out["stream_options"] = streamOptions
	}
	if r.Temperature != nil {
		out["temperature"] = *r.Temperature
	}
	if r.ToolChoice != nil {
		toolChoice, err := r.ToolChoice.ToAny()
		if err != nil {
			return err
		}
		out["tool_choice"] = toolChoice
	}
	if len(r.Tools) > 0 {
		tools, err := openAIChatToolListToMaps(r.Tools)
		if err != nil {
			return err
		}
		out["tools"] = tools
	}
	setMapInt(out, "top_logprobs", r.TopLogprobs)
	if r.TopP != nil {
		out["top_p"] = *r.TopP
	}
	setMapString(out, "user", r.User)
	setMapString(out, "verbosity", r.Verbosity)
	if r.WebSearchOptions != nil {
		webSearchOptions, err := r.WebSearchOptions.ToMap()
		if err != nil {
			return err
		}
		out["web_search_options"] = webSearchOptions
	}
	return nil
}

type OpenAIChatMessage struct {
	// Role controls which companion fields are expected:
	// - system: content
	// - developer: content
	// - user: content
	// - assistant: content and/or tool_calls, optionally refusal or audio
	// - tool: tool_call_id and content
	// - function: name and content
	Role         string                    `json:"role"`
	Content      *OpenAIChatMessageContent `json:"content,omitempty"`
	Audio        *OpenAIChatAudio          `json:"audio,omitempty"`
	Name         string                    `json:"name,omitempty"`
	Refusal      string                    `json:"refusal,omitempty"`
	Annotations  []OpenAIChatAnnotation    `json:"annotations,omitempty"`
	ToolCallID   string                    `json:"tool_call_id,omitempty"`
	ToolCalls    []OpenAIChatToolCall      `json:"tool_calls,omitempty"`
	FunctionCall *OpenAIFunctionCall       `json:"function_call,omitempty"`
}

func (m *OpenAIChatMessage) FromMap(v map[string]any) error {
	var err error
	m.Role, err = stringValue(v, "role")
	if err != nil {
		return err
	}
	m.Content, err = decodeOpenAIChatMessageContentPtrFromMapField(v, "content")
	if err != nil {
		return err
	}
	m.Audio, err = decodeOpenAIChatAudioPtrFromMapField(v, "audio")
	if err != nil {
		return err
	}
	m.Name, err = stringValue(v, "name")
	if err != nil {
		return err
	}
	m.Refusal, err = stringValue(v, "refusal")
	if err != nil {
		return err
	}
	m.Annotations, err = decodeOpenAIChatAnnotationListFromMapField(v, "annotations")
	if err != nil {
		return err
	}
	m.ToolCallID, err = stringValue(v, "tool_call_id")
	if err != nil {
		return err
	}
	m.ToolCalls, err = decodeOpenAIChatToolCallListFromMapField(v, "tool_calls")
	if err != nil {
		return err
	}
	m.FunctionCall, err = decodeOpenAIFunctionCallPtrFromMapField(v, "function_call")
	return err
}

func (m *OpenAIChatMessage) ToMap() (map[string]any, error) {
	out := map[string]any{"role": m.Role}
	if m.Content != nil {
		content, err := m.Content.ToAny()
		if err != nil {
			return nil, err
		}
		out["content"] = content
	}
	if m.Audio != nil {
		audio, err := m.Audio.ToMap()
		if err != nil {
			return nil, err
		}
		out["audio"] = audio
	}
	setMapString(out, "name", m.Name)
	setMapString(out, "refusal", m.Refusal)
	if len(m.Annotations) > 0 {
		annotations, err := openAIChatAnnotationListToMaps(m.Annotations)
		if err != nil {
			return nil, err
		}
		out["annotations"] = annotations
	}
	setMapString(out, "tool_call_id", m.ToolCallID)
	if len(m.ToolCalls) > 0 {
		toolCalls, err := openAIChatToolCallListToMaps(m.ToolCalls)
		if err != nil {
			return nil, err
		}
		out["tool_calls"] = toolCalls
	}
	if m.FunctionCall != nil {
		functionCall, err := m.FunctionCall.ToMap()
		if err != nil {
			return nil, err
		}
		out["function_call"] = functionCall
	}
	return out, nil
}

type OpenAIChatAudio struct {
	ID         string `json:"id,omitempty"`
	Data       string `json:"data,omitempty"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
	Transcript string `json:"transcript,omitempty"`
}

func (a *OpenAIChatAudio) FromMap(m map[string]any) error {
	var err error
	a.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	a.Data, err = stringValue(m, "data")
	if err != nil {
		return err
	}
	if m["expires_at"] != nil {
		expiresAt, convErr := toInt64(m["expires_at"])
		if convErr != nil {
			return convErr
		}
		a.ExpiresAt = expiresAt
	}
	a.Transcript, err = stringValue(m, "transcript")
	return err
}

func (a *OpenAIChatAudio) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "id", a.ID)
	setMapString(out, "data", a.Data)
	if a.ExpiresAt != 0 {
		out["expires_at"] = a.ExpiresAt
	}
	setMapString(out, "transcript", a.Transcript)
	return out, nil
}

type OpenAIChatTool struct {
	Type     string                          `json:"type"`
	Function *OpenAIFunctionDefinition       `json:"function,omitempty"`
	Custom   *OpenAIChatCustomToolDefinition `json:"custom,omitempty"`
}

func (t *OpenAIChatTool) FromMap(m map[string]any) error {
	var err error
	t.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	t.Function, err = decodeOpenAIFunctionDefinitionPtrFromMapField(m, "function")
	if err != nil {
		return err
	}
	t.Custom, err = decodeOpenAIChatCustomToolDefinitionPtrFromMapField(m, "custom")
	return err
}

func (t *OpenAIChatTool) ToMap() (map[string]any, error) {
	out := map[string]any{"type": t.Type}
	if t.Function != nil {
		function, err := t.Function.ToMap()
		if err != nil {
			return nil, err
		}
		out["function"] = function
	}
	if t.Custom != nil {
		custom, err := t.Custom.ToMap()
		if err != nil {
			return nil, err
		}
		out["custom"] = custom
	}
	return out, nil
}

type OpenAIChatToolCall struct {
	ID       string                    `json:"id,omitempty"`
	Type     string                    `json:"type,omitempty"`
	Function *OpenAIFunctionCall       `json:"function,omitempty"`
	Custom   *OpenAIChatCustomToolCall `json:"custom,omitempty"`
}

func (c *OpenAIChatToolCall) FromMap(m map[string]any) error {
	var err error
	c.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.Function, err = decodeOpenAIFunctionCallPtrFromMapField(m, "function")
	if err != nil {
		return err
	}
	c.Custom, err = decodeOpenAIChatCustomToolCallPtrFromMapField(m, "custom")
	return err
}

func (c *OpenAIChatToolCall) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "id", c.ID)
	setMapString(out, "type", c.Type)
	if c.Function != nil {
		function, err := c.Function.ToMap()
		if err != nil {
			return nil, err
		}
		out["function"] = function
	}
	if c.Custom != nil {
		custom, err := c.Custom.ToMap()
		if err != nil {
			return nil, err
		}
		out["custom"] = custom
	}
	return out, nil
}

type OpenAIChatMessageContent struct {
	Text  *string                 `json:"-"`
	Parts []OpenAIChatContentPart `json:"-"`
}

func (c *OpenAIChatMessageContent) FromAny(v any) error {
	if v == nil {
		c.Text = nil
		c.Parts = nil
		return nil
	}
	if text, ok := v.(string); ok {
		c.Text = &text
		c.Parts = nil
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	parts := make([]OpenAIChatContentPart, 0, len(items))
	for _, item := range items {
		mv, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var part OpenAIChatContentPart
		if err := part.FromMap(mv); err != nil {
			return err
		}
		parts = append(parts, part)
	}
	c.Text = nil
	c.Parts = parts
	return nil
}

func (c *OpenAIChatMessageContent) ToAny() (any, error) {
	if c == nil {
		return nil, nil
	}
	if c.Text != nil {
		return *c.Text, nil
	}
	if len(c.Parts) == 0 {
		return nil, nil
	}
	parts, err := openAIChatContentPartListToMaps(c.Parts)
	if err != nil {
		return nil, err
	}
	return parts, nil
}

type OpenAIChatContentPart struct {
	Type       string                `json:"type,omitempty"`
	Text       string                `json:"text,omitempty"`
	Refusal    string                `json:"refusal,omitempty"`
	ImageURL   *OpenAIChatImageURL   `json:"image_url,omitempty"`
	InputAudio *OpenAIChatInputAudio `json:"input_audio,omitempty"`
	File       *OpenAIChatFileInput  `json:"file,omitempty"`
}

func (p *OpenAIChatContentPart) FromMap(m map[string]any) error {
	var err error
	p.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	p.Text, err = stringValue(m, "text")
	if err != nil {
		return err
	}
	p.Refusal, err = stringValue(m, "refusal")
	if err != nil {
		return err
	}
	p.ImageURL, err = decodeOpenAIChatImageURLPtrFromMapField(m, "image_url")
	if err != nil {
		return err
	}
	p.InputAudio, err = decodeOpenAIChatInputAudioPtrFromMapField(m, "input_audio")
	if err != nil {
		return err
	}
	p.File, err = decodeOpenAIChatFileInputPtrFromMapField(m, "file")
	return err
}

func (p *OpenAIChatContentPart) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", p.Type)
	setMapString(out, "text", p.Text)
	setMapString(out, "refusal", p.Refusal)
	if p.ImageURL != nil {
		imageURL, err := p.ImageURL.ToMap()
		if err != nil {
			return nil, err
		}
		out["image_url"] = imageURL
	}
	if p.InputAudio != nil {
		inputAudio, err := p.InputAudio.ToMap()
		if err != nil {
			return nil, err
		}
		out["input_audio"] = inputAudio
	}
	if p.File != nil {
		file, err := p.File.ToMap()
		if err != nil {
			return nil, err
		}
		out["file"] = file
	}
	return out, nil
}

type OpenAIChatAudioParam struct {
	Format string             `json:"format,omitempty"`
	Voice  OpenAIChatVoiceRef `json:"voice,omitempty"`
}

func (p *OpenAIChatAudioParam) FromMap(m map[string]any) error {
	var err error
	p.Format, err = stringValue(m, "format")
	if err != nil {
		return err
	}
	if v, ok := mapValue(m, "voice"); ok {
		if err := p.Voice.FromAny(v); err != nil {
			return err
		}
	}
	return nil
}

func (p *OpenAIChatAudioParam) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "format", p.Format)
	if voice, err := p.Voice.ToAny(); err != nil {
		return nil, err
	} else if voice != nil {
		out["voice"] = voice
	}
	return out, nil
}

type OpenAIChatVoiceRef struct {
	Name string `json:"-"`
	ID   string `json:"-"`
}

func (v *OpenAIChatVoiceRef) FromAny(value any) error {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		v.Name = typed
		v.ID = ""
		return nil
	case map[string]any:
		id, err := stringValue(typed, "id")
		if err != nil {
			return err
		}
		v.Name = ""
		v.ID = id
		return nil
	default:
		return nil
	}
}

func (v *OpenAIChatVoiceRef) ToAny() (any, error) {
	if v == nil || (v.Name == "" && v.ID == "") {
		return nil, nil
	}
	if v.ID != "" {
		return map[string]any{"id": v.ID}, nil
	}
	return v.Name, nil
}

type OpenAIChatFunctionCallOption struct {
	Mode string `json:"-"`
	Name string `json:"-"`
}

func (o *OpenAIChatFunctionCallOption) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		o.Mode = typed
		o.Name = ""
		return nil
	case map[string]any:
		name, err := stringValue(typed, "name")
		if err != nil {
			return err
		}
		o.Mode = ""
		o.Name = name
		return nil
	default:
		return nil
	}
}

func (o *OpenAIChatFunctionCallOption) ToAny() (any, error) {
	if o == nil || (o.Mode == "" && o.Name == "") {
		return nil, nil
	}
	if o.Name != "" {
		return map[string]any{"name": o.Name}, nil
	}
	return o.Mode, nil
}

type OpenAIFunctionDefinition struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
}

func (d *OpenAIFunctionDefinition) FromMap(m map[string]any) error {
	var err error
	d.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	d.Description, err = stringValue(m, "description")
	if err != nil {
		return err
	}
	d.Parameters, err = mapStringAnyValue(m, "parameters")
	if err != nil {
		return err
	}
	d.Strict, err = boolPtrValue(m, "strict")
	return err
}

func (d *OpenAIFunctionDefinition) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "name", d.Name)
	setMapString(out, "description", d.Description)
	if d.Parameters != nil {
		out["parameters"] = d.Parameters
	}
	if d.Strict != nil {
		out["strict"] = *d.Strict
	}
	return out, nil
}

type OpenAIChatPrediction struct {
	Type    string                    `json:"type,omitempty"`
	Content *OpenAIChatMessageContent `json:"content,omitempty"`
}

func (p *OpenAIChatPrediction) FromMap(m map[string]any) error {
	var err error
	p.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	p.Content, err = decodeOpenAIChatMessageContentPtrFromMapField(m, "content")
	return err
}

func (p *OpenAIChatPrediction) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", p.Type)
	if p.Content != nil {
		content, err := p.Content.ToAny()
		if err != nil {
			return nil, err
		}
		out["content"] = content
	}
	return out, nil
}

type OpenAIChatResponseFormat struct {
	Type       string                              `json:"type,omitempty"`
	JSONSchema *OpenAIChatResponseFormatJSONSchema `json:"json_schema,omitempty"`
}

func (r *OpenAIChatResponseFormat) FromMap(m map[string]any) error {
	var err error
	r.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	r.JSONSchema, err = decodeOpenAIChatResponseFormatJSONSchemaPtrFromMapField(m, "json_schema")
	return err
}

func (r *OpenAIChatResponseFormat) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", r.Type)
	if r.JSONSchema != nil {
		jsonSchema, err := r.JSONSchema.ToMap()
		if err != nil {
			return nil, err
		}
		out["json_schema"] = jsonSchema
	}
	return out, nil
}

type OpenAIChatResponseFormatJSONSchema struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
}

func (r *OpenAIChatResponseFormatJSONSchema) FromMap(m map[string]any) error {
	var err error
	r.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	r.Description, err = stringValue(m, "description")
	if err != nil {
		return err
	}
	r.Schema, err = mapStringAnyValue(m, "schema")
	if err != nil {
		return err
	}
	r.Strict, err = boolPtrValue(m, "strict")
	return err
}

func (r *OpenAIChatResponseFormatJSONSchema) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "name", r.Name)
	setMapString(out, "description", r.Description)
	if r.Schema != nil {
		out["schema"] = r.Schema
	}
	if r.Strict != nil {
		out["strict"] = *r.Strict
	}
	return out, nil
}

type OpenAIChatStop struct {
	String string   `json:"-"`
	List   []string `json:"-"`
}

func (s *OpenAIChatStop) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		s.String = typed
		s.List = nil
		return nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			str, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, str)
		}
		s.String = ""
		s.List = out
		return nil
	default:
		return nil
	}
}

func (s *OpenAIChatStop) ToAny() (any, error) {
	if s == nil || (s.String == "" && len(s.List) == 0) {
		return nil, nil
	}
	if len(s.List) > 0 {
		items := make([]any, 0, len(s.List))
		for _, item := range s.List {
			items = append(items, item)
		}
		return items, nil
	}
	return s.String, nil
}

type OpenAIChatStreamOptions struct {
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
	IncludeUsage       *bool `json:"include_usage,omitempty"`
}

func (o *OpenAIChatStreamOptions) FromMap(m map[string]any) error {
	var err error
	o.IncludeObfuscation, err = boolPtrValue(m, "include_obfuscation")
	if err != nil {
		return err
	}
	o.IncludeUsage, err = boolPtrValue(m, "include_usage")
	return err
}

func (o *OpenAIChatStreamOptions) ToMap() (map[string]any, error) {
	out := map[string]any{}
	if o.IncludeObfuscation != nil {
		out["include_obfuscation"] = *o.IncludeObfuscation
	}
	if o.IncludeUsage != nil {
		out["include_usage"] = *o.IncludeUsage
	}
	return out, nil
}

type OpenAIChatToolChoice struct {
	Mode         string                             `json:"-"`
	AllowedTools *OpenAIChatAllowedToolChoice       `json:"-"`
	Function     *OpenAIChatNamedFunctionToolChoice `json:"-"`
	Custom       *OpenAIChatNamedCustomToolChoice   `json:"-"`
}

func (c *OpenAIChatToolChoice) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		c.Mode = typed
		return nil
	case map[string]any:
		typeValue, err := stringValue(typed, "type")
		if err != nil {
			return err
		}
		switch typeValue {
		case "allowed_tools":
			value := &OpenAIChatAllowedToolChoice{}
			if err := value.FromMap(typed); err != nil {
				return err
			}
			c.AllowedTools = value
		case "function":
			value := &OpenAIChatNamedFunctionToolChoice{}
			if err := value.FromMap(typed); err != nil {
				return err
			}
			c.Function = value
		case "custom":
			value := &OpenAIChatNamedCustomToolChoice{}
			if err := value.FromMap(typed); err != nil {
				return err
			}
			c.Custom = value
		default:
			c.Mode = typeValue
		}
		return nil
	default:
		return nil
	}
}

func (c *OpenAIChatToolChoice) ToAny() (any, error) {
	if c == nil {
		return nil, nil
	}
	switch {
	case c.AllowedTools != nil:
		return c.AllowedTools.ToMap()
	case c.Function != nil:
		return c.Function.ToMap()
	case c.Custom != nil:
		return c.Custom.ToMap()
	case c.Mode != "":
		return c.Mode, nil
	default:
		return nil, nil
	}
}

type OpenAIChatAllowedToolChoice struct {
	Type         string                     `json:"type,omitempty"`
	AllowedTools OpenAIChatAllowedToolsSpec `json:"allowed_tools,omitempty"`
}

func (c *OpenAIChatAllowedToolChoice) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.AllowedTools, err = decodeOpenAIChatAllowedToolsSpecFromMapField(m, "allowed_tools")
	return err
}

func (c *OpenAIChatAllowedToolChoice) ToMap() (map[string]any, error) {
	allowedTools, err := c.AllowedTools.ToMap()
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": c.Type, "allowed_tools": allowedTools}, nil
}

type OpenAIChatAllowedToolsSpec struct {
	Mode  string           `json:"mode,omitempty"`
	Tools []OpenAIChatTool `json:"tools,omitempty"`
}

func (s *OpenAIChatAllowedToolsSpec) FromMap(m map[string]any) error {
	var err error
	s.Mode, err = stringValue(m, "mode")
	if err != nil {
		return err
	}
	s.Tools, err = decodeOpenAIChatToolListFromMapField(m, "tools")
	return err
}

func (s *OpenAIChatAllowedToolsSpec) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "mode", s.Mode)
	if len(s.Tools) > 0 {
		tools, err := openAIChatToolListToMaps(s.Tools)
		if err != nil {
			return nil, err
		}
		out["tools"] = tools
	}
	return out, nil
}

type OpenAIChatNamedFunctionToolChoice struct {
	Type     string                          `json:"type,omitempty"`
	Function OpenAIChatNamedFunctionSelector `json:"function,omitempty"`
}

func (c *OpenAIChatNamedFunctionToolChoice) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.Function, err = decodeOpenAIChatNamedFunctionSelectorFromMapField(m, "function")
	return err
}

func (c *OpenAIChatNamedFunctionToolChoice) ToMap() (map[string]any, error) {
	function, err := c.Function.ToMap()
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": c.Type, "function": function}, nil
}

type OpenAIChatNamedFunctionSelector struct {
	Name string `json:"name,omitempty"`
}

func (s *OpenAIChatNamedFunctionSelector) FromMap(m map[string]any) error {
	var err error
	s.Name, err = stringValue(m, "name")
	return err
}

func (s *OpenAIChatNamedFunctionSelector) ToMap() (map[string]any, error) {
	return map[string]any{"name": s.Name}, nil
}

type OpenAIChatNamedCustomToolChoice struct {
	Type   string                        `json:"type,omitempty"`
	Custom OpenAIChatNamedCustomSelector `json:"custom,omitempty"`
}

func (c *OpenAIChatNamedCustomToolChoice) FromMap(m map[string]any) error {
	var err error
	c.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	c.Custom, err = decodeOpenAIChatNamedCustomSelectorFromMapField(m, "custom")
	return err
}

func (c *OpenAIChatNamedCustomToolChoice) ToMap() (map[string]any, error) {
	custom, err := c.Custom.ToMap()
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": c.Type, "custom": custom}, nil
}

type OpenAIChatNamedCustomSelector struct {
	Name string `json:"name,omitempty"`
}

func (s *OpenAIChatNamedCustomSelector) FromMap(m map[string]any) error {
	var err error
	s.Name, err = stringValue(m, "name")
	return err
}

func (s *OpenAIChatNamedCustomSelector) ToMap() (map[string]any, error) {
	return map[string]any{"name": s.Name}, nil
}

type OpenAIChatWebSearchOptions struct {
	SearchContextSize string                  `json:"search_context_size,omitempty"`
	UserLocation      *OpenAIChatUserLocation `json:"user_location,omitempty"`
}

func (o *OpenAIChatWebSearchOptions) FromMap(m map[string]any) error {
	var err error
	o.SearchContextSize, err = stringValue(m, "search_context_size")
	if err != nil {
		return err
	}
	o.UserLocation, err = decodeOpenAIChatUserLocationPtrFromMapField(m, "user_location")
	return err
}

func (o *OpenAIChatWebSearchOptions) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "search_context_size", o.SearchContextSize)
	if o.UserLocation != nil {
		userLocation, err := o.UserLocation.ToMap()
		if err != nil {
			return nil, err
		}
		out["user_location"] = userLocation
	}
	return out, nil
}

type OpenAIChatUserLocation struct {
	Type        string                         `json:"type,omitempty"`
	Approximate *OpenAIChatApproximateLocation `json:"approximate,omitempty"`
}

func (l *OpenAIChatUserLocation) FromMap(m map[string]any) error {
	var err error
	l.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	l.Approximate, err = decodeOpenAIChatApproximateLocationPtrFromMapField(m, "approximate")
	return err
}

func (l *OpenAIChatUserLocation) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", l.Type)
	if l.Approximate != nil {
		approximate, err := l.Approximate.ToMap()
		if err != nil {
			return nil, err
		}
		out["approximate"] = approximate
	}
	return out, nil
}

type OpenAIChatApproximateLocation struct {
	City     string `json:"city,omitempty"`
	Country  string `json:"country,omitempty"`
	Region   string `json:"region,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

func (l *OpenAIChatApproximateLocation) FromMap(m map[string]any) error {
	var err error
	l.City, err = stringValue(m, "city")
	if err != nil {
		return err
	}
	l.Country, err = stringValue(m, "country")
	if err != nil {
		return err
	}
	l.Region, err = stringValue(m, "region")
	if err != nil {
		return err
	}
	l.Timezone, err = stringValue(m, "timezone")
	return err
}

func (l *OpenAIChatApproximateLocation) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "city", l.City)
	setMapString(out, "country", l.Country)
	setMapString(out, "region", l.Region)
	setMapString(out, "timezone", l.Timezone)
	return out, nil
}

type OpenAIChatCustomToolDefinition struct {
	Name        string                      `json:"name,omitempty"`
	Description string                      `json:"description,omitempty"`
	Format      *OpenAIChatCustomToolFormat `json:"format,omitempty"`
}

func (d *OpenAIChatCustomToolDefinition) FromMap(m map[string]any) error {
	var err error
	d.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	d.Description, err = stringValue(m, "description")
	if err != nil {
		return err
	}
	d.Format, err = decodeOpenAIChatCustomToolFormatPtrFromMapField(m, "format")
	return err
}

func (d *OpenAIChatCustomToolDefinition) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "name", d.Name)
	setMapString(out, "description", d.Description)
	if d.Format != nil {
		format, err := d.Format.ToMap()
		if err != nil {
			return nil, err
		}
		out["format"] = format
	}
	return out, nil
}

type OpenAIChatCustomToolFormat struct {
	Type    string                      `json:"type,omitempty"`
	Grammar *OpenAIChatCustomGrammarRef `json:"grammar,omitempty"`
}

func (f *OpenAIChatCustomToolFormat) FromMap(m map[string]any) error {
	var err error
	f.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	f.Grammar, err = decodeOpenAIChatCustomGrammarRefPtrFromMapField(m, "grammar")
	return err
}

func (f *OpenAIChatCustomToolFormat) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", f.Type)
	if f.Grammar != nil {
		grammar, err := f.Grammar.ToMap()
		if err != nil {
			return nil, err
		}
		out["grammar"] = grammar
	}
	return out, nil
}

type OpenAIChatCustomGrammarRef struct {
	Definition string `json:"definition,omitempty"`
	Syntax     string `json:"syntax,omitempty"`
}

func (g *OpenAIChatCustomGrammarRef) FromMap(m map[string]any) error {
	var err error
	g.Definition, err = stringValue(m, "definition")
	if err != nil {
		return err
	}
	g.Syntax, err = stringValue(m, "syntax")
	return err
}

func (g *OpenAIChatCustomGrammarRef) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "definition", g.Definition)
	setMapString(out, "syntax", g.Syntax)
	return out, nil
}

type OpenAIChatCustomToolCall struct {
	Input string `json:"input,omitempty"`
	Name  string `json:"name,omitempty"`
}

func (c *OpenAIChatCustomToolCall) FromMap(m map[string]any) error {
	var err error
	c.Input, err = stringValue(m, "input")
	if err != nil {
		return err
	}
	c.Name, err = stringValue(m, "name")
	return err
}

func (c *OpenAIChatCustomToolCall) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "input", c.Input)
	setMapString(out, "name", c.Name)
	return out, nil
}

type OpenAIChatImageURL struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func (i *OpenAIChatImageURL) FromMap(m map[string]any) error {
	var err error
	i.URL, err = stringValue(m, "url")
	if err != nil {
		return err
	}
	i.Detail, err = stringValue(m, "detail")
	return err
}

func (i *OpenAIChatImageURL) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "url", i.URL)
	setMapString(out, "detail", i.Detail)
	return out, nil
}

type OpenAIChatInputAudio struct {
	Data   string `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
}

func (i *OpenAIChatInputAudio) FromMap(m map[string]any) error {
	var err error
	i.Data, err = stringValue(m, "data")
	if err != nil {
		return err
	}
	i.Format, err = stringValue(m, "format")
	return err
}

func (i *OpenAIChatInputAudio) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "data", i.Data)
	setMapString(out, "format", i.Format)
	return out, nil
}

type OpenAIChatFileInput struct {
	FileData string `json:"file_data,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
}

func (f *OpenAIChatFileInput) FromMap(m map[string]any) error {
	var err error
	f.FileData, err = stringValue(m, "file_data")
	if err != nil {
		return err
	}
	f.FileID, err = stringValue(m, "file_id")
	if err != nil {
		return err
	}
	f.Filename, err = stringValue(m, "filename")
	return err
}

func (f *OpenAIChatFileInput) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "file_data", f.FileData)
	setMapString(out, "file_id", f.FileID)
	setMapString(out, "filename", f.Filename)
	return out, nil
}

type OpenAIChatAnnotation struct {
	Type        string                 `json:"type,omitempty"`
	URLCitation *OpenAIChatURLCitation `json:"url_citation,omitempty"`
}

func (a *OpenAIChatAnnotation) FromMap(m map[string]any) error {
	var err error
	a.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	a.URLCitation, err = decodeOpenAIChatURLCitationPtrFromMapField(m, "url_citation")
	return err
}

func (a *OpenAIChatAnnotation) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", a.Type)
	if a.URLCitation != nil {
		urlCitation, err := a.URLCitation.ToMap()
		if err != nil {
			return nil, err
		}
		out["url_citation"] = urlCitation
	}
	return out, nil
}

type OpenAIChatURLCitation struct {
	EndIndex   int    `json:"end_index,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	Title      string `json:"title,omitempty"`
	URL        string `json:"url,omitempty"`
}

func (c *OpenAIChatURLCitation) FromMap(m map[string]any) error {
	var err error
	c.EndIndex, err = intValue(m, "end_index")
	if err != nil {
		return err
	}
	c.StartIndex, err = intValue(m, "start_index")
	if err != nil {
		return err
	}
	c.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	c.URL, err = stringValue(m, "url")
	return err
}

func (c *OpenAIChatURLCitation) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "end_index", c.EndIndex)
	setMapInt(out, "start_index", c.StartIndex)
	setMapString(out, "title", c.Title)
	setMapString(out, "url", c.URL)
	return out, nil
}

type OpenAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func (f *OpenAIFunctionCall) FromMap(m map[string]any) error {
	var err error
	f.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	f.Arguments, err = stringValue(m, "arguments")
	return err
}

func (f *OpenAIFunctionCall) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "name", f.Name)
	setMapString(out, "arguments", f.Arguments)
	return out, nil
}

type OpenAITokenDetails struct {
	CachedTokens             int `json:"cached_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

func (d *OpenAITokenDetails) FromMap(m map[string]any) error {
	var err error
	d.CachedTokens, err = intValue(m, "cached_tokens")
	if err != nil {
		return err
	}
	d.AudioTokens, err = intValue(m, "audio_tokens")
	if err != nil {
		return err
	}
	d.ReasoningTokens, err = intValue(m, "reasoning_tokens")
	if err != nil {
		return err
	}
	d.AcceptedPredictionTokens, err = intValue(m, "accepted_prediction_tokens")
	if err != nil {
		return err
	}
	d.RejectedPredictionTokens, err = intValue(m, "rejected_prediction_tokens")
	return err
}

func (d *OpenAITokenDetails) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "cached_tokens", d.CachedTokens)
	setMapInt(out, "audio_tokens", d.AudioTokens)
	setMapInt(out, "reasoning_tokens", d.ReasoningTokens)
	setMapInt(out, "accepted_prediction_tokens", d.AcceptedPredictionTokens)
	setMapInt(out, "rejected_prediction_tokens", d.RejectedPredictionTokens)
	return out, nil
}

type OpenAIChatCompletionsUsage struct {
	PromptTokens            int                 `json:"prompt_tokens,omitempty"`
	CompletionTokens        int                 `json:"completion_tokens,omitempty"`
	TotalTokens             int                 `json:"total_tokens,omitempty"`
	PromptTokensDetails     *OpenAITokenDetails `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *OpenAITokenDetails `json:"completion_tokens_details,omitempty"`
}

func (u *OpenAIChatCompletionsUsage) FromMap(m map[string]any) error {
	var err error
	u.PromptTokens, err = intValue(m, "prompt_tokens")
	if err != nil {
		return err
	}
	u.CompletionTokens, err = intValue(m, "completion_tokens")
	if err != nil {
		return err
	}
	u.TotalTokens, err = intValue(m, "total_tokens")
	if err != nil {
		return err
	}
	u.PromptTokensDetails, err = decodeOpenAITokenDetailsPtrFromMapField(m, "prompt_tokens_details")
	if err != nil {
		return err
	}
	u.CompletionTokensDetails, err = decodeOpenAITokenDetailsPtrFromMapField(m, "completion_tokens_details")
	return err
}

func (u *OpenAIChatCompletionsUsage) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "prompt_tokens", u.PromptTokens)
	setMapInt(out, "completion_tokens", u.CompletionTokens)
	setMapInt(out, "total_tokens", u.TotalTokens)
	if u.PromptTokensDetails != nil {
		promptDetails, err := u.PromptTokensDetails.ToMap()
		if err != nil {
			return nil, err
		}
		out["prompt_tokens_details"] = promptDetails
	}
	if u.CompletionTokensDetails != nil {
		completionDetails, err := u.CompletionTokensDetails.ToMap()
		if err != nil {
			return nil, err
		}
		out["completion_tokens_details"] = completionDetails
	}
	return out, nil
}

type OpenAIChatCompletionsChoice struct {
	Index        int               `json:"index,omitempty"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

func (c *OpenAIChatCompletionsChoice) FromMap(m map[string]any) error {
	var err error
	c.Index, err = intValue(m, "index")
	if err != nil {
		return err
	}
	c.Message, err = decodeOpenAIChatMessageFromMapField(m, "message")
	if err != nil {
		return err
	}
	c.FinishReason, err = stringValue(m, "finish_reason")
	return err
}

func (c *OpenAIChatCompletionsChoice) ToMap() (map[string]any, error) {
	message, err := c.Message.ToMap()
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"index":   c.Index,
		"message": message,
	}
	setMapString(out, "finish_reason", c.FinishReason)
	return out, nil
}

type OpenAIChatCompletionsResponse struct {
	ID                string                        `json:"id"`
	Object            string                        `json:"object"`
	Created           int64                         `json:"created"`
	Model             string                        `json:"model,omitempty"`
	Choices           []OpenAIChatCompletionsChoice `json:"choices"`
	Usage             *OpenAIChatCompletionsUsage   `json:"usage,omitempty"`
	SystemFingerprint string                        `json:"system_fingerprint,omitempty"`
	ServiceTier       string                        `json:"service_tier,omitempty"`
}

func (r *OpenAIChatCompletionsResponse) FromMap(m map[string]any) error {
	var err error
	r.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	r.Object, err = stringValue(m, "object")
	if err != nil {
		return err
	}
	created, err := toInt64(m["created"])
	if err == nil {
		r.Created = created
	} else if _, ok := m["created"]; ok && m["created"] != nil {
		return err
	}
	r.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	r.Choices, err = decodeOpenAIChatCompletionsChoiceListFromMapField(m, "choices")
	if err != nil {
		return err
	}
	r.Usage, err = decodeOpenAIChatCompletionsUsagePtrFromMapField(m, "usage")
	if err != nil {
		return err
	}
	r.SystemFingerprint, err = stringValue(m, "system_fingerprint")
	if err != nil {
		return err
	}
	r.ServiceTier, err = stringValue(m, "service_tier")
	return err
}

func (r *OpenAIChatCompletionsResponse) ToMap() (map[string]any, error) {
	choices, err := openAIChatCompletionsChoiceListToMaps(r.Choices)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"id":      r.ID,
		"object":  r.Object,
		"created": r.Created,
		"choices": choices,
	}
	setMapString(out, "model", r.Model)
	if r.Usage != nil {
		usage, err := r.Usage.ToMap()
		if err != nil {
			return nil, err
		}
		out["usage"] = usage
	}
	setMapString(out, "system_fingerprint", r.SystemFingerprint)
	setMapString(out, "service_tier", r.ServiceTier)
	return out, nil
}

type OpenAIChatToolCallDelta struct {
	Index    int                 `json:"index,omitempty"`
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function *OpenAIFunctionCall `json:"function,omitempty"`
}

func (d *OpenAIChatToolCallDelta) FromMap(m map[string]any) error {
	var err error
	d.Index, err = intValue(m, "index")
	if err != nil {
		return err
	}
	d.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	d.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	d.Function, err = decodeOpenAIFunctionCallPtrFromMapField(m, "function")
	return err
}

func (d *OpenAIChatToolCallDelta) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "index", d.Index)
	setMapString(out, "id", d.ID)
	setMapString(out, "type", d.Type)
	if d.Function != nil {
		function, err := d.Function.ToMap()
		if err != nil {
			return nil, err
		}
		out["function"] = function
	}
	return out, nil
}

type OpenAIChatCompletionsDelta struct {
	Role      string                    `json:"role,omitempty"`
	Content   string                    `json:"content,omitempty"`
	Refusal   string                    `json:"refusal,omitempty"`
	ToolCalls []OpenAIChatToolCallDelta `json:"tool_calls,omitempty"`
}

func (d *OpenAIChatCompletionsDelta) FromMap(m map[string]any) error {
	var err error
	d.Role, err = stringValue(m, "role")
	if err != nil {
		return err
	}
	d.Content, err = stringValue(m, "content")
	if err != nil {
		return err
	}
	d.Refusal, err = stringValue(m, "refusal")
	if err != nil {
		return err
	}
	d.ToolCalls, err = decodeOpenAIChatToolCallDeltaListFromMapField(m, "tool_calls")
	return err
}

func (d *OpenAIChatCompletionsDelta) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "role", d.Role)
	setMapString(out, "content", d.Content)
	setMapString(out, "refusal", d.Refusal)
	if len(d.ToolCalls) > 0 {
		toolCalls, err := openAIChatToolCallDeltaListToMaps(d.ToolCalls)
		if err != nil {
			return nil, err
		}
		out["tool_calls"] = toolCalls
	}
	return out, nil
}

type OpenAIChatCompletionsChunkChoice struct {
	Index        int                        `json:"index,omitempty"`
	Delta        OpenAIChatCompletionsDelta `json:"delta"`
	FinishReason string                     `json:"finish_reason,omitempty"`
}

func (c *OpenAIChatCompletionsChunkChoice) FromMap(m map[string]any) error {
	var err error
	c.Index, err = intValue(m, "index")
	if err != nil {
		return err
	}
	c.Delta, err = decodeOpenAIChatCompletionsDeltaFromMapField(m, "delta")
	if err != nil {
		return err
	}
	c.FinishReason, err = stringValue(m, "finish_reason")
	return err
}

func (c *OpenAIChatCompletionsChunkChoice) ToMap() (map[string]any, error) {
	delta, err := c.Delta.ToMap()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"index": c.Index, "delta": delta}
	setMapString(out, "finish_reason", c.FinishReason)
	return out, nil
}

type OpenAIChatCompletionsStreamResponse struct {
	ID                string                             `json:"id,omitempty"`
	Object            string                             `json:"object,omitempty"`
	Created           int64                              `json:"created,omitempty"`
	Model             string                             `json:"model,omitempty"`
	Choices           []OpenAIChatCompletionsChunkChoice `json:"choices,omitempty"`
	Usage             *OpenAIChatCompletionsUsage        `json:"usage,omitempty"`
	SystemFingerprint string                             `json:"system_fingerprint,omitempty"`
	ServiceTier       string                             `json:"service_tier,omitempty"`
}

func (r *OpenAIChatCompletionsStreamResponse) FromMap(m map[string]any) error {
	var err error
	r.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	r.Object, err = stringValue(m, "object")
	if err != nil {
		return err
	}
	created, err := toInt64(m["created"])
	if err == nil {
		r.Created = created
	} else if _, ok := m["created"]; ok && m["created"] != nil {
		return err
	}
	r.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	r.Choices, err = decodeOpenAIChatCompletionsChunkChoiceListFromMapField(m, "choices")
	if err != nil {
		return err
	}
	r.Usage, err = decodeOpenAIChatCompletionsUsagePtrFromMapField(m, "usage")
	if err != nil {
		return err
	}
	r.SystemFingerprint, err = stringValue(m, "system_fingerprint")
	if err != nil {
		return err
	}
	r.ServiceTier, err = stringValue(m, "service_tier")
	return err
}

func (r *OpenAIChatCompletionsStreamResponse) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "id", r.ID)
	setMapString(out, "object", r.Object)
	if r.Created != 0 {
		out["created"] = r.Created
	}
	setMapString(out, "model", r.Model)
	if len(r.Choices) > 0 {
		choices, err := openAIChatCompletionsChunkChoiceListToMaps(r.Choices)
		if err != nil {
			return nil, err
		}
		out["choices"] = choices
	}
	if r.Usage != nil {
		usage, err := r.Usage.ToMap()
		if err != nil {
			return nil, err
		}
		out["usage"] = usage
	}
	setMapString(out, "system_fingerprint", r.SystemFingerprint)
	setMapString(out, "service_tier", r.ServiceTier)
	return out, nil
}

type OpenAIResponsesRequest struct {
	Background           *bool                          `json:"background,omitempty"`
	Conversation         *OpenAIResponseConversationRef `json:"conversation,omitempty"`
	Include              []string                       `json:"include,omitempty"`
	Input                *OpenAIResponseInput           `json:"input,omitempty"`
	Instructions         string                         `json:"instructions,omitempty"`
	MaxOutputTokens      int                            `json:"max_output_tokens,omitempty"`
	MaxToolCalls         int                            `json:"max_tool_calls,omitempty"`
	Metadata             map[string]string              `json:"metadata,omitempty"`
	Model                string                         `json:"model"`
	ParallelToolCalls    *bool                          `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID   string                         `json:"previous_response_id,omitempty"`
	PromptCacheKey       string                         `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string                         `json:"prompt_cache_retention,omitempty"`
	Reasoning            *OpenAIResponseReasoningConfig `json:"reasoning,omitempty"`
	SafetyIdentifier     string                         `json:"safety_identifier,omitempty"`
	ServiceTier          string                         `json:"service_tier,omitempty"`
	Store                *bool                          `json:"store,omitempty"`
	Stream               bool                           `json:"stream,omitempty"`
	StreamOptions        *OpenAIResponsesStreamOptions  `json:"stream_options,omitempty"`
	Temperature          *float64                       `json:"temperature,omitempty"`
	Text                 *OpenAIResponsesTextConfig     `json:"text,omitempty"`
	ToolChoice           *OpenAIResponsesToolChoice     `json:"tool_choice,omitempty"`
	Tools                []OpenAIResponseTool           `json:"tools,omitempty"`
	TopLogprobs          int                            `json:"top_logprobs,omitempty"`
	TopP                 *float64                       `json:"top_p,omitempty"`
	Truncation           string                         `json:"truncation,omitempty"`
	User                 string                         `json:"user,omitempty"`
}

func (r *OpenAIResponsesRequest) GetPrompt() string {
	if r.Input == nil {
		return ""
	}
	if r.Input.Text != nil {
		return *r.Input.Text
	}
	var builder strings.Builder
	for i := range r.Input.Items {
		role := strings.ToLower(r.Input.Items[i].Role)
		if role != "" && role != "user" {
			continue
		}
		if r.Input.Items[i].Content == nil {
			continue
		}
		appendOpenAIResponseInputPrompt(&builder, r.Input.Items[i].Content)
	}
	return builder.String()
}

func (r *OpenAIResponsesRequest) FromMap(m map[string]any) error {
	var err error
	r.Background, err = boolPtrValue(m, "background")
	if err != nil {
		return err
	}
	r.Conversation, err = decodeOpenAIResponseConversationRefPtrFromMapField(m, "conversation")
	if err != nil {
		return err
	}
	r.Include, err = stringSliceValue(m, "include")
	if err != nil {
		return err
	}
	r.Input, err = decodeOpenAIResponseInputPtrFromMapField(m, "input")
	if err != nil {
		return err
	}
	r.Instructions, err = stringValue(m, "instructions")
	if err != nil {
		return err
	}
	r.MaxOutputTokens, err = intValue(m, "max_output_tokens")
	if err != nil {
		return err
	}
	r.MaxToolCalls, err = intValue(m, "max_tool_calls")
	if err != nil {
		return err
	}
	r.Metadata, err = mapStringStringValue(m, "metadata")
	if err != nil {
		return err
	}
	r.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	r.ParallelToolCalls, err = boolPtrValue(m, "parallel_tool_calls")
	if err != nil {
		return err
	}
	r.PreviousResponseID, err = stringValue(m, "previous_response_id")
	if err != nil {
		return err
	}
	r.PromptCacheKey, err = stringValue(m, "prompt_cache_key")
	if err != nil {
		return err
	}
	r.PromptCacheRetention, err = stringValue(m, "prompt_cache_retention")
	if err != nil {
		return err
	}
	r.Reasoning, err = decodeOpenAIResponseReasoningConfigPtrFromMapField(m, "reasoning")
	if err != nil {
		return err
	}
	r.SafetyIdentifier, err = stringValue(m, "safety_identifier")
	if err != nil {
		return err
	}
	r.ServiceTier, err = stringValue(m, "service_tier")
	if err != nil {
		return err
	}
	r.Store, err = boolPtrValue(m, "store")
	if err != nil {
		return err
	}
	r.Stream, err = boolValue(m, "stream")
	if err != nil {
		return err
	}
	r.StreamOptions, err = decodeOpenAIResponsesStreamOptionsPtrFromMapField(m, "stream_options")
	if err != nil {
		return err
	}
	r.Temperature, err = floatPtrValue(m, "temperature")
	if err != nil {
		return err
	}
	r.TopP, err = floatPtrValue(m, "top_p")
	if err != nil {
		return err
	}
	r.Tools, err = decodeOpenAIResponseToolListFromMapField(m, "tools")
	if err != nil {
		return err
	}
	r.ToolChoice, err = decodeOpenAIResponsesToolChoicePtrFromMapField(m, "tool_choice")
	if err != nil {
		return err
	}
	r.Text, err = decodeOpenAIResponsesTextConfigPtrFromMapField(m, "text")
	if err != nil {
		return err
	}
	r.TopLogprobs, err = intValue(m, "top_logprobs")
	if err != nil {
		return err
	}
	r.Truncation, err = stringValue(m, "truncation")
	if err != nil {
		return err
	}
	r.User, err = stringValue(m, "user")
	return err
}

func (r *OpenAIResponsesRequest) ToMap() (map[string]any, error) {
	out := map[string]any{"model": r.Model}
	if r.Background != nil {
		out["background"] = *r.Background
	}
	if r.Conversation != nil {
		conversation, err := r.Conversation.ToAny()
		if err != nil {
			return nil, err
		}
		out["conversation"] = conversation
	}
	setMapStringSlice(out, "include", r.Include)
	if r.Input != nil {
		input, err := r.Input.ToAny()
		if err != nil {
			return nil, err
		}
		out["input"] = input
	}
	setMapString(out, "instructions", r.Instructions)
	setMapInt(out, "max_output_tokens", r.MaxOutputTokens)
	setMapInt(out, "max_tool_calls", r.MaxToolCalls)
	if r.Metadata != nil {
		metadata := make(map[string]any, len(r.Metadata))
		for k, v := range r.Metadata {
			metadata[k] = v
		}
		out["metadata"] = metadata
	}
	if r.ParallelToolCalls != nil {
		out["parallel_tool_calls"] = *r.ParallelToolCalls
	}
	setMapString(out, "previous_response_id", r.PreviousResponseID)
	setMapString(out, "prompt_cache_key", r.PromptCacheKey)
	setMapString(out, "prompt_cache_retention", r.PromptCacheRetention)
	if r.Reasoning != nil {
		reasoning, err := r.Reasoning.ToMap()
		if err != nil {
			return nil, err
		}
		out["reasoning"] = reasoning
	}
	setMapString(out, "safety_identifier", r.SafetyIdentifier)
	setMapString(out, "service_tier", r.ServiceTier)
	if r.Store != nil {
		out["store"] = *r.Store
	}
	setMapBool(out, "stream", r.Stream)
	if r.StreamOptions != nil {
		streamOptions, err := r.StreamOptions.ToMap()
		if err != nil {
			return nil, err
		}
		out["stream_options"] = streamOptions
	}
	if r.Temperature != nil {
		out["temperature"] = *r.Temperature
	}
	if r.TopP != nil {
		out["top_p"] = *r.TopP
	}
	if len(r.Tools) > 0 {
		tools, err := openAIResponseToolListToMaps(r.Tools)
		if err != nil {
			return nil, err
		}
		out["tools"] = tools
	}
	if r.ToolChoice != nil {
		toolChoice, err := r.ToolChoice.ToAny()
		if err != nil {
			return nil, err
		}
		out["tool_choice"] = toolChoice
	}
	if r.Text != nil {
		text, err := r.Text.ToMap()
		if err != nil {
			return nil, err
		}
		out["text"] = text
	}
	setMapInt(out, "top_logprobs", r.TopLogprobs)
	setMapString(out, "truncation", r.Truncation)
	setMapString(out, "user", r.User)
	return out, nil
}

type OpenAIResponseTool struct {
	Type               string                                  `json:"type"`
	Name               string                                  `json:"name,omitempty"`
	Description        string                                  `json:"description,omitempty"`
	Parameters         map[string]any                          `json:"parameters,omitempty"`
	Strict             *bool                                   `json:"strict,omitempty"`
	DeferLoading       *bool                                   `json:"defer_loading,omitempty"`
	VectorStoreIDs     []string                                `json:"vector_store_ids,omitempty"`
	Filters            map[string]any                          `json:"filters,omitempty"`
	MaxNumResults      int                                     `json:"max_num_results,omitempty"`
	RankingOptions     *OpenAIResponseFileSearchRankingOptions `json:"ranking_options,omitempty"`
	SearchContextSize  string                                  `json:"search_context_size,omitempty"`
	SearchContentTypes []string                                `json:"search_content_types,omitempty"`
	UserLocation       *OpenAIChatApproximateLocation          `json:"user_location,omitempty"`
}

func (t *OpenAIResponseTool) FromMap(m map[string]any) error {
	var err error
	t.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	t.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	t.Description, err = stringValue(m, "description")
	if err != nil {
		return err
	}
	t.Parameters, err = mapStringAnyValue(m, "parameters")
	if err != nil {
		return err
	}
	t.Strict, err = boolPtrValue(m, "strict")
	if err != nil {
		return err
	}
	t.DeferLoading, err = boolPtrValue(m, "defer_loading")
	if err != nil {
		return err
	}
	t.VectorStoreIDs, err = stringSliceValue(m, "vector_store_ids")
	if err != nil {
		return err
	}
	t.Filters, err = mapStringAnyValue(m, "filters")
	if err != nil {
		return err
	}
	t.MaxNumResults, err = intValue(m, "max_num_results")
	if err != nil {
		return err
	}
	t.RankingOptions, err = decodeOpenAIResponseFileSearchRankingOptionsPtrFromMapField(m, "ranking_options")
	if err != nil {
		return err
	}
	t.SearchContextSize, err = stringValue(m, "search_context_size")
	if err != nil {
		return err
	}
	t.SearchContentTypes, err = stringSliceValue(m, "search_content_types")
	if err != nil {
		return err
	}
	t.UserLocation, err = decodeOpenAIChatApproximateLocationPtrFromMapField(m, "user_location")
	return err
}

func (t *OpenAIResponseTool) ToMap() (map[string]any, error) {
	out := map[string]any{"type": t.Type}
	setMapString(out, "name", t.Name)
	setMapString(out, "description", t.Description)
	if t.Parameters != nil {
		out["parameters"] = t.Parameters
	}
	if t.Strict != nil {
		out["strict"] = *t.Strict
	}
	if t.DeferLoading != nil {
		out["defer_loading"] = *t.DeferLoading
	}
	setMapStringSlice(out, "vector_store_ids", t.VectorStoreIDs)
	if t.Filters != nil {
		out["filters"] = t.Filters
	}
	setMapInt(out, "max_num_results", t.MaxNumResults)
	if t.RankingOptions != nil {
		rankingOptions, err := t.RankingOptions.ToMap()
		if err != nil {
			return nil, err
		}
		out["ranking_options"] = rankingOptions
	}
	setMapString(out, "search_context_size", t.SearchContextSize)
	setMapStringSlice(out, "search_content_types", t.SearchContentTypes)
	if t.UserLocation != nil {
		userLocation, err := t.UserLocation.ToMap()
		if err != nil {
			return nil, err
		}
		out["user_location"] = userLocation
	}
	return out, nil
}

type OpenAIResponseContentPart struct {
	Type        string                       `json:"type,omitempty"`
	Text        string                       `json:"text,omitempty"`
	Refusal     string                       `json:"refusal,omitempty"`
	Annotations []OpenAIResponseAnnotation   `json:"annotations,omitempty"`
	Logprobs    []OpenAIResponseTokenLogprob `json:"logprobs,omitempty"`
}

func (p *OpenAIResponseContentPart) FromMap(m map[string]any) error {
	var err error
	p.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	p.Text, err = stringValue(m, "text")
	if err != nil {
		return err
	}
	p.Refusal, err = stringValue(m, "refusal")
	if err != nil {
		return err
	}
	p.Annotations, err = decodeOpenAIResponseAnnotationListFromMapField(m, "annotations")
	if err != nil {
		return err
	}
	p.Logprobs, err = decodeOpenAIResponseTokenLogprobListFromMapField(m, "logprobs")
	return err
}

func (p *OpenAIResponseContentPart) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", p.Type)
	setMapString(out, "text", p.Text)
	setMapString(out, "refusal", p.Refusal)
	if len(p.Annotations) > 0 {
		annotations, err := openAIResponseAnnotationListToMaps(p.Annotations)
		if err != nil {
			return nil, err
		}
		out["annotations"] = annotations
	}
	if len(p.Logprobs) > 0 {
		logprobs, err := openAIResponseTokenLogprobListToMaps(p.Logprobs)
		if err != nil {
			return nil, err
		}
		out["logprobs"] = logprobs
	}
	return out, nil
}

type OpenAIResponseOutputItem struct {
	ID               string                           `json:"id,omitempty"`
	Type             string                           `json:"type,omitempty"`
	Role             string                           `json:"role,omitempty"`
	Status           string                           `json:"status,omitempty"`
	Content          []OpenAIResponseContentPart      `json:"content,omitempty"`
	Name             string                           `json:"name,omitempty"`
	Arguments        string                           `json:"arguments,omitempty"`
	CallID           string                           `json:"call_id,omitempty"`
	Summary          []OpenAIResponseSummaryText      `json:"summary,omitempty"`
	Queries          []string                         `json:"queries,omitempty"`
	Results          []OpenAIResponseFileSearchResult `json:"results,omitempty"`
	Phase            string                           `json:"phase,omitempty"`
	EncryptedContent string                           `json:"encrypted_content,omitempty"`
}

func (i *OpenAIResponseOutputItem) FromMap(m map[string]any) error {
	var err error
	i.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	i.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	i.Role, err = stringValue(m, "role")
	if err != nil {
		return err
	}
	i.Status, err = stringValue(m, "status")
	if err != nil {
		return err
	}
	i.Content, err = decodeOpenAIResponseContentPartListFromMapField(m, "content")
	if err != nil {
		return err
	}
	i.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	i.Arguments, err = stringValue(m, "arguments")
	if err != nil {
		return err
	}
	i.CallID, err = stringValue(m, "call_id")
	if err != nil {
		return err
	}
	i.Summary, err = decodeOpenAIResponseSummaryTextListFromMapField(m, "summary")
	if err != nil {
		return err
	}
	i.Queries, err = stringSliceValue(m, "queries")
	if err != nil {
		return err
	}
	i.Results, err = decodeOpenAIResponseFileSearchResultListFromMapField(m, "results")
	if err != nil {
		return err
	}
	i.Phase, err = stringValue(m, "phase")
	if err != nil {
		return err
	}
	i.EncryptedContent, err = stringValue(m, "encrypted_content")
	return err
}

func (i *OpenAIResponseOutputItem) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "id", i.ID)
	setMapString(out, "type", i.Type)
	setMapString(out, "role", i.Role)
	setMapString(out, "status", i.Status)
	if len(i.Content) > 0 {
		content, err := openAIResponseContentPartListToMaps(i.Content)
		if err != nil {
			return nil, err
		}
		out["content"] = content
	}
	setMapString(out, "name", i.Name)
	setMapString(out, "arguments", i.Arguments)
	setMapString(out, "call_id", i.CallID)
	setMapString(out, "phase", i.Phase)
	setMapString(out, "encrypted_content", i.EncryptedContent)
	if len(i.Summary) > 0 {
		summary, err := openAIResponseSummaryTextListToMaps(i.Summary)
		if err != nil {
			return nil, err
		}
		out["summary"] = summary
	}
	setMapStringSlice(out, "queries", i.Queries)
	if len(i.Results) > 0 {
		results, err := openAIResponseFileSearchResultListToMaps(i.Results)
		if err != nil {
			return nil, err
		}
		out["results"] = results
	}
	return out, nil
}

type OpenAIResponsesUsage struct {
	InputTokens        int                 `json:"input_tokens,omitempty"`
	OutputTokens       int                 `json:"output_tokens,omitempty"`
	TotalTokens        int                 `json:"total_tokens,omitempty"`
	InputTokenDetails  *OpenAITokenDetails `json:"input_token_details,omitempty"`
	OutputTokenDetails *OpenAITokenDetails `json:"output_token_details,omitempty"`
}

type OpenAIResponseFileSearchRankingOptions struct {
	Ranker         string   `json:"ranker,omitempty"`
	ScoreThreshold *float64 `json:"score_threshold,omitempty"`
}

func (o *OpenAIResponseFileSearchRankingOptions) FromMap(m map[string]any) error {
	var err error
	o.Ranker, err = stringValue(m, "ranker")
	if err != nil {
		return err
	}
	o.ScoreThreshold, err = floatPtrValue(m, "score_threshold")
	return err
}

func (o *OpenAIResponseFileSearchRankingOptions) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "ranker", o.Ranker)
	if o.ScoreThreshold != nil {
		out["score_threshold"] = *o.ScoreThreshold
	}
	return out, nil
}

type OpenAIResponseConversationRef struct {
	ID string `json:"id,omitempty"`
}

func (r *OpenAIResponseConversationRef) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		r.ID = typed
	case map[string]any:
		id, err := stringValue(typed, "id")
		if err != nil {
			return err
		}
		r.ID = id
	}
	return nil
}

func (r *OpenAIResponseConversationRef) ToAny() (any, error) {
	if r == nil || r.ID == "" {
		return nil, nil
	}
	return map[string]any{"id": r.ID}, nil
}

type OpenAIResponseInput struct {
	Text  *string                   `json:"-"`
	Items []OpenAIResponseInputItem `json:"-"`
}

func (i *OpenAIResponseInput) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		i.Text = &typed
		i.Items = nil
	case []any:
		items, err := decodeOpenAIResponseInputItemsFromAnySlice(typed)
		if err != nil {
			return err
		}
		i.Text = nil
		i.Items = items
	case []map[string]any:
		items := make([]OpenAIResponseInputItem, 0, len(typed))
		for _, item := range typed {
			var decoded OpenAIResponseInputItem
			if err := decoded.FromMap(item); err != nil {
				return err
			}
			items = append(items, decoded)
		}
		i.Text = nil
		i.Items = items
	}
	return nil
}

func (i *OpenAIResponseInput) ToAny() (any, error) {
	if i == nil {
		return nil, nil
	}
	if i.Text != nil {
		return *i.Text, nil
	}
	items := make([]any, 0, len(i.Items))
	for idx := range i.Items {
		item, err := i.Items[idx].ToMap()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

type OpenAIResponseInputItem struct {
	ID        string                      `json:"id,omitempty"`
	Type      string                      `json:"type,omitempty"`
	Role      string                      `json:"role,omitempty"`
	Status    string                      `json:"status,omitempty"`
	Phase     string                      `json:"phase,omitempty"`
	Content   *OpenAIResponseInputContent `json:"content,omitempty"`
	Name      string                      `json:"name,omitempty"`
	CallID    string                      `json:"call_id,omitempty"`
	Arguments string                      `json:"arguments,omitempty"`
	Output    *OpenAIResponseInputContent `json:"output,omitempty"`
}

func (i *OpenAIResponseInputItem) FromMap(m map[string]any) error {
	var err error
	i.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	i.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	i.Role, err = stringValue(m, "role")
	if err != nil {
		return err
	}
	i.Status, err = stringValue(m, "status")
	if err != nil {
		return err
	}
	i.Phase, err = stringValue(m, "phase")
	if err != nil {
		return err
	}
	i.Content, err = decodeOpenAIResponseInputContentPtrFromMapField(m, "content")
	if err != nil {
		return err
	}
	i.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	i.CallID, err = stringValue(m, "call_id")
	if err != nil {
		return err
	}
	i.Arguments, err = stringValue(m, "arguments")
	if err != nil {
		return err
	}
	i.Output, err = decodeOpenAIResponseInputContentPtrFromMapField(m, "output")
	return err
}

func (i *OpenAIResponseInputItem) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "id", i.ID)
	setMapString(out, "type", i.Type)
	setMapString(out, "role", i.Role)
	setMapString(out, "status", i.Status)
	setMapString(out, "phase", i.Phase)
	if i.Content != nil {
		content, err := i.Content.ToAny()
		if err != nil {
			return nil, err
		}
		out["content"] = content
	}
	setMapString(out, "name", i.Name)
	setMapString(out, "call_id", i.CallID)
	setMapString(out, "arguments", i.Arguments)
	if i.Output != nil {
		output, err := i.Output.ToAny()
		if err != nil {
			return nil, err
		}
		out["output"] = output
	}
	return out, nil
}

type OpenAIResponseInputContent struct {
	Text  *string                   `json:"-"`
	Parts []OpenAIResponseInputPart `json:"-"`
}

func (c *OpenAIResponseInputContent) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		c.Text = &typed
		c.Parts = nil
	case []any:
		parts, err := decodeOpenAIResponseInputPartsFromAnySlice(typed)
		if err != nil {
			return err
		}
		c.Text = nil
		c.Parts = parts
	case []map[string]any:
		parts := make([]OpenAIResponseInputPart, 0, len(typed))
		for _, item := range typed {
			var part OpenAIResponseInputPart
			if err := part.FromMap(item); err != nil {
				return err
			}
			parts = append(parts, part)
		}
		c.Text = nil
		c.Parts = parts
	}
	return nil
}

func (c *OpenAIResponseInputContent) ToAny() (any, error) {
	if c == nil {
		return nil, nil
	}
	if c.Text != nil {
		return *c.Text, nil
	}
	items := make([]any, 0, len(c.Parts))
	for idx := range c.Parts {
		part, err := c.Parts[idx].ToMap()
		if err != nil {
			return nil, err
		}
		items = append(items, part)
	}
	return items, nil
}

type OpenAIResponseInputPart struct {
	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	Detail   string `json:"detail,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileData string `json:"file_data,omitempty"`
	FileURL  string `json:"file_url,omitempty"`
	Filename string `json:"filename,omitempty"`
}

func appendOpenAIChatMessagePrompt(builder *strings.Builder, content *OpenAIChatMessageContent) {
	if content == nil {
		return
	}
	if content.Text != nil {
		_, _ = builder.WriteString(*content.Text)
		return
	}
	for _, part := range content.Parts {
		if part.Text == "" {
			continue
		}
		if builder.Len() > 0 {
			_, _ = builder.WriteString("\n")
		}
		_, _ = builder.WriteString(part.Text)
	}
}

func appendOpenAIResponseInputPrompt(builder *strings.Builder, content *OpenAIResponseInputContent) {
	if content == nil {
		return
	}
	if content.Text != nil {
		_, _ = builder.WriteString(*content.Text)
		return
	}
	for _, part := range content.Parts {
		if part.Text == "" {
			continue
		}
		if builder.Len() > 0 {
			_, _ = builder.WriteString("\n")
		}
		_, _ = builder.WriteString(part.Text)
	}
}

func decodeOpenAIResponseInputItemsFromAnySlice(items []any) ([]OpenAIResponseInputItem, error) {
	decodedItems := make([]OpenAIResponseInputItem, 0, len(items))
	for _, item := range items {
		mv, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var decoded OpenAIResponseInputItem
		if err := decoded.FromMap(mv); err != nil {
			return nil, err
		}
		decodedItems = append(decodedItems, decoded)
	}
	return decodedItems, nil
}

func decodeOpenAIResponseInputPartsFromAnySlice(items []any) ([]OpenAIResponseInputPart, error) {
	decodedParts := make([]OpenAIResponseInputPart, 0, len(items))
	for _, item := range items {
		mv, ok := item.(map[string]any)
		if !ok {
			continue
		}
		var part OpenAIResponseInputPart
		if err := part.FromMap(mv); err != nil {
			return nil, err
		}
		decodedParts = append(decodedParts, part)
	}
	return decodedParts, nil
}

func (p *OpenAIResponseInputPart) FromMap(m map[string]any) error {
	var err error
	p.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	p.Text, err = stringValue(m, "text")
	if err != nil {
		return err
	}
	p.Detail, err = stringValue(m, "detail")
	if err != nil {
		return err
	}
	p.FileID, err = stringValue(m, "file_id")
	if err != nil {
		return err
	}
	p.ImageURL, err = stringValue(m, "image_url")
	if err != nil {
		return err
	}
	p.FileData, err = stringValue(m, "file_data")
	if err != nil {
		return err
	}
	p.FileURL, err = stringValue(m, "file_url")
	if err != nil {
		return err
	}
	p.Filename, err = stringValue(m, "filename")
	return err
}

func (p *OpenAIResponseInputPart) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", p.Type)
	setMapString(out, "text", p.Text)
	setMapString(out, "detail", p.Detail)
	setMapString(out, "file_id", p.FileID)
	setMapString(out, "image_url", p.ImageURL)
	setMapString(out, "file_data", p.FileData)
	setMapString(out, "file_url", p.FileURL)
	setMapString(out, "filename", p.Filename)
	return out, nil
}

type OpenAIResponseReasoningConfig struct {
	Effort          string `json:"effort,omitempty"`
	GenerateSummary string `json:"generate_summary,omitempty"`
	Summary         string `json:"summary,omitempty"`
}

func (c *OpenAIResponseReasoningConfig) FromMap(m map[string]any) error {
	var err error
	c.Effort, err = stringValue(m, "effort")
	if err != nil {
		return err
	}
	c.GenerateSummary, err = stringValue(m, "generate_summary")
	if err != nil {
		return err
	}
	c.Summary, err = stringValue(m, "summary")
	return err
}

func (c *OpenAIResponseReasoningConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "effort", c.Effort)
	setMapString(out, "generate_summary", c.GenerateSummary)
	setMapString(out, "summary", c.Summary)
	return out, nil
}

type OpenAIResponsesStreamOptions struct {
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

func (o *OpenAIResponsesStreamOptions) FromMap(m map[string]any) error {
	var err error
	o.IncludeObfuscation, err = boolPtrValue(m, "include_obfuscation")
	return err
}

func (o *OpenAIResponsesStreamOptions) ToMap() (map[string]any, error) {
	out := map[string]any{}
	if o.IncludeObfuscation != nil {
		out["include_obfuscation"] = *o.IncludeObfuscation
	}
	return out, nil
}

type OpenAIResponsesTextConfig struct {
	Format    *OpenAIResponsesTextFormat `json:"format,omitempty"`
	Verbosity string                     `json:"verbosity,omitempty"`
}

func (c *OpenAIResponsesTextConfig) FromMap(m map[string]any) error {
	var err error
	c.Format, err = decodeOpenAIResponsesTextFormatPtrFromMapField(m, "format")
	if err != nil {
		return err
	}
	c.Verbosity, err = stringValue(m, "verbosity")
	return err
}

func (c *OpenAIResponsesTextConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	if c.Format != nil {
		format, err := c.Format.ToMap()
		if err != nil {
			return nil, err
		}
		out["format"] = format
	}
	setMapString(out, "verbosity", c.Verbosity)
	return out, nil
}

type OpenAIResponsesTextFormat struct {
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Description string         `json:"description,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
}

func (f *OpenAIResponsesTextFormat) FromMap(m map[string]any) error {
	var err error
	f.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	f.Name, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	f.Schema, err = mapStringAnyValue(m, "schema")
	if err != nil {
		return err
	}
	f.Description, err = stringValue(m, "description")
	if err != nil {
		return err
	}
	f.Strict, err = boolPtrValue(m, "strict")
	return err
}

func (f *OpenAIResponsesTextFormat) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", f.Type)
	setMapString(out, "name", f.Name)
	if f.Schema != nil {
		out["schema"] = f.Schema
	}
	setMapString(out, "description", f.Description)
	if f.Strict != nil {
		out["strict"] = *f.Strict
	}
	return out, nil
}

type OpenAIResponsesToolChoice struct {
	Mode         string               `json:"-"`
	Type         string               `json:"-"`
	Name         string               `json:"-"`
	ServerLabel  string               `json:"-"`
	AllowedMode  string               `json:"-"`
	AllowedTools []OpenAIResponseTool `json:"-"`
}

func (c *OpenAIResponsesToolChoice) FromAny(v any) error {
	switch typed := v.(type) {
	case nil:
		return nil
	case string:
		c.Mode = typed
	case map[string]any:
		var err error
		c.Type, err = stringValue(typed, "type")
		if err != nil {
			return err
		}
		c.Name, err = stringValue(typed, "name")
		if err != nil {
			return err
		}
		c.ServerLabel, err = stringValue(typed, "server_label")
		if err != nil {
			return err
		}
		c.AllowedMode, err = stringValue(typed, "mode")
		if err != nil {
			return err
		}
		c.AllowedTools, err = decodeOpenAIResponseToolListFromMapField(typed, "tools")
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *OpenAIResponsesToolChoice) ToAny() (any, error) {
	if c == nil {
		return nil, nil
	}
	if c.Mode != "" {
		return c.Mode, nil
	}
	out := map[string]any{}
	setMapString(out, "type", c.Type)
	setMapString(out, "name", c.Name)
	setMapString(out, "server_label", c.ServerLabel)
	setMapString(out, "mode", c.AllowedMode)
	if len(c.AllowedTools) > 0 {
		tools, err := openAIResponseToolListToMaps(c.AllowedTools)
		if err != nil {
			return nil, err
		}
		out["tools"] = tools
	}
	return out, nil
}

type OpenAIResponseAnnotation struct {
	Type        string `json:"type,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Index       int    `json:"index,omitempty"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url,omitempty"`
	StartIndex  int    `json:"start_index,omitempty"`
	EndIndex    int    `json:"end_index,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

func (a *OpenAIResponseAnnotation) FromMap(m map[string]any) error {
	var err error
	a.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	a.FileID, err = stringValue(m, "file_id")
	if err != nil {
		return err
	}
	a.Filename, err = stringValue(m, "filename")
	if err != nil {
		return err
	}
	a.Index, err = intValue(m, "index")
	if err != nil {
		return err
	}
	a.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	a.URL, err = stringValue(m, "url")
	if err != nil {
		return err
	}
	a.StartIndex, err = intValue(m, "start_index")
	if err != nil {
		return err
	}
	a.EndIndex, err = intValue(m, "end_index")
	if err != nil {
		return err
	}
	a.ContainerID, err = stringValue(m, "container_id")
	return err
}

func (a *OpenAIResponseAnnotation) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", a.Type)
	setMapString(out, "file_id", a.FileID)
	setMapString(out, "filename", a.Filename)
	setMapInt(out, "index", a.Index)
	setMapString(out, "title", a.Title)
	setMapString(out, "url", a.URL)
	setMapInt(out, "start_index", a.StartIndex)
	setMapInt(out, "end_index", a.EndIndex)
	setMapString(out, "container_id", a.ContainerID)
	return out, nil
}

type OpenAIResponseTokenLogprob struct {
	Token       string                       `json:"token,omitempty"`
	Bytes       []int                        `json:"bytes,omitempty"`
	Logprob     *float64                     `json:"logprob,omitempty"`
	TopLogprobs []OpenAIResponseTokenLogprob `json:"top_logprobs,omitempty"`
}

func (l *OpenAIResponseTokenLogprob) FromMap(m map[string]any) error {
	var err error
	l.Token, err = stringValue(m, "token")
	if err != nil {
		return err
	}
	l.Bytes, err = intSliceValue(m, "bytes")
	if err != nil {
		return err
	}
	l.Logprob, err = floatPtrValue(m, "logprob")
	if err != nil {
		return err
	}
	l.TopLogprobs, err = decodeOpenAIResponseTokenLogprobListFromMapField(m, "top_logprobs")
	return err
}

func (l *OpenAIResponseTokenLogprob) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "token", l.Token)
	if len(l.Bytes) > 0 {
		bytes := make([]any, 0, len(l.Bytes))
		for _, b := range l.Bytes {
			bytes = append(bytes, b)
		}
		out["bytes"] = bytes
	}
	if l.Logprob != nil {
		out["logprob"] = *l.Logprob
	}
	if len(l.TopLogprobs) > 0 {
		topLogprobs, err := openAIResponseTokenLogprobListToMaps(l.TopLogprobs)
		if err != nil {
			return nil, err
		}
		out["top_logprobs"] = topLogprobs
	}
	return out, nil
}

type OpenAIResponseSummaryText struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

func (s *OpenAIResponseSummaryText) FromMap(m map[string]any) error {
	var err error
	s.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	s.Text, err = stringValue(m, "text")
	return err
}

func (s *OpenAIResponseSummaryText) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", s.Type)
	setMapString(out, "text", s.Text)
	return out, nil
}

type OpenAIResponseFileSearchResult struct {
	FileID     string         `json:"file_id,omitempty"`
	Filename   string         `json:"filename,omitempty"`
	Score      *float64       `json:"score,omitempty"`
	Text       string         `json:"text,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

func (r *OpenAIResponseFileSearchResult) FromMap(m map[string]any) error {
	var err error
	r.FileID, err = stringValue(m, "file_id")
	if err != nil {
		return err
	}
	r.Filename, err = stringValue(m, "filename")
	if err != nil {
		return err
	}
	r.Score, err = floatPtrValue(m, "score")
	if err != nil {
		return err
	}
	r.Text, err = stringValue(m, "text")
	if err != nil {
		return err
	}
	r.Attributes, err = mapStringAnyValue(m, "attributes")
	return err
}

func (r *OpenAIResponseFileSearchResult) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "file_id", r.FileID)
	setMapString(out, "filename", r.Filename)
	if r.Score != nil {
		out["score"] = *r.Score
	}
	setMapString(out, "text", r.Text)
	if r.Attributes != nil {
		out["attributes"] = r.Attributes
	}
	return out, nil
}

func (u *OpenAIResponsesUsage) FromMap(m map[string]any) error {
	var err error
	u.InputTokens, err = intValue(m, "input_tokens")
	if err != nil {
		return err
	}
	u.OutputTokens, err = intValue(m, "output_tokens")
	if err != nil {
		return err
	}
	u.TotalTokens, err = intValue(m, "total_tokens")
	if err != nil {
		return err
	}
	u.InputTokenDetails, err = decodeOpenAITokenDetailsPtrFromMapField(m, "input_token_details")
	if err != nil {
		return err
	}
	u.OutputTokenDetails, err = decodeOpenAITokenDetailsPtrFromMapField(m, "output_token_details")
	return err
}

func (u *OpenAIResponsesUsage) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "input_tokens", u.InputTokens)
	setMapInt(out, "output_tokens", u.OutputTokens)
	setMapInt(out, "total_tokens", u.TotalTokens)
	if u.InputTokenDetails != nil {
		details, err := u.InputTokenDetails.ToMap()
		if err != nil {
			return nil, err
		}
		out["input_token_details"] = details
	}
	if u.OutputTokenDetails != nil {
		details, err := u.OutputTokenDetails.ToMap()
		if err != nil {
			return nil, err
		}
		out["output_token_details"] = details
	}
	return out, nil
}

type OpenAIResponsesResponse struct {
	ID                 string                         `json:"id,omitempty"`
	Object             string                         `json:"object,omitempty"`
	CreatedAt          int64                          `json:"created_at,omitempty"`
	Model              string                         `json:"model,omitempty"`
	Status             string                         `json:"status,omitempty"`
	CompletedAt        int64                          `json:"completed_at,omitempty"`
	Instructions       *OpenAIResponseInput           `json:"instructions,omitempty"`
	MaxOutputTokens    int                            `json:"max_output_tokens,omitempty"`
	MaxToolCalls       int                            `json:"max_tool_calls,omitempty"`
	OutputText         string                         `json:"output_text,omitempty"`
	Output             []OpenAIResponseOutputItem     `json:"output,omitempty"`
	ParallelToolCalls  *bool                          `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID string                         `json:"previous_response_id,omitempty"`
	Reasoning          *OpenAIResponseReasoningConfig `json:"reasoning,omitempty"`
	ServiceTier        string                         `json:"service_tier,omitempty"`
	Store              *bool                          `json:"store,omitempty"`
	Temperature        *float64                       `json:"temperature,omitempty"`
	Text               *OpenAIResponsesTextConfig     `json:"text,omitempty"`
	ToolChoice         *OpenAIResponsesToolChoice     `json:"tool_choice,omitempty"`
	Tools              []OpenAIResponseTool           `json:"tools,omitempty"`
	TopLogprobs        int                            `json:"top_logprobs,omitempty"`
	TopP               *float64                       `json:"top_p,omitempty"`
	Truncation         string                         `json:"truncation,omitempty"`
	Usage              *OpenAIResponsesUsage          `json:"usage,omitempty"`
	Error              *Error                         `json:"error,omitempty"`
	Metadata           map[string]string              `json:"metadata,omitempty"`
	User               string                         `json:"user,omitempty"`
}

func (r *OpenAIResponsesResponse) FromMap(m map[string]any) error {
	var err error
	r.ID, err = stringValue(m, "id")
	if err != nil {
		return err
	}
	r.Object, err = stringValue(m, "object")
	if err != nil {
		return err
	}
	createdAt, err := toInt64(m["created_at"])
	if err == nil {
		r.CreatedAt = createdAt
	} else if _, ok := m["created_at"]; ok && m["created_at"] != nil {
		return err
	}
	r.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	r.Status, err = stringValue(m, "status")
	if err != nil {
		return err
	}
	if m["completed_at"] != nil {
		completedAt, convErr := toInt64(m["completed_at"])
		if convErr != nil {
			return convErr
		}
		r.CompletedAt = completedAt
	}
	r.Instructions, err = decodeOpenAIResponseInputPtrFromMapField(m, "instructions")
	if err != nil {
		return err
	}
	r.MaxOutputTokens, err = intValue(m, "max_output_tokens")
	if err != nil {
		return err
	}
	r.MaxToolCalls, err = intValue(m, "max_tool_calls")
	if err != nil {
		return err
	}
	r.OutputText, err = stringValue(m, "output_text")
	if err != nil {
		return err
	}
	r.Output, err = decodeOpenAIResponseOutputItemListFromMapField(m, "output")
	if err != nil {
		return err
	}
	r.ParallelToolCalls, err = boolPtrValue(m, "parallel_tool_calls")
	if err != nil {
		return err
	}
	r.PreviousResponseID, err = stringValue(m, "previous_response_id")
	if err != nil {
		return err
	}
	r.Reasoning, err = decodeOpenAIResponseReasoningConfigPtrFromMapField(m, "reasoning")
	if err != nil {
		return err
	}
	r.ServiceTier, err = stringValue(m, "service_tier")
	if err != nil {
		return err
	}
	r.Store, err = boolPtrValue(m, "store")
	if err != nil {
		return err
	}
	r.Temperature, err = floatPtrValue(m, "temperature")
	if err != nil {
		return err
	}
	r.Text, err = decodeOpenAIResponsesTextConfigPtrFromMapField(m, "text")
	if err != nil {
		return err
	}
	r.ToolChoice, err = decodeOpenAIResponsesToolChoicePtrFromMapField(m, "tool_choice")
	if err != nil {
		return err
	}
	r.Tools, err = decodeOpenAIResponseToolListFromMapField(m, "tools")
	if err != nil {
		return err
	}
	r.TopLogprobs, err = intValue(m, "top_logprobs")
	if err != nil {
		return err
	}
	r.TopP, err = floatPtrValue(m, "top_p")
	if err != nil {
		return err
	}
	r.Truncation, err = stringValue(m, "truncation")
	if err != nil {
		return err
	}
	r.Usage, err = decodeOpenAIResponsesUsagePtrFromMapField(m, "usage")
	if err != nil {
		return err
	}
	r.Error, err = decodeGeminiErrorPtrFromMapField(m, "error")
	if err != nil {
		return err
	}
	r.Metadata, err = mapStringStringValue(m, "metadata")
	if err != nil {
		return err
	}
	r.User, err = stringValue(m, "user")
	return err
}

func (r *OpenAIResponsesResponse) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "id", r.ID)
	setMapString(out, "object", r.Object)
	if r.CreatedAt != 0 {
		out["created_at"] = r.CreatedAt
	}
	setMapString(out, "model", r.Model)
	setMapString(out, "status", r.Status)
	if r.CompletedAt != 0 {
		out["completed_at"] = r.CompletedAt
	}
	if r.Instructions != nil {
		instructions, err := r.Instructions.ToAny()
		if err != nil {
			return nil, err
		}
		out["instructions"] = instructions
	}
	setMapInt(out, "max_output_tokens", r.MaxOutputTokens)
	setMapInt(out, "max_tool_calls", r.MaxToolCalls)
	setMapString(out, "output_text", r.OutputText)
	if len(r.Output) > 0 {
		output, err := openAIResponseOutputItemListToMaps(r.Output)
		if err != nil {
			return nil, err
		}
		out["output"] = output
	}
	if r.ParallelToolCalls != nil {
		out["parallel_tool_calls"] = *r.ParallelToolCalls
	}
	setMapString(out, "previous_response_id", r.PreviousResponseID)
	if r.Reasoning != nil {
		reasoning, err := r.Reasoning.ToMap()
		if err != nil {
			return nil, err
		}
		out["reasoning"] = reasoning
	}
	setMapString(out, "service_tier", r.ServiceTier)
	if r.Store != nil {
		out["store"] = *r.Store
	}
	if r.Temperature != nil {
		out["temperature"] = *r.Temperature
	}
	if r.Text != nil {
		text, err := r.Text.ToMap()
		if err != nil {
			return nil, err
		}
		out["text"] = text
	}
	if r.ToolChoice != nil {
		toolChoice, err := r.ToolChoice.ToAny()
		if err != nil {
			return nil, err
		}
		out["tool_choice"] = toolChoice
	}
	if len(r.Tools) > 0 {
		tools, err := openAIResponseToolListToMaps(r.Tools)
		if err != nil {
			return nil, err
		}
		out["tools"] = tools
	}
	setMapInt(out, "top_logprobs", r.TopLogprobs)
	if r.TopP != nil {
		out["top_p"] = *r.TopP
	}
	setMapString(out, "truncation", r.Truncation)
	if r.Usage != nil {
		usage, err := r.Usage.ToMap()
		if err != nil {
			return nil, err
		}
		out["usage"] = usage
	}
	if r.Error != nil {
		errMap, err := r.Error.ToMap()
		if err != nil {
			return nil, err
		}
		out["error"] = errMap
	}
	if r.Metadata != nil {
		metadata := make(map[string]any, len(r.Metadata))
		for k, v := range r.Metadata {
			metadata[k] = v
		}
		out["metadata"] = metadata
	}
	setMapString(out, "user", r.User)
	return out, nil
}

type OpenAIResponsesStreamResponse struct {
	Type           string                    `json:"type,omitempty"`
	Response       *OpenAIResponsesResponse  `json:"response,omitempty"`
	OutputIndex    int                       `json:"output_index,omitempty"`
	ContentIndex   int                       `json:"content_index,omitempty"`
	Item           *OpenAIResponseOutputItem `json:"item,omitempty"`
	Delta          string                    `json:"delta,omitempty"`
	Error          *Error                    `json:"error,omitempty"`
	Usage          *OpenAIResponsesUsage     `json:"usage,omitempty"`
	SequenceNumber int64                     `json:"sequence_number,omitempty"`
}

func (r *OpenAIResponsesStreamResponse) FromMap(m map[string]any) error {
	var err error
	r.Type, err = stringValue(m, "type")
	if err != nil {
		return err
	}
	r.Response, err = decodeOpenAIResponsesResponsePtrFromMapField(m, "response")
	if err != nil {
		return err
	}
	r.OutputIndex, err = intValue(m, "output_index")
	if err != nil {
		return err
	}
	r.ContentIndex, err = intValue(m, "content_index")
	if err != nil {
		return err
	}
	r.Item, err = decodeOpenAIResponseOutputItemPtrFromMapField(m, "item")
	if err != nil {
		return err
	}
	r.Delta, err = stringValue(m, "delta")
	if err != nil {
		return err
	}
	r.Error, err = decodeGeminiErrorPtrFromMapField(m, "error")
	if err != nil {
		return err
	}
	r.Usage, err = decodeOpenAIResponsesUsagePtrFromMapField(m, "usage")
	if err != nil {
		return err
	}
	seq, err := toInt64(m["sequence_number"])
	if err == nil {
		r.SequenceNumber = seq
	} else if _, ok := m["sequence_number"]; ok && m["sequence_number"] != nil {
		return err
	}
	return nil
}

func (r *OpenAIResponsesStreamResponse) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "type", r.Type)
	if r.Response != nil {
		response, err := r.Response.ToMap()
		if err != nil {
			return nil, err
		}
		out["response"] = response
	}
	setMapInt(out, "output_index", r.OutputIndex)
	setMapInt(out, "content_index", r.ContentIndex)
	if r.Item != nil {
		item, err := r.Item.ToMap()
		if err != nil {
			return nil, err
		}
		out["item"] = item
	}
	setMapString(out, "delta", r.Delta)
	if r.Error != nil {
		errMap, err := r.Error.ToMap()
		if err != nil {
			return nil, err
		}
		out["error"] = errMap
	}
	if r.Usage != nil {
		usage, err := r.Usage.ToMap()
		if err != nil {
			return nil, err
		}
		out["usage"] = usage
	}
	if r.SequenceNumber != 0 {
		out["sequence_number"] = r.SequenceNumber
	}
	return out, nil
}

func decodeOpenAIChatMessageFromMapField(m map[string]any, key string) (OpenAIChatMessage, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return OpenAIChatMessage{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return OpenAIChatMessage{}, nil
	}
	var out OpenAIChatMessage
	return out, out.FromMap(mv)
}

func decodeOpenAIChatMessageContentPtrFromMapField(m map[string]any, key string) (*OpenAIChatMessageContent, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIChatMessageContent
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIChatMessageListFromMapField(m map[string]any, key string) ([]OpenAIChatMessage, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatMessage, 0, len(items))
	for _, item := range items {
		var v OpenAIChatMessage
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIChatAudioParamPtrFromMapField(m map[string]any, key string) (*OpenAIChatAudioParam, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatAudioParam
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatFunctionCallOptionPtrFromMapField(m map[string]any, key string) (*OpenAIChatFunctionCallOption, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIChatFunctionCallOption
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIFunctionDefinitionPtrFromMapField(m map[string]any, key string) (*OpenAIFunctionDefinition, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIFunctionDefinition
	return &out, out.FromMap(mv)
}

func decodeOpenAIFunctionDefinitionListFromMapField(m map[string]any, key string) ([]OpenAIFunctionDefinition, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIFunctionDefinition, 0, len(items))
	for _, item := range items {
		var value OpenAIFunctionDefinition
		if err := value.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func decodeOpenAIChatPredictionPtrFromMapField(m map[string]any, key string) (*OpenAIChatPrediction, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatPrediction
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatResponseFormatPtrFromMapField(m map[string]any, key string) (*OpenAIChatResponseFormat, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatResponseFormat
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatResponseFormatJSONSchemaPtrFromMapField(m map[string]any, key string) (*OpenAIChatResponseFormatJSONSchema, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatResponseFormatJSONSchema
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatStopPtrFromMapField(m map[string]any, key string) (*OpenAIChatStop, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIChatStop
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIChatStreamOptionsPtrFromMapField(m map[string]any, key string) (*OpenAIChatStreamOptions, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatStreamOptions
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatToolChoicePtrFromMapField(m map[string]any, key string) (*OpenAIChatToolChoice, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIChatToolChoice
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIChatWebSearchOptionsPtrFromMapField(m map[string]any, key string) (*OpenAIChatWebSearchOptions, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatWebSearchOptions
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatToolListFromMapField(m map[string]any, key string) ([]OpenAIChatTool, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatTool, 0, len(items))
	for _, item := range items {
		var v OpenAIChatTool
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIChatAllowedToolsSpecFromMapField(m map[string]any, key string) (OpenAIChatAllowedToolsSpec, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return OpenAIChatAllowedToolsSpec{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return OpenAIChatAllowedToolsSpec{}, nil
	}
	var out OpenAIChatAllowedToolsSpec
	return out, out.FromMap(mv)
}

func decodeOpenAIChatNamedFunctionSelectorFromMapField(m map[string]any, key string) (OpenAIChatNamedFunctionSelector, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return OpenAIChatNamedFunctionSelector{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return OpenAIChatNamedFunctionSelector{}, nil
	}
	var out OpenAIChatNamedFunctionSelector
	return out, out.FromMap(mv)
}

func decodeOpenAIChatNamedCustomSelectorFromMapField(m map[string]any, key string) (OpenAIChatNamedCustomSelector, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return OpenAIChatNamedCustomSelector{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return OpenAIChatNamedCustomSelector{}, nil
	}
	var out OpenAIChatNamedCustomSelector
	return out, out.FromMap(mv)
}

func decodeOpenAIChatUserLocationPtrFromMapField(m map[string]any, key string) (*OpenAIChatUserLocation, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatUserLocation
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatApproximateLocationPtrFromMapField(m map[string]any, key string) (*OpenAIChatApproximateLocation, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatApproximateLocation
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatCustomToolDefinitionPtrFromMapField(m map[string]any, key string) (*OpenAIChatCustomToolDefinition, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatCustomToolDefinition
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatCustomToolFormatPtrFromMapField(m map[string]any, key string) (*OpenAIChatCustomToolFormat, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatCustomToolFormat
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatCustomGrammarRefPtrFromMapField(m map[string]any, key string) (*OpenAIChatCustomGrammarRef, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatCustomGrammarRef
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatAudioPtrFromMapField(m map[string]any, key string) (*OpenAIChatAudio, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatAudio
	return &out, out.FromMap(mv)
}

func decodeOpenAIFunctionCallPtrFromMapField(m map[string]any, key string) (*OpenAIFunctionCall, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIFunctionCall
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatCustomToolCallPtrFromMapField(m map[string]any, key string) (*OpenAIChatCustomToolCall, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatCustomToolCall
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatImageURLPtrFromMapField(m map[string]any, key string) (*OpenAIChatImageURL, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatImageURL
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatInputAudioPtrFromMapField(m map[string]any, key string) (*OpenAIChatInputAudio, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatInputAudio
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatFileInputPtrFromMapField(m map[string]any, key string) (*OpenAIChatFileInput, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatFileInput
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatAnnotationListFromMapField(m map[string]any, key string) ([]OpenAIChatAnnotation, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatAnnotation, 0, len(items))
	for _, item := range items {
		var value OpenAIChatAnnotation
		if err := value.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func decodeOpenAIChatURLCitationPtrFromMapField(m map[string]any, key string) (*OpenAIChatURLCitation, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatURLCitation
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatToolCallListFromMapField(m map[string]any, key string) ([]OpenAIChatToolCall, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatToolCall, 0, len(items))
	for _, item := range items {
		var v OpenAIChatToolCall
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAITokenDetailsPtrFromMapField(m map[string]any, key string) (*OpenAITokenDetails, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAITokenDetails
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatCompletionsChoiceListFromMapField(m map[string]any, key string) ([]OpenAIChatCompletionsChoice, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatCompletionsChoice, 0, len(items))
	for _, item := range items {
		var v OpenAIChatCompletionsChoice
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIChatCompletionsUsagePtrFromMapField(m map[string]any, key string) (*OpenAIChatCompletionsUsage, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIChatCompletionsUsage
	return &out, out.FromMap(mv)
}

func decodeOpenAIChatCompletionsDeltaFromMapField(m map[string]any, key string) (OpenAIChatCompletionsDelta, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return OpenAIChatCompletionsDelta{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return OpenAIChatCompletionsDelta{}, nil
	}
	var out OpenAIChatCompletionsDelta
	return out, out.FromMap(mv)
}

func decodeOpenAIChatToolCallDeltaListFromMapField(m map[string]any, key string) ([]OpenAIChatToolCallDelta, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatToolCallDelta, 0, len(items))
	for _, item := range items {
		var v OpenAIChatToolCallDelta
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIChatCompletionsChunkChoiceListFromMapField(m map[string]any, key string) ([]OpenAIChatCompletionsChunkChoice, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIChatCompletionsChunkChoice, 0, len(items))
	for _, item := range items {
		var v OpenAIChatCompletionsChunkChoice
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIResponseToolListFromMapField(m map[string]any, key string) ([]OpenAIResponseTool, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseTool, 0, len(items))
	for _, item := range items {
		var v OpenAIResponseTool
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIResponseConversationRefPtrFromMapField(m map[string]any, key string) (*OpenAIResponseConversationRef, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIResponseConversationRef
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIResponseInputPtrFromMapField(m map[string]any, key string) (*OpenAIResponseInput, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIResponseInput
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIResponseInputContentPtrFromMapField(m map[string]any, key string) (*OpenAIResponseInputContent, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIResponseInputContent
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIResponseReasoningConfigPtrFromMapField(m map[string]any, key string) (*OpenAIResponseReasoningConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponseReasoningConfig
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponsesStreamOptionsPtrFromMapField(m map[string]any, key string) (*OpenAIResponsesStreamOptions, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponsesStreamOptions
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponsesTextConfigPtrFromMapField(m map[string]any, key string) (*OpenAIResponsesTextConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponsesTextConfig
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponsesTextFormatPtrFromMapField(m map[string]any, key string) (*OpenAIResponsesTextFormat, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponsesTextFormat
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponsesToolChoicePtrFromMapField(m map[string]any, key string) (*OpenAIResponsesToolChoice, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	var out OpenAIResponsesToolChoice
	if err := out.FromAny(v); err != nil {
		return nil, err
	}
	return &out, nil
}

func decodeOpenAIResponseFileSearchRankingOptionsPtrFromMapField(m map[string]any, key string) (*OpenAIResponseFileSearchRankingOptions, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponseFileSearchRankingOptions
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponseContentPartListFromMapField(m map[string]any, key string) ([]OpenAIResponseContentPart, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseContentPart, 0, len(items))
	for _, item := range items {
		var v OpenAIResponseContentPart
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIResponseAnnotationListFromMapField(m map[string]any, key string) ([]OpenAIResponseAnnotation, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseAnnotation, 0, len(items))
	for _, item := range items {
		var value OpenAIResponseAnnotation
		if err := value.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func decodeOpenAIResponseTokenLogprobListFromMapField(m map[string]any, key string) ([]OpenAIResponseTokenLogprob, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseTokenLogprob, 0, len(items))
	for _, item := range items {
		var value OpenAIResponseTokenLogprob
		if err := value.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func decodeOpenAIResponseSummaryTextListFromMapField(m map[string]any, key string) ([]OpenAIResponseSummaryText, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseSummaryText, 0, len(items))
	for _, item := range items {
		var value OpenAIResponseSummaryText
		if err := value.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func decodeOpenAIResponseFileSearchResultListFromMapField(m map[string]any, key string) ([]OpenAIResponseFileSearchResult, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseFileSearchResult, 0, len(items))
	for _, item := range items {
		var value OpenAIResponseFileSearchResult
		if err := value.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func decodeOpenAIResponseOutputItemListFromMapField(m map[string]any, key string) ([]OpenAIResponseOutputItem, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]OpenAIResponseOutputItem, 0, len(items))
	for _, item := range items {
		var v OpenAIResponseOutputItem
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOpenAIResponsesUsagePtrFromMapField(m map[string]any, key string) (*OpenAIResponsesUsage, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponsesUsage
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponsesResponsePtrFromMapField(m map[string]any, key string) (*OpenAIResponsesResponse, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponsesResponse
	return &out, out.FromMap(mv)
}

func decodeOpenAIResponseOutputItemPtrFromMapField(m map[string]any, key string) (*OpenAIResponseOutputItem, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, nil
	}
	var out OpenAIResponseOutputItem
	return &out, out.FromMap(mv)
}

func openAIChatMessageListToMaps(items []OpenAIChatMessage) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIFunctionDefinitionListToMaps(items []OpenAIFunctionDefinition) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatToolListToMaps(items []OpenAIChatTool) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatContentPartListToMaps(items []OpenAIChatContentPart) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatAnnotationListToMaps(items []OpenAIChatAnnotation) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatToolCallListToMaps(items []OpenAIChatToolCall) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatCompletionsChoiceListToMaps(items []OpenAIChatCompletionsChoice) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatToolCallDeltaListToMaps(items []OpenAIChatToolCallDelta) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIChatCompletionsChunkChoiceListToMaps(items []OpenAIChatCompletionsChunkChoice) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseToolListToMaps(items []OpenAIResponseTool) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseAnnotationListToMaps(items []OpenAIResponseAnnotation) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseTokenLogprobListToMaps(items []OpenAIResponseTokenLogprob) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseContentPartListToMaps(items []OpenAIResponseContentPart) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseSummaryTextListToMaps(items []OpenAIResponseSummaryText) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseFileSearchResultListToMaps(items []OpenAIResponseFileSearchResult) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}

func openAIResponseOutputItemListToMaps(items []OpenAIResponseOutputItem) ([]any, error) {
	out := make([]any, 0, len(items))
	for i := range items {
		mv, err := (&items[i]).ToMap()
		if err != nil {
			return nil, err
		}
		out = append(out, mv)
	}
	return out, nil
}
