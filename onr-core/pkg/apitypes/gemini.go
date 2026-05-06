package apitypes

import (
	"fmt"
	"strings"
)

const (
	MediaResolutionLow       = "MEDIA_RESOLUTION_LOW"
	MediaResolutionMedium    = "MEDIA_RESOLUTION_MEDIUM"
	MediaResolutionHigh      = "MEDIA_RESOLUTION_HIGH"
	MediaResolutionUltraHigh = "MEDIA_RESOLUTION_ULTRA_HIGH"
)

type ChatRequest struct {
	Contents          []ChatContent        `json:"contents"`
	SafetySettings    []ChatSafetySettings `json:"safety_settings,omitempty"`
	GenerationConfig  ChatGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []ChatTools          `json:"tools,omitempty"`
	SystemInstruction *ChatContent         `json:"system_instruction,omitempty"`
}

func (r *ChatRequest) GetPrompt() string {
	builder := strings.Builder{}
	appendParts := func(parts []Part) {
		for _, part := range parts {
			if part.Text != "" {
				_, _ = builder.WriteString(part.Text)
			}
		}
	}
	for i := range r.Contents {
		role := strings.ToLower(r.Contents[i].Role)
		if role == "" || role == "user" {
			appendParts(r.Contents[i].Parts)
		}
	}
	if r.SystemInstruction != nil {
		appendParts(r.SystemInstruction.Parts)
	}
	return builder.String()
}

func (r *ChatRequest) FromMap(m map[string]any) error {
	var err error
	r.Contents, err = decodeChatContentListFromMapField(m, "contents")
	if err != nil {
		return err
	}
	r.SafetySettings, err = decodeChatSafetySettingsListFromMapField(m, "safety_settings")
	if err != nil {
		return err
	}
	r.GenerationConfig, err = decodeChatGenerationConfigFromMapField(m, "generationConfig")
	if err != nil {
		return err
	}
	r.Tools, err = decodeChatToolsListFromMapField(m, "tools")
	if err != nil {
		return err
	}
	r.SystemInstruction, err = decodeChatContentPtrFromMapField(m, "system_instruction")
	return err
}

func (r *ChatRequest) ToMap() (map[string]any, error) {
	out := map[string]any{}
	contents, err := chatContentListToMaps(r.Contents)
	if err != nil {
		return nil, err
	}
	out["contents"] = contents
	if len(r.SafetySettings) > 0 {
		safetySettings, err := chatSafetySettingsListToMaps(r.SafetySettings)
		if err != nil {
			return nil, err
		}
		out["safety_settings"] = safetySettings
	}
	if generationConfig, err := r.GenerationConfig.ToMap(); err != nil {
		return nil, err
	} else if len(generationConfig) > 0 {
		out["generationConfig"] = generationConfig
	}
	if len(r.Tools) > 0 {
		tools, err := chatToolsListToMaps(r.Tools)
		if err != nil {
			return nil, err
		}
		out["tools"] = tools
	}
	if r.SystemInstruction != nil {
		systemInstruction, err := r.SystemInstruction.ToMap()
		if err != nil {
			return nil, err
		}
		out["system_instruction"] = systemInstruction
	}
	return out, nil
}

type EmbeddingRequest struct {
	Model                string      `json:"model"`
	Content              ChatContent `json:"content"`
	TaskType             string      `json:"taskType,omitempty"`
	Title                string      `json:"title,omitempty"`
	OutputDimensionality int         `json:"outputDimensionality,omitempty"`
}

func (r *EmbeddingRequest) FromMap(m map[string]any) error {
	var err error
	r.Model, err = stringValue(m, "model")
	if err != nil {
		return err
	}
	r.Content, err = decodeChatContentFromMapField(m, "content")
	if err != nil {
		return err
	}
	r.TaskType, err = stringValue(m, "taskType")
	if err != nil {
		return err
	}
	r.Title, err = stringValue(m, "title")
	if err != nil {
		return err
	}
	r.OutputDimensionality, err = intValue(m, "outputDimensionality")
	return err
}

