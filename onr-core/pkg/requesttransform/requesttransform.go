package requesttransform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
)

const contentEncodingIdentity = "identity"

type ApplyOptions struct {
	ContentEncoding string
	RequestHeaders  http.Header
}

type Result struct {
	Body []byte
	// Value is the transformed request object root. It is nil only when callers provide
	// no parsed object root and req_map is not used to rebuild one from raw body.
	Value       map[string]any
	Root        map[string]any
	ContentType string
}

// Apply transforms a request body and parsed request object root using request transform rules.
// meta and t must be non-nil.
// value must already be normalized to a top-level JSON object root. It may be nil when callers
// only have raw body bytes and want req_map to parse the object on demand.
func Apply(meta *dslmeta.Meta, contentType string, body []byte, value map[string]any, t *dslconfig.RequestTransform, opts ApplyOptions) (Result, error) {
	if opts.RequestHeaders != nil {
		meta.RequestHeaders = opts.RequestHeaders
	}
	result := Result{
		Body:        body,
		Value:       value,
		Root:        value,
		ContentType: strings.TrimSpace(contentType),
	}

	out := value
	changed := false
	modelMapped := strings.TrimSpace(meta.DSLModelMapped)
	if out != nil {
		if modelMapped != "" {
			if _, exists := out["model"]; exists {
				if current, ok := out["model"].(string); !ok || current != modelMapped {
					out["model"] = modelMapped
					changed = true
				}
			}
		}
	}

	if out != nil && len(t.JSONOps) > 0 {
		out, err := dslconfig.ApplyJSONOps(meta, out, t.JSONOps)
		if err != nil {
			return Result{}, err
		}
		changed = true
		result.Value = out
		result.Root = out
	}

	reqMapMode := strings.TrimSpace(t.ReqMapMode)
	if reqMapMode == "" {
		if out != nil && len(t.AfterReqMapJSONOps) > 0 {
			out, err := dslconfig.ApplyJSONOps(meta, out, t.AfterReqMapJSONOps)
			if err != nil {
				return Result{}, err
			}
			changed = true
			result.Value = out
			result.Root = out
		}
		if out != nil && (changed || result.Body == nil) {
			reqBody, err := marshalBody(result.Body, out, result.ContentType)
			if err != nil {
				return Result{}, err
			}
			result.Body = reqBody
		}
		return result, nil
	}

	mappedBody, mappedRoot, err := ApplyReqMap(reqMapMode, result.Body, out, opts)
	if err != nil {
		return Result{}, err
	}
	result.Body = mappedBody

	result.Value = mappedRoot
	result.Root = mappedRoot
	if mappedRoot != nil && len(t.AfterReqMapJSONOps) > 0 {
		mappedRoot, err = dslconfig.ApplyJSONOps(meta, mappedRoot, t.AfterReqMapJSONOps)
		if err != nil {
			return Result{}, err
		}
		reqBody, err := marshalBody(result.Body, mappedRoot, result.ContentType)
		if err != nil {
			return Result{}, err
		}
		result.Body = reqBody
		result.Value = mappedRoot
		result.Root = mappedRoot
	}
	return result, nil
}

// ApplyReqMap remaps a request object root to another object-root request schema.
// out should be the already-parsed request object root when available; otherwise raw is reparsed.
func ApplyReqMap(mode string, raw []byte, out map[string]any, opts ApplyOptions) ([]byte, map[string]any, error) {
	ce := strings.ToLower(strings.TrimSpace(opts.ContentEncoding))
	if ce != "" && ce != contentEncodingIdentity {
		return nil, nil, fmt.Errorf("cannot transform encoded client request (Content-Encoding=%q)", opts.ContentEncoding)
	}
	if out != nil {
		return applyReqMapObject(mode, apitypes.JSONObject(out))
	}
	root, err := parseReqMapInputObject(mode, raw)
	if err != nil {
		return nil, nil, err
	}
	return applyReqMapObject(mode, root)
}

