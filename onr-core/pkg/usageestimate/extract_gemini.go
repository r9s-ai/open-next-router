package usageestimate

import (
	"sort"
	"strings"
)

func extractGeminiRequest(ectx *EstimateContext, bodyMap map[string]any) {
	ectx.AddOverHead(ItemPromptBase, 1)
	extractGeminiGenerationConfig(ectx, firstMapValue(bodyMap, "generationConfig", "generation_config"))
	toolMode := extractGeminiToolConfig(ectx, firstMapValue(bodyMap, "toolConfig", "tool_config"))

	if systemInstruction := firstMapValue(bodyMap, "system_instruction", "systemInstruction"); systemInstruction != nil {
		msg := extractGeminiContent(ectx, systemInstruction, "system")
		if len(msg.Content) != 0 || len(msg.ToolCalls) != 0 {
			msg.Role = "system"
			ectx.Messages = append(ectx.Messages, msg)
		}
	}

	if contents, ok := bodyMap["contents"].([]any); ok {
		for _, item := range contents {
			content, ok := item.(map[string]any)
			if !ok {
				continue
			}
			msg := extractGeminiContent(ectx, content, "user")
			if len(msg.Content) != 0 || len(msg.ToolCalls) != 0 {
				ectx.Messages = append(ectx.Messages, msg)
			}
		}
	}

	skipTools := toolMode == "NONE" && strings.Contains(ectx.Model, "gemini-3-flash-preview")
	if tools, ok := bodyMap["tools"].([]any); ok && !skipTools {
		extracted := extractGeminiTools(ectx, tools)
		ectx.Tools = append(ectx.Tools, extracted...)
		addToolDefinitionOverheads(ectx, extracted)
	}
}

func extractGeminiResponse(ectx *EstimateContext, bodyMap map[string]any) {
	if candidates, ok := bodyMap["candidates"].([]any); ok {
		for _, item := range candidates {
			candidate, ok := item.(map[string]any)
			if !ok {
				continue
			}
			content, ok := candidate["content"].(map[string]any)
			if !ok {
				continue
			}
			msg := extractGeminiContent(ectx, content, "model")
			if len(msg.Content) != 0 || len(msg.ToolCalls) != 0 {
				ectx.Messages = append(ectx.Messages, msg)
			}
		}
	}
}

func extractGeminiGenerationConfig(ectx *EstimateContext, cfg map[string]any) {
	if cfg == nil {
		return
	}
	if v, ok := intFromAny(cfg["maxOutputTokens"]); ok {
		ectx.MaxTokens = v
	}
	if thinkingConfig, ok := cfg["thinkingConfig"].(map[string]any); ok {
		if v, ok := intFromAny(thinkingConfig["thinkingBudget"]); ok {
			ectx.MaxThinkingTokens = v
			ectx.AddOverHead(ItemThinkingBlock, 1)
		}
		if includeThoughts, ok := thinkingConfig["includeThoughts"].(bool); ok && includeThoughts {
			ectx.AddOverHead(ItemHiddenReasoningBlock, 1)
		}
	}
	if schema, ok := cfg["responseSchema"].(map[string]any); ok {
		ectx.AddOverHead(ItemResponseFormatJsonSchema, 1)
		ectx.AddOverHead(ItemResponseFormatJsonSchemaStringPropertyRequired, countRequiredStringProperties(schema))
		extracted := extracOpenAPISchema(ectx, schema)
		appendGeminiSchemaTexts(ectx, &extracted)
	}
}

func extractGeminiToolConfig(ectx *EstimateContext, cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	functionCallingConfig, ok := firstAnyValue(cfg, "functionCallingConfig", "function_calling_config").(map[string]any)
	if !ok {
		return ""
	}
	mode := strings.ToUpper(strings.TrimSpace(stringFromAny(functionCallingConfig["mode"])))
	switch mode {
	case "NONE":
		ectx.AddOverHead(ItemToolChoiceNone, 1)
	case "AUTO":
		ectx.AddOverHead(ItemToolChoiceAuto, 1)
	case "ANY":
		ectx.AddOverHead(ItemToolChoiceAny, 1)
	}
	return mode
}

func extractGeminiContent(ectx *EstimateContext, content map[string]any, defaultRole string) EstimateMessage {
	role, _ := content["role"].(string)
	if defaultRole == "system" || role == "" {
		role = defaultRole
	}
	msg := EstimateMessage{Role: normalizeGeminiRole(role)}

	if parts, ok := content["parts"].([]any); ok {
		for _, item := range parts {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			extractGeminiPart(ectx, &msg, part)
		}
	}
	if msg.Role != "" && (len(msg.Content) != 0 || len(msg.ToolCalls) != 0) {
		addOpenAIChatRoleOverhead(ectx, msg.Role)
	}
	return msg
}

func extractGeminiPart(ectx *EstimateContext, msg *EstimateMessage, part map[string]any) {
	if text, ok := part["text"].(string); ok {
		contentType := "text"
		if thought, ok := part["thought"].(bool); ok && thought {
			contentType = "thinking"
			ectx.AddOverHead(ItemThinkingBlock, 1)
		}
		msg.Content = append(msg.Content, EstimateMessagesContent{Type: contentType, Text: text})
	}
	if signature, ok := part["thoughtSignature"].(string); ok && signature != "" {
		ectx.AddOverHead(ItemHiddenReasoningBlock, 1)
	}
	if functionCall, ok := part["functionCall"].(map[string]any); ok {
		msg.ToolCalls = append(msg.ToolCalls, extractGeminiFunctionCall(ectx, functionCall))
	}
	if functionResponse, ok := part["functionResponse"].(map[string]any); ok {
		msg.Role = "tool"
		msg.Name, _ = functionResponse["name"].(string)
		ectx.AddOverHead(ItemFunctionCallResult, 1)
		msg.Content = append(msg.Content, EstimateMessagesContent{Type: "tool_result", Text: flattenGeminiValue(functionResponse["response"])})
	}
	if inlineData, ok := part["inlineData"].(map[string]any); ok {
		addGeminiMediaOverhead(ectx, inlineData["mimeType"])
	}
	if fileData, ok := part["fileData"].(map[string]any); ok {
		addGeminiMediaOverhead(ectx, fileData["mimeType"])
	}
}