func (r *EmbeddingRequest) ToMap() (map[string]any, error) {
	content, err := r.Content.ToMap()
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"model":   r.Model,
		"content": content,
	}
	setMapString(out, "taskType", r.TaskType)
	setMapString(out, "title", r.Title)
	setMapInt(out, "outputDimensionality", r.OutputDimensionality)
	return out, nil
}

type BatchEmbeddingRequest struct {
	Requests []EmbeddingRequest `json:"requests"`
}

func (r *BatchEmbeddingRequest) FromMap(m map[string]any) error {
	var err error
	r.Requests, err = decodeEmbeddingRequestListFromMapField(m, "requests")
	return err
}

func (r *BatchEmbeddingRequest) ToMap() (map[string]any, error) {
	requests, err := embeddingRequestListToMaps(r.Requests)
	if err != nil {
		return nil, err
	}
	return map[string]any{"requests": requests}, nil
}

type EmbeddingData struct {
	Values []float64 `json:"values"`
}

func (d *EmbeddingData) FromMap(m map[string]any) error {
	values, ok := mapValue(m, "values")
	if !ok || values == nil {
		d.Values = nil
		return nil
	}
	items, ok := values.([]any)
	if !ok {
		return fmt.Errorf("values must be []any, got %T", values)
	}
	out := make([]float64, 0, len(items))
	for _, item := range items {
		v, err := toFloat64(item)
		if err != nil {
			return err
		}
		out = append(out, v)
	}
	d.Values = out
	return nil
}

func (d *EmbeddingData) ToMap() (map[string]any, error) {
	values := make([]any, 0, len(d.Values))
	for _, v := range d.Values {
		values = append(values, v)
	}
	return map[string]any{"values": values}, nil
}

type EmbeddingResponse struct {
	Embeddings    []EmbeddingData `json:"embeddings"`
	Error         *Error          `json:"error,omitempty"`
	UsageMetadata *UsageMetadata  `json:"usageMetadata,omitempty"`
}

func (r *EmbeddingResponse) FromMap(m map[string]any) error {
	var err error
	r.Embeddings, err = decodeEmbeddingDataListFromMapField(m, "embeddings")
	if err != nil {
		return err
	}
	r.Error, err = decodeGeminiErrorPtrFromMapField(m, "error")
	if err != nil {
		return err
	}
	r.UsageMetadata, err = decodeUsageMetadataPtrFromMapField(m, "usageMetadata")
	return err
}

func (r *EmbeddingResponse) ToMap() (map[string]any, error) {
	out := map[string]any{}
	embeddings, err := embeddingDataListToMaps(r.Embeddings)
	if err != nil {
		return nil, err
	}
	out["embeddings"] = embeddings
	if r.Error != nil {
		errMap, err := r.Error.ToMap()
		if err != nil {
			return nil, err
		}
		out["error"] = errMap
	}
	if r.UsageMetadata != nil {
		usageMetadata, err := r.UsageMetadata.ToMap()
		if err != nil {
			return nil, err
		}
		out["usageMetadata"] = usageMetadata
	}
	return out, nil
}

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
}

func (e *Error) FromMap(m map[string]any) error {
	var err error
	e.Code, err = intValue(m, "code")
	if err != nil {
		return err
	}
	e.Message, err = stringValue(m, "message")
	if err != nil {
		return err
	}
	e.Status, err = stringValue(m, "status")
	return err
}

func (e *Error) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapInt(out, "code", e.Code)
	setMapString(out, "message", e.Message)
	setMapString(out, "status", e.Status)
	return out, nil
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

func (d *InlineData) FromMap(m map[string]any) error {
	var err error
	d.MimeType, err = stringValue(m, "mimeType")
	if err != nil {
		return err
	}
	d.Data, err = stringValue(m, "data")
	return err
}