func applyReqMapObject(mode string, root apitypes.JSONObject) ([]byte, map[string]any, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "openai_chat_to_openai_responses":
		var src apitypes.OpenAIChatCompletionsRequest
		if err := src.FromMap(root); err != nil {
			return nil, nil, err
		}
		dst, err := mapOpenAIChatCompletionsToResponsesRequest(&src)
		if err != nil {
			return nil, nil, err
		}
		return marshalReqMapResult(dst)
	case "openai_chat_to_anthropic_messages":
		var src apitypes.OpenAIChatCompletionsRequest
		if err := src.FromMap(root); err != nil {
			return nil, nil, err
		}
		dst, err := mapOpenAIChatCompletionsToClaudeRequest(&src)
		if err != nil {
			return nil, nil, err
		}
		return marshalReqMapResult(dst)
	case "openai_chat_to_gemini_generate_content":
		var src apitypes.OpenAIChatCompletionsRequest
		if err := src.FromMap(root); err != nil {
			return nil, nil, err
		}
		dst := mapOpenAIChatCompletionsToGeminiGenerateContentRequest(&src)
		return marshalReqMapResult(dst)
	case "anthropic_to_openai_chat":
		var src apitypes.ClaudeRequest
		if err := src.FromMap(root); err != nil {
			return nil, nil, err
		}
		dst, err := mapClaudeRequestToOpenAIChatCompletions(&src)
		if err != nil {
			return nil, nil, err
		}
		return marshalReqMapResult(dst)
	case "gemini_to_openai_chat":
		var src apitypes.GeminiGenerateContentRequest
		if err := src.FromMap(root); err != nil {
			return nil, nil, err
		}
		dst, err := mapGeminiGenerateContentRequestToOpenAIChatCompletions(&src)
		if err != nil {
			return nil, nil, err
		}
		return marshalReqMapResult(dst)
	default:
		return nil, nil, fmt.Errorf("unsupported req_map mode %q", mode)
	}
}

func marshalReqMapResult(value apitypes.ToMapper) ([]byte, map[string]any, error) {
	root, err := value.ToMap()
	if err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(root)
	if err != nil {
		return nil, nil, err
	}
	return body, root, nil
}

const (
	defaultAnthropicMaxTokens = 64 * 1000

	claudeReasoningBudgetLow    = 1024
	claudeReasoningBudgetMedium = 8 * 1024
	claudeReasoningBudgetHigh   = 16 * 1024
	claudeReasoningBudgetXHigh  = 32 * 1024
)

var claudeModelMaxTokensExact = map[string]int{
	"claude-opus-4-6": 128 * 1000,
	"claude-opus-4-1": 32 * 1000,
	"claude-opus-4-0": 32 * 1000,
}

var claudeModelMaxTokensPrefixes = map[string]int{
	"claude-opus":   64 * 1000,
	"claude-sonnet": 64 * 1000,
	"claude-haiku":  64 * 1000,
}

var reasoningEffortBudgetMap = map[string]int{
	"low":    claudeReasoningBudgetLow,
	"medium": claudeReasoningBudgetMedium,
	"high":   claudeReasoningBudgetHigh,
	"xhigh":  claudeReasoningBudgetXHigh,
}

// mapOpenAIChatCompletionsToResponsesRequest requires a non-nil typed OpenAI chat request.
func mapOpenAIChatCompletionsToResponsesRequest(req *apitypes.OpenAIChatCompletionsRequest) (*apitypes.OpenAIResponsesRequest, error) {
	srcMap, err := req.ToMap()
	if err != nil {
		return nil, err
	}
	mappedObj, err := apitransform.MapOpenAIChatCompletionsToResponsesObject(apitypes.JSONObject(srcMap))
	if err != nil {
		return nil, err
	}
	var dst apitypes.OpenAIResponsesRequest
	if err := dst.FromMap(mappedObj); err != nil {
		return nil, err
	}
	return &dst, nil
}