func extractGeminiFunctionCall(ectx *EstimateContext, functionCall map[string]any) EstimateToolCall {
	out := EstimateToolCall{Type: "function"}
	if name, ok := functionCall["name"].(string); ok {
		out.Name = name
	}
	if args := functionCall["args"]; args != nil {
		out.Arguments = args
	}
	ectx.AddOverHead(ItemFunctionCall, 1)
	return out
}

func extractGeminiTools(ectx *EstimateContext, tools []any) []EstimateTool {
	var out []EstimateTool
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if declarations, ok := firstAnyValue(tool, "function_declarations", "functionDeclarations").([]any); ok {
			for _, rawDeclaration := range declarations {
				declaration, ok := rawDeclaration.(map[string]any)
				if !ok {
					continue
				}
				out = append(out, extractGeminiFunctionDeclaration(ectx, declaration))
			}
			continue
		}
		if toolName := firstNonEmptyString(tool, "googleSearch", "google_search", "codeExecution", "code_execution"); toolName != "" {
			out = append(out, EstimateTool{Type: toolName, Name: toolName})
		}
	}
	return out
}

func extractGeminiFunctionDeclaration(ectx *EstimateContext, declaration map[string]any) EstimateTool {
	tool := EstimateTool{Type: "function"}
	if name, ok := declaration["name"].(string); ok {
		tool.Name = name
	}
	if description, ok := declaration["description"].(string); ok {
		tool.Description = description
		ectx.AddOverHead(ItemToolDescription, 1)
	}
	if parameters, ok := declaration["parameters"].(map[string]any); ok {
		tool.Parameters = extracOpenAPISchema(ectx, parameters)
	}
	return tool
}

func normalizeGeminiRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "model":
		return "assistant"
	case "function", "tool":
		return "tool"
	case "system":
		return "system"
	case "user":
		return "user"
	default:
		return role
	}
}

func addGeminiMediaOverhead(ectx *EstimateContext, rawMime any) {
	mime := strings.ToLower(strings.TrimSpace(stringFromAny(rawMime)))
	switch {
	case strings.HasPrefix(mime, "image/"):
		ectx.AddOverHead(ItemImageBlock, 1)
	case strings.HasPrefix(mime, "application/pdf"), strings.HasPrefix(mime, "text/"):
		ectx.AddOverHead(ItemDocumentBlock, 1)
	default:
		ectx.AddOverHead(ItemUnknownBlcok, 1)
	}
}

func firstMapValue(root map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := root[key].(map[string]any); ok {
			return value
		}
	}
	return nil
}

func firstAnyValue(root map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := root[key]; ok {
			return value
		}
	}
	return nil
}

func firstNonEmptyString(root map[string]any, keys ...string) string {
	for _, key := range keys {
		if _, ok := root[key]; ok {
			return key
		}
	}
	return ""
}

func stringFromAny(v any) string {
	if text, ok := v.(string); ok {
		return text
	}
	return ""
}

func appendGeminiSchemaTexts(ectx *EstimateContext, schema *ToolSchema) {
	if ectx == nil || schema == nil {
		return
	}
	if schema.Description != "" {
		ectx.Texts = append(ectx.Texts, EstimateText{Kind: "gemini_schema", Text: schema.Description})
	}
	for _, text := range schema.Required {
		ectx.Texts = append(ectx.Texts, EstimateText{Kind: "gemini_schema", Text: text})
	}
	for _, item := range schema.Enum {
		if text, ok := item.(string); ok {
			ectx.Texts = append(ectx.Texts, EstimateText{Kind: "gemini_schema", Text: text})
		}
	}
	if schema.Items != nil {
		appendGeminiSchemaTexts(ectx, schema.Items)
	}
	for name, child := range schema.Properties {
		ectx.Texts = append(ectx.Texts, EstimateText{Kind: "gemini_schema", Text: name})
		appendGeminiSchemaTexts(ectx, child)
	}
	for _, child := range schema.AnyOf {
		appendGeminiSchemaTexts(ectx, child)
	}
	for _, child := range schema.OneOf {
		appendGeminiSchemaTexts(ectx, child)
	}
	for _, child := range schema.AllOf {
		appendGeminiSchemaTexts(ectx, child)
	}
}

func flattenGeminiValue(value any) string {
	var parts []string
	appendFlattenedGeminiValue(&parts, value)
	return strings.Join(parts, " ")
}

func appendFlattenedGeminiValue(parts *[]string, value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		*parts = append(*parts, v)
	case bool:
		if v {
			*parts = append(*parts, "true")
		} else {
			*parts = append(*parts, "false")
		}
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		*parts = append(*parts, stringifyOpenAIValue(v))
	case []any:
		for _, item := range v {
			appendFlattenedGeminiValue(parts, item)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			*parts = append(*parts, key)
			appendFlattenedGeminiValue(parts, v[key])
		}
	default:
		*parts = append(*parts, stringifyOpenAIValue(v))
	}
}