func (d *InlineData) ToMap() (map[string]any, error) {
	return map[string]any{
		"mimeType": d.MimeType,
		"data":     d.Data,
	}, nil
}

type FunctionCall struct {
	FunctionName string `json:"name"`
	Arguments    any    `json:"args"`
}

func (f *FunctionCall) FromMap(m map[string]any) error {
	var err error
	f.FunctionName, err = stringValue(m, "name")
	if err != nil {
		return err
	}
	f.Arguments, _ = mapValue(m, "args")
	return nil
}

func (f *FunctionCall) ToMap() (map[string]any, error) {
	out := map[string]any{"name": f.FunctionName}
	if f.Arguments != nil {
		out["args"] = f.Arguments
	}
	return out, nil
}

type Part struct {
	Text            string        `json:"text,omitempty"`
	InlineData      *InlineData   `json:"inlineData,omitempty"`
	FunctionCall    *FunctionCall `json:"functionCall,omitempty"`
	MediaResolution string        `json:"mediaResolution,omitempty"`
}

func (p *Part) FromMap(m map[string]any) error {
	var err error
	p.Text, err = stringValue(m, "text")
	if err != nil {
		return err
	}
	p.InlineData, err = decodeInlineDataPtrFromMapField(m, "inlineData")
	if err != nil {
		return err
	}
	p.FunctionCall, err = decodeFunctionCallPtrFromMapField(m, "functionCall")
	if err != nil {
		return err
	}
	p.MediaResolution, err = stringValue(m, "mediaResolution")
	return err
}

func (p *Part) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "text", p.Text)
	if p.InlineData != nil {
		inlineData, err := p.InlineData.ToMap()
		if err != nil {
			return nil, err
		}
		out["inlineData"] = inlineData
	}
	if p.FunctionCall != nil {
		functionCall, err := p.FunctionCall.ToMap()
		if err != nil {
			return nil, err
		}
		out["functionCall"] = functionCall
	}
	setMapString(out, "mediaResolution", p.MediaResolution)
	return out, nil
}

type ChatContent struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

func (c *ChatContent) FromMap(m map[string]any) error {
	var err error
	c.Role, err = stringValue(m, "role")
	if err != nil {
		return err
	}
	c.Parts, err = decodePartListFromMapField(m, "parts")
	return err
}

func (c *ChatContent) ToMap() (map[string]any, error) {
	parts, err := partListToMaps(c.Parts)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"parts": parts}
	setMapString(out, "role", c.Role)
	return out, nil
}

// GenerateContentStreamResponse models one Gemini SSE data payload returned by
// streamGenerateContent endpoints.
type GenerateContentStreamResponse struct {
	Candidates    []GenerateContentCandidate `json:"candidates,omitempty"`
	UsageMetadata *UsageMetadata             `json:"usageMetadata,omitempty"`
	ModelVersion  string                     `json:"modelVersion,omitempty"`
	Model         string                     `json:"model,omitempty"`
}

// GenerateContentCandidate is a streamed candidate unit in Gemini responses.
type GenerateContentCandidate struct {
	Content      ChatContent `json:"content"`
	FinishReason string      `json:"finishReason,omitempty"`
	Index        int         `json:"index,omitempty"`
}

type ChatSafetySettings struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

func (s *ChatSafetySettings) FromMap(m map[string]any) error {
	var err error
	s.Category, err = stringValue(m, "category")
	if err != nil {
		return err
	}
	s.Threshold, err = stringValue(m, "threshold")
	return err
}

func (s *ChatSafetySettings) ToMap() (map[string]any, error) {
	return map[string]any{
		"category":  s.Category,
		"threshold": s.Threshold,
	}, nil
}

type ChatTools struct {
	FunctionDeclarations any `json:"function_declarations,omitempty"`
}

func (t *ChatTools) FromMap(m map[string]any) error {
	t.FunctionDeclarations, _ = mapValue(m, "function_declarations")
	return nil
}