// mapOpenAIChatCompletionsToClaudeRequest requires a non-nil typed OpenAI chat request.
func mapOpenAIChatCompletionsToClaudeRequest(req *apitypes.OpenAIChatCompletionsRequest) (*apitypes.ClaudeRequest, error) {
	tools, toolChoice := buildClaudeToolsAndChoice(req)
	dst := &apitypes.ClaudeRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       tools,
		ToolChoice:  toolChoice,
		ServiceTier: req.ServiceTier,
	}
	if req.Stream {
		stream := true
		dst.Stream = &stream
	}
	if req.User != "" {
		dst.Metadata = &apitypes.ClaudeMetadata{UserId: req.User}
	}
	if stopSequences := normalizeStopSequences(req.Stop); len(stopSequences) > 0 {
		dst.StopSequences = stopSequences
	}
	if req.ReasoningEffort != "" {
		dst.Thinking = convertReasoningEffortToThinking(req.ReasoningEffort, dst.Model)
	}

	maxTokens := req.MaxTokens
	if req.MaxCompletionTokens > maxTokens {
		maxTokens = req.MaxCompletionTokens
	}
	modelMaxTokens := claudeModelMaxTokens(dst.Model)
	if maxTokens <= 0 || maxTokens > modelMaxTokens {
		maxTokens = modelMaxTokens
	}
	dst.MaxTokens = &maxTokens

	switch dst.Model {
	case "claude-instant-1":
		dst.Model = "claude-instant-1.1"
	case "claude-2":
		dst.Model = "claude-2.1"
	}

	system, messages, err := convertOpenAIChatMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	if len(system) > 0 {
		dst.System = system
	}
	dst.Messages = messages
	return dst, nil
}

// buildClaudeToolsAndChoice requires a non-nil typed OpenAI chat request.
func buildClaudeToolsAndChoice(req *apitypes.OpenAIChatCompletionsRequest) ([]apitypes.ClaudeTool, *apitypes.ClaudeToolChoice) {
	tools := make([]apitypes.ClaudeTool, 0, len(req.Tools))
	for i := range req.Tools {
		tool := req.Tools[i]
		toolType := tool.Type
		if toolType == "" {
			toolType = "function"
		}
		if toolType == "web_search" {
			tools = append(tools, apitypes.ClaudeTool{
				Type: "web_search_20260209",
				Name: "web_search",
			})
			continue
		}
		if toolType != "function" || tool.Function == nil {
			continue
		}

		properties := map[string]any{}
		if rawProperties, ok := tool.Function.Parameters["properties"].(map[string]any); ok {
			properties = rawProperties
		}
		required := collectRequiredProperties(tool.Function.Parameters["required"])
		schemaType := "object"
		if rawType, ok := tool.Function.Parameters["type"].(string); ok && strings.TrimSpace(rawType) != "" {
			schemaType = rawType
		}
		tools = append(tools, apitypes.ClaudeTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: &apitypes.ClaudeInputSchema{
				Type:       schemaType,
				Properties: properties,
				Required:   required,
			},
		})
	}
	if len(tools) == 0 {
		return tools, nil
	}

	choice := &apitypes.ClaudeToolChoice{Type: "auto"}
	if req.ToolChoice == nil {
		return tools, choice
	}
	switch {
	case req.ToolChoice.Function != nil:
		choice.Type = "tool"
		choice.Name = req.ToolChoice.Function.Function.Name
	case req.ToolChoice.Mode != "":
		choice.Type = req.ToolChoice.Mode
		if choice.Type == "required" {
			choice.Type = "any"
		}
	}
	return tools, choice
}

func collectRequiredProperties(v any) []string {
	switch typed := v.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func convertOpenAIChatMessages(messages []apitypes.OpenAIChatMessage) ([]apitypes.ClaudeTextContent, []apitypes.ClaudeMessage, error) {
	system := make([]apitypes.ClaudeTextContent, 0)
	out := make([]apitypes.ClaudeMessage, 0, len(messages))

	for i := range messages {
		msg := messages[i]
		switch msg.Role {
		case "system", "developer":
			system = append(system, extractSystemBlocks(msg)...)
		case "tool":
			content, err := buildClaudeToolResults(msg)
			if err != nil {
				return nil, nil, err
			}
			if len(content) == 0 {
				continue
			}
			out = append(out, apitypes.ClaudeMessage{
				Role:    "user",
				Content: content,
			})
		case "function":
			content := buildClaudeTextBlocks(msg.Content)
			if len(content) == 0 {
				continue
			}
			out = append(out, apitypes.ClaudeMessage{
				Role:    "user",
				Content: content,
			})
		default:
			content, err := buildClaudeMessageContent(msg)
			if err != nil {
				return nil, nil, err
			}
			if len(content) == 0 {
				continue
			}
			out = append(out, apitypes.ClaudeMessage{
				Role:    msg.Role,
				Content: content,
			})
		}
	}

	return system, out, nil
}

func extractSystemBlocks(msg apitypes.OpenAIChatMessage) []apitypes.ClaudeTextContent {
	texts := buildClaudeTextBlocks(msg.Content)
	out := make([]apitypes.ClaudeTextContent, 0, len(texts))
	for _, block := range texts {
		textBlock, ok := block.(*apitypes.ClaudeTextContent)
		if !ok {
			continue
		}
		out = append(out, *textBlock)
	}
	return out
}

func buildClaudeMessageContent(msg apitypes.OpenAIChatMessage) ([]apitypes.ClaudeContent, error) {
	content := make([]apitypes.ClaudeContent, 0, len(msg.ToolCalls)+4)
	for i := range msg.ToolCalls {
		toolCall := msg.ToolCalls[i]
		if toolCall.Function == nil || toolCall.ID == "" || toolCall.Function.Name == "" {
			continue
		}
		input := map[string]any{}
		if strings.TrimSpace(toolCall.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
				return nil, err
			}
		}
		content = append(content, &apitypes.ClaudeToolUseContent{
			ClaudeBaseContent: apitypes.ClaudeBaseContent{Type: "tool_use"},
			Id:                toolCall.ID,
			Name:              toolCall.Function.Name,
			Input:             input,
		})
	}
	content = append(content, buildClaudeBlocks(msg.Content)...)
	return content, nil
}

func buildClaudeToolResults(msg apitypes.OpenAIChatMessage) ([]apitypes.ClaudeContent, error) {
	blocks := buildClaudeTextBlocks(msg.Content)
	if len(blocks) == 0 {
		return nil, nil
	}
	content := make([]apitypes.ClaudeContent, 0, len(blocks))
	for _, block := range blocks {
		textBlock, ok := block.(*apitypes.ClaudeTextContent)
		if !ok || textBlock.Text == "" {
			continue
		}
		content = append(content, &apitypes.ClaudeToolResultContent{
			ClaudeBaseContent: apitypes.ClaudeBaseContent{Type: "tool_result"},
			ToolUseId:         msg.ToolCallID,
			Content:           textBlock.Text,
		})
	}
	return content, nil
}

func buildClaudeTextBlocks(content *apitypes.OpenAIChatMessageContent) []apitypes.ClaudeContent {
	all := buildClaudeBlocks(content)
	out := make([]apitypes.ClaudeContent, 0, len(all))
	for _, block := range all {
		if textBlock, ok := block.(*apitypes.ClaudeTextContent); ok && textBlock.Text != "" {
			out = append(out, textBlock)
		}
	}
	return out
}

func buildClaudeBlocks(content *apitypes.OpenAIChatMessageContent) []apitypes.ClaudeContent {
	if content == nil {
		return nil
	}
	if content.Text != nil {
		if text := strings.TrimSpace(*content.Text); text != "" {
			return []apitypes.ClaudeContent{
				&apitypes.ClaudeTextContent{
					ClaudeBaseContent: apitypes.ClaudeBaseContent{Type: "text"},
					Text:              *content.Text,
				},
			}
		}
		return nil
	}

	out := make([]apitypes.ClaudeContent, 0, len(content.Parts))
	for i := range content.Parts {
		part := content.Parts[i]
		switch part.Type {
		case "", "text":
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			out = append(out, &apitypes.ClaudeTextContent{
				ClaudeBaseContent: apitypes.ClaudeBaseContent{Type: "text"},
				Text:              part.Text,
			})
		case "image_url":
			if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
				continue
			}
			out = append(out, &apitypes.ClaudeImageContent{
				ClaudeBaseContent: apitypes.ClaudeBaseContent{Type: "image"},
				Source: &apitypes.ClaudeURLSource{
					ClaudeBaseSource: apitypes.ClaudeBaseSource{Type: "url"},
					URL:              part.ImageURL.URL,
				},
			})
		default:
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			out = append(out, &apitypes.ClaudeTextContent{
				ClaudeBaseContent: apitypes.ClaudeBaseContent{Type: "text"},
				Text:              part.Text,
			})
		}
	}
	return out
}