func (t *ChatTools) ToMap() (map[string]any, error) {
	out := map[string]any{}
	if t.FunctionDeclarations != nil {
		out["function_declarations"] = t.FunctionDeclarations
	}
	return out, nil
}

type ImageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

func (c *ImageConfig) FromMap(m map[string]any) error {
	var err error
	c.AspectRatio, err = stringValue(m, "aspectRatio")
	if err != nil {
		return err
	}
	c.ImageSize, err = stringValue(m, "imageSize")
	return err
}

func (c *ImageConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "aspectRatio", c.AspectRatio)
	setMapString(out, "imageSize", c.ImageSize)
	return out, nil
}

type GeminiThinkingConfig struct {
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
	IncludeThoughts *bool  `json:"includeThoughts,omitempty"`
}

func (c *GeminiThinkingConfig) FromMap(m map[string]any) error {
	var err error
	c.ThinkingLevel, err = stringValue(m, "thinkingLevel")
	if err != nil {
		return err
	}
	c.ThinkingBudget, err = intPtrValue(m, "thinkingBudget")
	if err != nil {
		return err
	}
	c.IncludeThoughts, err = boolPtrValue(m, "includeThoughts")
	return err
}

func (c *GeminiThinkingConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "thinkingLevel", c.ThinkingLevel)
	if c.ThinkingBudget != nil {
		out["thinkingBudget"] = *c.ThinkingBudget
	}
	if c.IncludeThoughts != nil {
		out["includeThoughts"] = *c.IncludeThoughts
	}
	return out, nil
}