func normalizeStopSequences(stop *apitypes.OpenAIChatStop) []string {
	if stop == nil {
		return nil
	}
	if strings.TrimSpace(stop.String) != "" {
		return []string{stop.String}
	}
	out := make([]string, 0, len(stop.List))
	for _, item := range stop.List {
		if s := strings.TrimSpace(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func convertReasoningEffortToThinking(reasoning string, model string) *apitypes.ThinkingConfig {
	normalized := strings.ToLower(strings.TrimSpace(reasoning))
	switch normalized {
	case "", "none", "minimal", "off", "disabled":
		return &apitypes.ThinkingConfig{
			Data: &apitypes.ThinkingConfigDisabled{
				BaseThinkingConfig: apitypes.BaseThinkingConfig{Type: "disabled"},
			},
		}
	}
	if supportsAdaptiveThinking(model) {
		return &apitypes.ThinkingConfig{
			Data: &apitypes.ThinkingConfigAdaptive{
				BaseThinkingConfig: apitypes.BaseThinkingConfig{Type: "adaptive"},
				Display:            "omitted",
			},
		}
	}
	if budget, ok := reasoningEffortBudgetMap[normalized]; ok {
		return &apitypes.ThinkingConfig{
			Data: &apitypes.ThinkingConfigEnabled{
				BaseThinkingConfig: apitypes.BaseThinkingConfig{Type: "enabled"},
				BudgetTokens:       budget,
				Display:            "omitted",
			},
		}
	}
	return &apitypes.ThinkingConfig{
		Data: &apitypes.ThinkingConfigDisabled{
			BaseThinkingConfig: apitypes.BaseThinkingConfig{Type: "disabled"},
		},
	}
}

func supportsAdaptiveThinking(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "claude-opus-4-6") || strings.HasPrefix(normalized, "claude-sonnet-4-6")
}

func claudeModelMaxTokens(model string) int {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return defaultAnthropicMaxTokens
	}
	if limit, ok := claudeModelMaxTokensExact[normalized]; ok {
		return limit
	}
	for prefix, limit := range claudeModelMaxTokensPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return limit
		}
	}
	return defaultAnthropicMaxTokens
}

const (
	reasoningEffortLow      = "low"
	reasoningEffortMedium   = "medium"
	reasoningEffortHigh     = "high"
	reasoningEffortNone     = "none"
	reasoningEffortOff      = "off"
	reasoningEffortDisabled = "disabled"
	roleAssistant           = "assistant"
	roleUser                = "user"
)

var mimeTypeMap = map[string]string{
	"json_object": "application/json",
	"text":        "text/plain",
}

var openAIToGeminiModalities = map[string]string{
	"text":  "TEXT",
	"image": "IMAGE",
	"audio": "AUDIO",
	"video": "VIDEO",
}

func mapOpenAIChatCompletionsToGeminiGenerateContentRequest(req *apitypes.OpenAIChatCompletionsRequest) *apitypes.GeminiGenerateContentRequest {
	if req == nil {
		return &apitypes.GeminiGenerateContentRequest{}
	}

	maxOutputTokens := req.MaxTokens
	if req.MaxCompletionTokens > 0 {
		maxOutputTokens = req.MaxCompletionTokens
	}
	isGemini3 := isGemini3Model(req.Model)
	dst := &apitypes.GeminiGenerateContentRequest{
		Contents: make([]apitypes.ChatContent, 0, len(req.Messages)),
		GenerationConfig: apitypes.ChatGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: maxOutputTokens,
		},
	}
	if req.ResponseFormat != nil {
		if mimeType, ok := mimeTypeMap[req.ResponseFormat.Type]; ok {
			dst.GenerationConfig.ResponseMimeType = mimeType
		}
		if req.ResponseFormat.JSONSchema != nil {
			dst.GenerationConfig.ResponseSchema = req.ResponseFormat.JSONSchema.Schema
			dst.GenerationConfig.ResponseMimeType = mimeTypeMap["json_object"]
		}
	}
	if len(req.Tools) > 0 {
		if functions := convertOpenAIChatToolsToGeminiFunctions(req.Tools); len(functions) > 0 {
			dst.Tools = []apitypes.ChatTools{{FunctionDeclarations: functions}}
		}
	} else if len(req.Functions) > 0 {
		dst.Tools = []apitypes.ChatTools{{FunctionDeclarations: req.Functions}}
	}
	if isGemini3 {
		if modalities := convertModalitiesToGemini(req.Modalities); len(modalities) > 0 {
			dst.GenerationConfig.ResponseModalities = modalities
		}
	}

	shouldAddDummyModelMessage := false
	for i := range req.Messages {
		msg := req.Messages[i]
		content := apitypes.ChatContent{
			Role:  msg.Role,
			Parts: buildGeminiPartsFromMessageContent(msg.Content, isGemini3),
		}
		if content.Role == roleAssistant {
			content.Role = "model"
		}
		if content.Role == "system" {
			shouldAddDummyModelMessage = true
			content.Role = roleUser
		}
		dst.Contents = append(dst.Contents, content)
		if shouldAddDummyModelMessage {
			dst.Contents = append(dst.Contents, apitypes.ChatContent{
				Role: "model",
				Parts: []apitypes.Part{
					{Text: "Okay"},
				},
			})
			shouldAddDummyModelMessage = false
		}
	}

	if strings.TrimSpace(req.ReasoningEffort) != "" {
		dst.GenerationConfig.ThinkingConfig = convertReasoningEffortToGeminiThinkingConfig(req.ReasoningEffort, req.Model)
	}

	return dst
}