type ChatGenerationConfig struct {
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	ResponseModalities []string              `json:"responseModalities,omitempty"`
	ResponseSchema     any                   `json:"responseSchema,omitempty"`
	ImageConfig        *ImageConfig          `json:"imageConfig,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
	MediaResolution    string                `json:"mediaResolution,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	TopK               float64               `json:"topK,omitempty"`
	MaxOutputTokens    int                   `json:"maxOutputTokens,omitempty"`
	CandidateCount     int                   `json:"candidateCount,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
}

func (c *ChatGenerationConfig) FromMap(m map[string]any) error {
	var err error
	c.ResponseMimeType, err = stringValue(m, "responseMimeType")
	if err != nil {
		return err
	}
	c.ResponseModalities, err = stringSliceValue(m, "responseModalities")
	if err != nil {
		return err
	}
	c.ResponseSchema, _ = mapValue(m, "responseSchema")
	c.ImageConfig, err = decodeImageConfigPtrFromMapField(m, "imageConfig")
	if err != nil {
		return err
	}
	c.ThinkingConfig, err = decodeGeminiThinkingConfigPtrFromMapField(m, "thinkingConfig")
	if err != nil {
		return err
	}
	c.MediaResolution, err = stringValue(m, "mediaResolution")
	if err != nil {
		return err
	}
	c.Temperature, err = floatPtrValue(m, "temperature")
	if err != nil {
		return err
	}
	c.TopP, err = floatPtrValue(m, "topP")
	if err != nil {
		return err
	}
	c.TopK, err = floatValue(m, "topK")
	if err != nil {
		return err
	}
	c.MaxOutputTokens, err = intValue(m, "maxOutputTokens")
	if err != nil {
		return err
	}
	c.CandidateCount, err = intValue(m, "candidateCount")
	if err != nil {
		return err
	}
	c.StopSequences, err = stringSliceValue(m, "stopSequences")
	return err
}

func (c *ChatGenerationConfig) ToMap() (map[string]any, error) {
	out := map[string]any{}
	setMapString(out, "responseMimeType", c.ResponseMimeType)
	setMapStringSlice(out, "responseModalities", c.ResponseModalities)
	if c.ResponseSchema != nil {
		out["responseSchema"] = c.ResponseSchema
	}
	if c.ImageConfig != nil {
		imageConfig, err := c.ImageConfig.ToMap()
		if err != nil {
			return nil, err
		}
		out["imageConfig"] = imageConfig
	}
	if c.ThinkingConfig != nil {
		thinkingConfig, err := c.ThinkingConfig.ToMap()
		if err != nil {
			return nil, err
		}
		out["thinkingConfig"] = thinkingConfig
	}
	setMapString(out, "mediaResolution", c.MediaResolution)
	if c.Temperature != nil {
		out["temperature"] = *c.Temperature
	}
	if c.TopP != nil {
		out["topP"] = *c.TopP
	}
	if c.TopK != 0 {
		out["topK"] = c.TopK
	}
	setMapInt(out, "maxOutputTokens", c.MaxOutputTokens)
	setMapInt(out, "candidateCount", c.CandidateCount)
	setMapStringSlice(out, "stopSequences", c.StopSequences)
	return out, nil
}

type UsageMetadata struct {
	PromptTokenCount        int                  `json:"promptTokenCount"`
	CandidatesTokenCount    int                  `json:"candidatesTokenCount"`
	TotalTokenCount         int                  `json:"totalTokenCount"`
	ThoughtsTokenCount      int                  `json:"thoughtsTokenCount,omitempty"`
	CachedContentTokenCount int                  `json:"cachedContentTokenCount,omitempty"`
	PromptTokensDetails     []ModalityTokenCount `json:"promptTokensDetails,omitempty"`
	CandidatesTokensDetails []ModalityTokenCount `json:"candidatesTokensDetails,omitempty"`
}

func (u *UsageMetadata) FromMap(m map[string]any) error {
	var err error
	u.PromptTokenCount, err = intValue(m, "promptTokenCount")
	if err != nil {
		return err
	}
	u.CandidatesTokenCount, err = intValue(m, "candidatesTokenCount")
	if err != nil {
		return err
	}
	u.TotalTokenCount, err = intValue(m, "totalTokenCount")
	if err != nil {
		return err
	}
	u.ThoughtsTokenCount, err = intValue(m, "thoughtsTokenCount")
	if err != nil {
		return err
	}
	u.CachedContentTokenCount, err = intValue(m, "cachedContentTokenCount")
	if err != nil {
		return err
	}
	u.PromptTokensDetails, err = decodeModalityTokenCountListFromMapField(m, "promptTokensDetails")
	if err != nil {
		return err
	}
	u.CandidatesTokensDetails, err = decodeModalityTokenCountListFromMapField(m, "candidatesTokensDetails")
	return err
}

func (u *UsageMetadata) ToMap() (map[string]any, error) {
	out := map[string]any{
		"promptTokenCount":     u.PromptTokenCount,
		"candidatesTokenCount": u.CandidatesTokenCount,
		"totalTokenCount":      u.TotalTokenCount,
	}
	setMapInt(out, "thoughtsTokenCount", u.ThoughtsTokenCount)
	setMapInt(out, "cachedContentTokenCount", u.CachedContentTokenCount)
	if len(u.PromptTokensDetails) > 0 {
		promptTokensDetails, err := modalityTokenCountListToMaps(u.PromptTokensDetails)
		if err != nil {
			return nil, err
		}
		out["promptTokensDetails"] = promptTokensDetails
	}
	if len(u.CandidatesTokensDetails) > 0 {
		candidatesTokensDetails, err := modalityTokenCountListToMaps(u.CandidatesTokensDetails)
		if err != nil {
			return nil, err
		}
		out["candidatesTokensDetails"] = candidatesTokensDetails
	}
	return out, nil
}

type ModalityTokenCount struct {
	Modality   string `json:"modality"`
	TokenCount int    `json:"tokenCount"`
}

func (m *ModalityTokenCount) FromMap(v map[string]any) error {
	var err error
	m.Modality, err = stringValue(v, "modality")
	if err != nil {
		return err
	}
	m.TokenCount, err = intValue(v, "tokenCount")
	return err
}

func (m *ModalityTokenCount) ToMap() (map[string]any, error) {
	return map[string]any{
		"modality":   m.Modality,
		"tokenCount": m.TokenCount,
	}, nil
}

type GeminiGenerateContentRequest = ChatRequest

func decodeChatContentFromMapField(m map[string]any, key string) (ChatContent, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return ChatContent{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return ChatContent{}, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ChatContent
	return out, out.FromMap(mv)
}

func decodeChatContentPtrFromMapField(m map[string]any, key string) (*ChatContent, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ChatContent
	return &out, out.FromMap(mv)
}

func decodeChatContentListFromMapField(m map[string]any, key string) ([]ChatContent, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ChatContent, 0, len(items))
	for _, item := range items {
		var v ChatContent
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodePartListFromMapField(m map[string]any, key string) ([]Part, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]Part, 0, len(items))
	for _, item := range items {
		var v Part
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeChatSafetySettingsListFromMapField(m map[string]any, key string) ([]ChatSafetySettings, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ChatSafetySettings, 0, len(items))
	for _, item := range items {
		var v ChatSafetySettings
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeChatToolsListFromMapField(m map[string]any, key string) ([]ChatTools, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ChatTools, 0, len(items))
	for _, item := range items {
		var v ChatTools
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeEmbeddingRequestListFromMapField(m map[string]any, key string) ([]EmbeddingRequest, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]EmbeddingRequest, 0, len(items))
	for _, item := range items {
		var v EmbeddingRequest
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeEmbeddingDataListFromMapField(m map[string]any, key string) ([]EmbeddingData, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]EmbeddingData, 0, len(items))
	for _, item := range items {
		var v EmbeddingData
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeModalityTokenCountListFromMapField(m map[string]any, key string) ([]ModalityTokenCount, error) {
	items, err := mapListValue(m, key)
	if err != nil {
		return nil, err
	}
	out := make([]ModalityTokenCount, 0, len(items))
	for _, item := range items {
		var v ModalityTokenCount
		if err := v.FromMap(item); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeInlineDataPtrFromMapField(m map[string]any, key string) (*InlineData, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out InlineData
	return &out, out.FromMap(mv)
}

func decodeFunctionCallPtrFromMapField(m map[string]any, key string) (*FunctionCall, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out FunctionCall
	return &out, out.FromMap(mv)
}

func decodeImageConfigPtrFromMapField(m map[string]any, key string) (*ImageConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ImageConfig
	return &out, out.FromMap(mv)
}

func decodeGeminiThinkingConfigPtrFromMapField(m map[string]any, key string) (*GeminiThinkingConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out GeminiThinkingConfig
	return &out, out.FromMap(mv)
}

func decodeChatGenerationConfigFromMapField(m map[string]any, key string) (ChatGenerationConfig, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return ChatGenerationConfig{}, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return ChatGenerationConfig{}, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out ChatGenerationConfig
	return out, out.FromMap(mv)
}

func decodeGeminiErrorPtrFromMapField(m map[string]any, key string) (*Error, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out Error
	return &out, out.FromMap(mv)
}

func decodeUsageMetadataPtrFromMapField(m map[string]any, key string) (*UsageMetadata, error) {
	v, ok := mapValue(m, key)
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	var out UsageMetadata
	return &out, out.FromMap(mv)
}

func partListToMaps(items []Part) ([]any, error) {
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

func chatContentListToMaps(items []ChatContent) ([]any, error) {
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

func chatSafetySettingsListToMaps(items []ChatSafetySettings) ([]any, error) {
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

func chatToolsListToMaps(items []ChatTools) ([]any, error) {
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

func embeddingRequestListToMaps(items []EmbeddingRequest) ([]any, error) {
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

func embeddingDataListToMaps(items []EmbeddingData) ([]any, error) {
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

func modalityTokenCountListToMaps(items []ModalityTokenCount) ([]any, error) {
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