func convertOpenAIChatToolsToGeminiFunctions(tools []apitypes.OpenAIChatTool) []apitypes.OpenAIFunctionDefinition {
	if len(tools) == 0 {
		return nil
	}
	functions := make([]apitypes.OpenAIFunctionDefinition, 0, len(tools))
	for i := range tools {
		tool := tools[i]
		if strings.TrimSpace(tool.Type) != "function" || tool.Function == nil {
			continue
		}
		functions = append(functions, *tool.Function)
	}
	return functions
}

func buildGeminiPartsFromMessageContent(content *apitypes.OpenAIChatMessageContent, isGemini3 bool) []apitypes.Part {
	if content == nil {
		return nil
	}
	if content.Text != nil {
		if strings.TrimSpace(*content.Text) == "" {
			return nil
		}
		return []apitypes.Part{{Text: *content.Text}}
	}

	parts := make([]apitypes.Part, 0, len(content.Parts))
	for i := range content.Parts {
		part := content.Parts[i]
		switch part.Type {
		case "", "text", "input_text":
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			parts = append(parts, apitypes.Part{Text: part.Text})
		case "image_url":
			if part.ImageURL == nil || strings.TrimSpace(part.ImageURL.URL) == "" {
				continue
			}
			inlineData, ok := dataURLToInlineData(part.ImageURL.URL)
			if !ok {
				continue
			}
			geminiPart := apitypes.Part{InlineData: inlineData}
			if isGemini3 {
				geminiPart.MediaResolution = convertDetailToMediaResolution(part.ImageURL.Detail)
			}
			parts = append(parts, geminiPart)
		}
	}
	return parts
}

func dataURLToInlineData(value string) (*apitypes.InlineData, bool) {
	if !strings.HasPrefix(value, "data:") {
		return nil, false
	}
	comma := strings.IndexByte(value, ',')
	if comma <= len("data:") {
		return nil, false
	}
	header := value[len("data:"):comma]
	data := value[comma+1:]
	mimeType := header
	if semi := strings.IndexByte(header, ';'); semi >= 0 {
		mimeType = header[:semi]
	}
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" || data == "" {
		return nil, false
	}
	return &apitypes.InlineData{
		MimeType: mimeType,
		Data:     data,
	}, true
}

func convertReasoningEffortToGeminiThinkingConfig(reasoningEffort string, model string) *apitypes.GeminiThinkingConfig {
	if isGemini3Model(model) {
		thinkingLevel := reasoningEffortMedium
		switch reasoningEffort {
		case reasoningEffortLow:
			thinkingLevel = reasoningEffortLow
		case reasoningEffortMedium:
			thinkingLevel = reasoningEffortMedium
		case reasoningEffortHigh:
			thinkingLevel = reasoningEffortHigh
		case reasoningEffortNone, reasoningEffortDisabled, reasoningEffortOff:
			thinkingLevel = reasoningEffortNone
		}
		return &apitypes.GeminiThinkingConfig{ThinkingLevel: thinkingLevel}
	}
	if isGemini25Model(model) {
		budget := 8192
		switch reasoningEffort {
		case reasoningEffortLow:
			budget = 1024
		case reasoningEffortMedium:
			budget = 8192
		case reasoningEffortHigh:
			budget = 24576
		case reasoningEffortNone, reasoningEffortDisabled, reasoningEffortOff:
			budget = 0
		}
		return &apitypes.GeminiThinkingConfig{ThinkingBudget: &budget}
	}
	return nil
}

func isGemini3Model(model string) bool {
	return strings.HasPrefix(model, "gemini-3")
}

func isGemini25Model(model string) bool {
	return strings.HasPrefix(model, "gemini-2.5")
}

var detailToMediaResolution = map[string]string{
	"low":        apitypes.MediaResolutionLow,
	"high":       apitypes.MediaResolutionHigh,
	"ultra":      apitypes.MediaResolutionUltraHigh,
	"ultra_high": apitypes.MediaResolutionUltraHigh,
	"auto":       apitypes.MediaResolutionMedium,
	"":           apitypes.MediaResolutionMedium,
}

func convertDetailToMediaResolution(detail string) string {
	normalized := strings.ToLower(strings.TrimSpace(detail))
	if resolution, ok := detailToMediaResolution[normalized]; ok {
		return resolution
	}
	return apitypes.MediaResolutionMedium
}

func convertModalitiesToGemini(modalities []string) []string {
	if len(modalities) == 0 {
		return nil
	}
	result := make([]string, 0, len(modalities))
	seen := make(map[string]struct{}, len(modalities))
	for _, modality := range modalities {
		normalized := strings.ToLower(strings.TrimSpace(modality))
		if normalized == "" {
			continue
		}
		if mapped, ok := openAIToGeminiModalities[normalized]; ok {
			normalized = mapped
		} else {
			normalized = strings.ToUpper(normalized)
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// mapClaudeRequestToOpenAIChatCompletions requires a non-nil typed Claude request.
func mapClaudeRequestToOpenAIChatCompletions(req *apitypes.ClaudeRequest) (*apitypes.OpenAIChatCompletionsRequest, error) {
	srcMap, err := req.ToMap()
	if err != nil {
		return nil, err
	}
	mappedObj, err := apitransform.MapClaudeMessagesToOpenAIChatCompletionsObject(apitypes.JSONObject(srcMap))
	if err != nil {
		return nil, err
	}
	var dst apitypes.OpenAIChatCompletionsRequest
	if err := dst.FromMap(mappedObj); err != nil {
		return nil, err
	}
	return &dst, nil
}

// mapGeminiGenerateContentRequestToOpenAIChatCompletions requires a non-nil typed Gemini request.
func mapGeminiGenerateContentRequestToOpenAIChatCompletions(req *apitypes.GeminiGenerateContentRequest) (*apitypes.OpenAIChatCompletionsRequest, error) {
	srcMap, err := req.ToMap()
	if err != nil {
		return nil, err
	}
	mappedObj, err := apitransform.MapGeminiGenerateContentToOpenAIChatCompletionsObject(apitypes.JSONObject(srcMap))
	if err != nil {
		return nil, err
	}
	var dst apitypes.OpenAIChatCompletionsRequest
	if err := dst.FromMap(mappedObj); err != nil {
		return nil, err
	}
	return &dst, nil
}

func parseReqMapInputObject(mode string, raw []byte) (apitypes.JSONObject, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "openai_chat_to_openai_responses", "openai_chat_to_anthropic_messages", "openai_chat_to_gemini_generate_content":
	case "anthropic_to_openai_chat":
	case "gemini_to_openai_chat":
	default:
		return nil, fmt.Errorf("unsupported req_map mode %q", mode)
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parse json object: %w", err)
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return nil, fmt.Errorf("json is not an object")
	}
	return apitypes.JSONObject(root), nil
}

func marshalBody(originalBody []byte, out map[string]any, contentType string) ([]byte, error) {
	if out == nil || requestcanon.IsMultipartFormData(contentType) {
		return originalBody, nil
	}
	return json.Marshal(out)
}
