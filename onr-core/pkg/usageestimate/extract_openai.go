package usageestimate

import (
	"encoding/json"
	"strings"
)

func extractOpenAIChatRequest(ectx *EstimateContext, bodyMap map[string]any) {
	ectx.AddOverHead(ItemPromptBase, 1)
	if v, ok := intFromAny(bodyMap["max_completion_tokens"]); ok {
		ectx.MaxTokens = v
	} else if v, ok := intFromAny(bodyMap["max_tokens"]); ok {
		ectx.MaxTokens = v
	}
	extractOpenAIChatToolChoice(ectx, bodyMap["tool_choice"])

	if messages, ok := bodyMap["messages"].([]any); ok {
		for _, item := range messages {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			msg := extractOpenAIChatMessage(ectx, m)
			ectx.Messages = append(ectx.Messages, msg)
		}
	}

	if tools, ok := bodyMap["tools"].([]any); ok {
		extracted := extractOpenAIChatTools(ectx, tools)
		ectx.Tools = append(ectx.Tools, extracted...)
		addToolDefinitionOverheads(ectx, extracted)
	}
}

func extractOpenAIChatResponse(ectx *EstimateContext, bodyMap map[string]any) {
	if choices, ok := bodyMap["choices"].([]any); ok {
		for _, item := range choices {
			choice, ok := item.(map[string]any)
			if !ok {
				continue
			}
			message, ok := choice["message"].(map[string]any)
			if !ok {
				continue
			}
			msg := extractOpenAIChatMessage(ectx, message)
			if msg.Role == "" {
				msg.Role = "assistant"
				ectx.AddOverHead(ItemRoleAssistant, 1)
			}
			ectx.Messages = append(ectx.Messages, msg)
		}
	}
}

func extractOpenAIResponsesRequest(ectx *EstimateContext, bodyMap map[string]any) {
	ectx.AddOverHead(ItemPromptBase, 1)
	if v, ok := intFromAny(bodyMap["max_output_tokens"]); ok {
		ectx.MaxTokens = v
	}
	extractOpenAIResponsesTextFormat(ectx, bodyMap["text"])
	extractOpenAIResponsesToolChoice(ectx, bodyMap["tool_choice"])

	if instructions, ok := bodyMap["instructions"].(string); ok && instructions != "" {
		ectx.AddOverHead(ItemRoleSystem, 1)
		ectx.Messages = append(ectx.Messages, EstimateMessage{
			Role:    "system",
			Content: []EstimateMessagesContent{{Type: "text", Text: instructions}},
		})
	}

	extractOpenAIResponsesInput(ectx, bodyMap["input"])

	if tools, ok := bodyMap["tools"].([]any); ok {
		extracted := extractOpenAIResponsesTools(ectx, tools)
		ectx.Tools = append(ectx.Tools, extracted...)
		addToolDefinitionOverheads(ectx, extracted)
	}
}

func extractOpenAIResponsesTextFormat(ectx *EstimateContext, raw any) {
	text, ok := raw.(map[string]any)
	if !ok {
		return
	}
	format, ok := text["format"].(map[string]any)
	if !ok {
		return
	}
	formatType, _ := format["type"].(string)
	if formatType != "json_schema" {
		return
	}
	ectx.AddOverHead(ItemResponseFormatJsonSchema, 1)
	if schema, ok := format["schema"].(map[string]any); ok {
		ectx.AddOverHead(ItemResponseFormatJsonSchemaStringPropertyRequired, countRequiredStringProperties(schema))
	}
}

func extractOpenAIResponsesResponse(ectx *EstimateContext, bodyMap map[string]any) {
	extractedText := false
	if output, ok := bodyMap["output"].([]any); ok {
		for _, item := range output {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if extractOpenAIResponsesOutputItem(ectx, m) {
				extractedText = true
			}
		}
	}
	if !extractedText {
		if outputText, ok := bodyMap["output_text"].(string); ok && outputText != "" {
			ectx.AddOverHead(ItemRoleAssistant, 1)
			ectx.Messages = append(ectx.Messages, EstimateMessage{
				Role:    "assistant",
				Content: []EstimateMessagesContent{{Type: "text", Text: outputText}},
			})
		}
	}
}

func extractOpenAIResponsesInput(ectx *EstimateContext, input any) {
	switch v := input.(type) {
	case string:
		ectx.AddOverHead(ItemRoleUser, 1)
		ectx.Messages = append(ectx.Messages, EstimateMessage{
			Role:    "user",
			Content: []EstimateMessagesContent{{Type: "text", Text: v}},
		})
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			extractOpenAIResponsesInputItem(ectx, m)
		}
	}
}

func extractOpenAIResponsesInputItem(ectx *EstimateContext, item map[string]any) {
	itemType, _ := item["type"].(string)
	switch itemType {
	case "function_call":
		msg := EstimateMessage{Role: "assistant"}
		ectx.AddOverHead(ItemRoleAssistant, 1)
		msg.ToolCalls = append(msg.ToolCalls, extractOpenAIResponsesFunctionCall(ectx, item))
		ectx.Messages = append(ectx.Messages, msg)
	case "custom_tool_call":
		msg := EstimateMessage{Role: "assistant"}
		ectx.AddOverHead(ItemRoleAssistant, 1)
		msg.ToolCalls = append(msg.ToolCalls, extractOpenAIResponsesCustomToolCall(ectx, item))
		ectx.Messages = append(ectx.Messages, msg)
	case "function_call_output":
		msg := EstimateMessage{Role: "tool"}
		ectx.AddOverHead(ItemRoleTool, 1)
		ectx.AddOverHead(ItemFunctionCallResult, 1)
		msg.Content = append(msg.Content, EstimateMessagesContent{Type: "text", Text: stringifyOpenAIValue(item["output"])})
		ectx.Messages = append(ectx.Messages, msg)
	case "custom_tool_call_output":
		msg := EstimateMessage{Role: "tool"}
		ectx.AddOverHead(ItemRoleTool, 1)
		ectx.AddOverHead(ItemCustomToolCallOutput, 1)
		msg.Content = append(msg.Content, EstimateMessagesContent{Type: "text", Text: stringifyOpenAIValue(item["output"])})
		ectx.Messages = append(ectx.Messages, msg)
	case "reasoning":
		msg := extractOpenAIResponsesReasoningItem(ectx, item)
		if len(msg.Content) != 0 {
			ectx.Messages = append(ectx.Messages, msg)
		}
	default:
		if _, ok := item["role"].(string); ok {
			ectx.Messages = append(ectx.Messages, extractOpenAIResponsesMessageItem(ectx, item, false))
		} else if itemType != "" {
			ectx.AddOverHead(ItemUnknownBlcok, 1)
		}
	}
}

func extractOpenAIResponsesOutputItem(ectx *EstimateContext, item map[string]any) bool {
	itemType, _ := item["type"].(string)
	switch itemType {
	case "message":
		msg := extractOpenAIResponsesMessageItem(ectx, item, true)
		ectx.Messages = append(ectx.Messages, msg)
		return messageHasText(msg)
	case "function_call":
		msg := EstimateMessage{Role: "assistant"}
		ectx.AddOverHead(ItemRoleAssistant, 1)
		msg.ToolCalls = append(msg.ToolCalls, extractOpenAIResponsesFunctionCall(ectx, item))
		ectx.Messages = append(ectx.Messages, msg)
	case "custom_tool_call":
		msg := EstimateMessage{Role: "assistant"}
		ectx.AddOverHead(ItemRoleAssistant, 1)
		msg.ToolCalls = append(msg.ToolCalls, extractOpenAIResponsesCustomToolCall(ectx, item))
		ectx.Messages = append(ectx.Messages, msg)
	case "custom_tool_call_output":
		msg := EstimateMessage{Role: "tool"}
		ectx.AddOverHead(ItemRoleTool, 1)
		ectx.AddOverHead(ItemCustomToolCallOutput, 1)
		msg.Content = append(msg.Content, EstimateMessagesContent{Type: "text", Text: stringifyOpenAIValue(item["output"])})
		ectx.Messages = append(ectx.Messages, msg)
	case "reasoning":
		msg := extractOpenAIResponsesReasoningItem(ectx, item)
		if len(msg.Content) != 0 {
			ectx.Messages = append(ectx.Messages, msg)
			return true
		}
	default:
		if itemType != "" {
			ectx.AddOverHead(ItemUnknownBlcok, 1)
		}
	}
	return false
}

func extractOpenAIResponsesMessageItem(ectx *EstimateContext, item map[string]any, defaultAssistant bool) EstimateMessage {
	role, _ := item["role"].(string)
	if role == "" && defaultAssistant {
		role = "assistant"
	}
	msg := EstimateMessage{Role: role}
	if role != "" {
		addOpenAIChatRoleOverhead(ectx, role)
	}
	msg.Content = extractOpenAIChatContent(ectx, item["content"])
	return msg
}

func extractOpenAIResponsesFunctionCall(ectx *EstimateContext, item map[string]any) EstimateToolCall {
	call := EstimateToolCall{Type: "function"}
	if name, ok := item["name"].(string); ok {
		call.Name = name
	}
	if arguments, ok := item["arguments"].(string); ok {
		call.Arguments = arguments
	} else if arguments := item["arguments"]; arguments != nil {
		call.Arguments = arguments
	}
	ectx.AddOverHead(ItemFunctionCall, 1)
	return call
}

func extractOpenAIResponsesCustomToolCall(ectx *EstimateContext, item map[string]any) EstimateToolCall {
	call := EstimateToolCall{Type: "custom"}
	if name, ok := item["name"].(string); ok {
		call.Name = name
	}
	if input, ok := item["input"].(string); ok {
		call.Arguments = input
	} else if input := item["input"]; input != nil {
		call.Arguments = input
	}
	ectx.AddOverHead(ItemCustomToolCall, 1)
	return call
}

func extractOpenAIResponsesReasoningItem(ectx *EstimateContext, item map[string]any) EstimateMessage {
	msg := EstimateMessage{Role: "assistant"}
	if summaries, ok := item["summary"].([]any); ok {
		for _, summary := range summaries {
			m, ok := summary.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				msg.Content = append(msg.Content, EstimateMessagesContent{Type: "thinking", Text: text})
			}
		}
	}
	_, hasEncryptedContent := item["encrypted_content"].(string)
	if len(msg.Content) != 0 || hasEncryptedContent {
		ectx.AddOverHead(ItemRoleAssistant, 1)
		ectx.AddOverHead(ItemHiddenReasoningBlock, 1)
	}
	return msg
}

func messageHasText(msg EstimateMessage) bool {
	for _, content := range msg.Content {
		if content.Text != "" {
			return true
		}
	}
	return false
}

func extractOpenAIResponsesTools(ectx *EstimateContext, tools []any) []EstimateTool {
	var out []EstimateTool
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		toolType, _ := tool["type"].(string)
		t := EstimateTool{Type: toolType}
		if description, ok := tool["description"].(string); ok {
			t.Description = description
			ectx.AddOverHead(ItemToolDescription, 1)
		}
		if parameters, ok := tool["parameters"].(map[string]any); ok {
			t.Parameters = extracOpenAPISchema(ectx, parameters)
		}
		switch toolType {
		case "function":
			if name, ok := tool["name"].(string); ok {
				t.Name = name
			}
		case "custom":
			if name, ok := tool["name"].(string); ok {
				t.Name = name
			}
			if format, ok := tool["format"].(map[string]any); ok {
				if definition, ok := format["definition"].(string); ok {
					t.Definition = definition
				}
			}
		default:
			t.Name = toolType
		}
		if t.Type != "" || t.Name != "" || t.Description != "" || t.Definition != "" {
			out = append(out, t)
		}
	}
	return out
}

func extractOpenAIResponsesToolChoice(ectx *EstimateContext, choice any) {
	switch v := choice.(type) {
	case string:
		switch v {
		case "none":
			ectx.AddOverHead(ItemToolChoiceNone, 1)
		case "auto":
			ectx.AddOverHead(ItemToolChoiceAuto, 1)
		case "required":
			ectx.AddOverHead(ItemToolChoiceAny, 1)
		}
		if v != "" {
			ectx.ToolChoice = map[string]string{"type": v}
		}
	case map[string]any:
		choiceType, _ := v["type"].(string)
		out := map[string]string{"type": choiceType}
		if name, ok := v["name"].(string); ok {
			out["name"] = name
		}
		ectx.ToolChoice = out
		switch choiceType {
		case "none":
			ectx.AddOverHead(ItemToolChoiceNone, 1)
		case "auto":
			ectx.AddOverHead(ItemToolChoiceAuto, 1)
		case "required", "allowed_tools":
			ectx.AddOverHead(ItemToolChoiceAny, 1)
		default:
			ectx.AddOverHead(ItemToolChoiceToolName, 1)
		}
	}
}

func stringifyOpenAIValue(v any) string {
	if text, ok := v.(string); ok {
		return text
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func extractEmbeddingRequest(ectx *EstimateContext, bodyMap map[string]any) *EstimateContext {
	return ectx
}

func extractEmbeddingResponse(ectx *EstimateContext, bodyMap map[string]any) *EstimateContext {
	return ectx
}

func extractOpenAIChatMessage(ectx *EstimateContext, m map[string]any) EstimateMessage {
	msg := EstimateMessage{}
	if role, ok := m["role"].(string); ok {
		msg.Role = role
		addOpenAIChatRoleOverhead(ectx, role)
	}
	if name, ok := m["name"].(string); ok {
		msg.Name = name
	}
	msg.Content = extractOpenAIChatContent(ectx, m["content"])
	if refusal, ok := m["refusal"].(string); ok {
		msg.Content = append(msg.Content, EstimateMessagesContent{Type: "refusal", Text: refusal})
	}

	if toolCalls, ok := m["tool_calls"].([]any); ok {
		msg.ToolCalls = append(msg.ToolCalls, extractOpenAIChatToolCalls(ectx, toolCalls)...)
	}
	if functionCall, ok := m["function_call"].(map[string]any); ok {
		msg.ToolCalls = append(msg.ToolCalls, extractOpenAIChatFunctionCall(ectx, functionCall))
	}
	if toolCallID, ok := m["tool_call_id"].(string); ok {
		msg.ToolCallID = toolCallID
	}
	if msg.Role == "tool" || msg.Role == "function" {
		ectx.AddOverHead(ItemFunctionCallResult, 1)
	}
	return msg
}

func addOpenAIChatRoleOverhead(ectx *EstimateContext, role string) {
	switch role {
	case "user":
		ectx.AddOverHead(ItemRoleUser, 1)
	case "assistant":
		ectx.AddOverHead(ItemRoleAssistant, 1)
	case "system", "developer":
		ectx.AddOverHead(ItemRoleSystem, 1)
	case "tool", "function":
		ectx.AddOverHead(ItemRoleTool, 1)
	}
}

func extractOpenAIChatContent(ectx *EstimateContext, content any) []EstimateMessagesContent {
	switch v := content.(type) {
	case string:
		return []EstimateMessagesContent{{Type: "text", Text: v}}
	case []any:
		var out []EstimateMessagesContent
		for _, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text", "input_text", "output_text":
				if text, ok := block["text"].(string); ok {
					out = append(out, EstimateMessagesContent{Type: "text", Text: text})
				}
			case "refusal":
				if text, ok := block["refusal"].(string); ok {
					out = append(out, EstimateMessagesContent{Type: "refusal", Text: text})
				}
			case "image_url", "input_image":
				out = append(out, EstimateMessagesContent{Type: "image"})
				ectx.AddOverHead(ItemImageBlock, 1)
			case "input_audio", "audio", "file", "input_file":
				out = append(out, EstimateMessagesContent{Type: blockType})
				ectx.AddOverHead(ItemUnknownBlcok, 1)
			default:
				if blockType != "" {
					ectx.AddOverHead(ItemUnknownBlcok, 1)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func extractOpenAIChatToolCalls(ectx *EstimateContext, toolCalls []any) []EstimateToolCall {
	var out []EstimateToolCall
	for _, item := range toolCalls {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, extractOpenAIChatToolCall(ectx, call))
	}
	return out
}

func extractOpenAIChatToolCall(ectx *EstimateContext, call map[string]any) EstimateToolCall {
	out := EstimateToolCall{Type: "function"}
	if callType, ok := call["type"].(string); ok {
		out.Type = callType
	}
	if function, ok := call["function"].(map[string]any); ok {
		if name, ok := function["name"].(string); ok {
			out.Name = name
		}
		if arguments, ok := function["arguments"].(string); ok {
			out.Arguments = arguments
		} else if arguments := function["arguments"]; arguments != nil {
			out.Arguments = arguments
		}
	} else if custom, ok := call["custom"].(map[string]any); ok {
		out.Type = "custom"
		if name, ok := custom["name"].(string); ok {
			out.Name = name
		}
		if input := custom["input"]; input != nil {
			out.Arguments = input
		}
	}
	ectx.AddOverHead(ItemFunctionCall, 1)
	return out
}

func extractOpenAIChatFunctionCall(ectx *EstimateContext, functionCall map[string]any) EstimateToolCall {
	out := EstimateToolCall{Type: "function"}
	if name, ok := functionCall["name"].(string); ok {
		out.Name = name
	}
	if arguments, ok := functionCall["arguments"].(string); ok {
		out.Arguments = arguments
	} else if arguments := functionCall["arguments"]; arguments != nil {
		out.Arguments = arguments
	}
	ectx.AddOverHead(ItemFunctionCall, 1)
	return out
}

func extractOpenAIChatTools(ectx *EstimateContext, tools []any) []EstimateTool {
	var out []EstimateTool
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		toolType, _ := tool["type"].(string)
		t := EstimateTool{Type: toolType}
		switch toolType {
		case "function", "":
			if function, ok := tool["function"].(map[string]any); ok {
				t.Type = "function"
				if name, ok := function["name"].(string); ok {
					t.Name = name
				}
				if description, ok := function["description"].(string); ok {
					t.Description = description
					ectx.AddOverHead(ItemToolDescription, 1)
				}
				if parameters, ok := function["parameters"].(map[string]any); ok {
					t.Parameters = extracOpenAPISchema(ectx, parameters)
				}
			}
		case "custom":
			if custom, ok := tool["custom"].(map[string]any); ok {
				if name, ok := custom["name"].(string); ok {
					t.Name = name
				}
				if description, ok := custom["description"].(string); ok {
					t.Description = description
					ectx.AddOverHead(ItemToolDescription, 1)
				}
			}
		}
		if t.Type != "" || t.Name != "" || t.Description != "" {
			out = append(out, t)
		}
	}
	return out
}

func extractOpenAIChatToolChoice(ectx *EstimateContext, choice any) {
	switch v := choice.(type) {
	case string:
		switch v {
		case "none":
			ectx.AddOverHead(ItemToolChoiceNone, 1)
		case "auto":
			ectx.AddOverHead(ItemToolChoiceAuto, 1)
		case "required":
			ectx.AddOverHead(ItemToolChoiceAny, 1)
		}
		if v != "" {
			ectx.ToolChoice = map[string]string{"type": v}
		}
	case map[string]any:
		choiceType, _ := v["type"].(string)
		out := map[string]string{"type": choiceType}
		if function, ok := v["function"].(map[string]any); ok {
			if name, ok := function["name"].(string); ok {
				out["name"] = name
			}
		}
		if custom, ok := v["custom"].(map[string]any); ok {
			if name, ok := custom["name"].(string); ok {
				out["name"] = name
			}
		}
		ectx.ToolChoice = out
		switch choiceType {
		case "none":
			ectx.AddOverHead(ItemToolChoiceNone, 1)
		case "auto":
			ectx.AddOverHead(ItemToolChoiceAuto, 1)
		case "required", "allowed_tools":
			ectx.AddOverHead(ItemToolChoiceAny, 1)
		default:
			ectx.AddOverHead(ItemToolChoiceToolName, 1)
		}
	}
}

func extracOpenAPISchema(ectx *EstimateContext, toolSchema map[string]any) ToolSchema {
	if toolSchema == nil {
		return ToolSchema{}
	}

	data, err := json.Marshal(toolSchema)
	if err != nil {
		return ToolSchema{}
	}
	var schema ToolSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return ToolSchema{}
	}
	addOpenAPISchemaOverheads(ectx, &schema)
	return schema
}

func addOpenAPISchemaOverheads(ectx *EstimateContext, schema *ToolSchema) {
	if ectx == nil || schema == nil {
		return
	}

	if schema.Description != "" {
		ectx.AddOverHead(ItemToolPropertiesDescription, 1)
	}
	addOpenAPISchemaTypeOverhead(ectx, schema.Type, schema.Items)

	if len(schema.Required) != 0 {
		ectx.AddOverHead(ItemToolRequired, 1)
		ectx.AddOverHead(ItemToolRequiredItem, len(schema.Required))
	}

	if schema.AdditionalProperties != nil {
		switch v := schema.AdditionalProperties.(type) {
		case bool:
			ectx.AddOverHead(ItemTooladditionalPropertiesBool, 1)
		case map[string]any:
			if typeName, ok := v["type"].(string); ok {
				addOpenAPISchemaAdditionalPropertiesTypeOverhead(ectx, typeName)
			} else {
				ectx.AddOverHead(ItemTooladditionalPropertiesTypeUnknown, 1)
			}
		default:
			ectx.AddOverHead(ItemTooladditionalPropertiesTypeUnknown, 1)
		}
	}

	if len(schema.Enum) != 0 {
		ectx.AddOverHead(ItemToolEnum, 1)
		ectx.AddOverHead(ItemToolEnumItem, len(schema.Enum))
	}

	if schema.Items != nil {
		addOpenAPISchemaOverheads(ectx, schema.Items)
	}
	for _, v := range schema.Properties {
		addOpenAPISchemaOverheads(ectx, v)
	}
	for _, v := range schema.AnyOf {
		addOpenAPISchemaOverheads(ectx, v)
	}
	for _, v := range schema.OneOf {
		addOpenAPISchemaOverheads(ectx, v)
	}
	for _, v := range schema.AllOf {
		addOpenAPISchemaOverheads(ectx, v)
	}
}

func addOpenAPISchemaTypeOverhead(ectx *EstimateContext, schemaType any, items *ToolSchema) {
	switch v := schemaType.(type) {
	case string:
		addOpenAPISchemaSingleTypeOverhead(ectx, v, items)
	case []any:
		for _, item := range v {
			if text, ok := item.(string); ok {
				addOpenAPISchemaSingleTypeOverhead(ectx, text, items)
			}
		}
	case []string:
		for _, text := range v {
			addOpenAPISchemaSingleTypeOverhead(ectx, text, items)
		}
	}
}

func addOpenAPISchemaSingleTypeOverhead(ectx *EstimateContext, schemaType string, items *ToolSchema) {
	switch strings.ToLower(strings.TrimSpace(schemaType)) {
	case "string":
		ectx.AddOverHead(ItemToolPropertiesTypeString, 1)
	case "array":
		if items != nil && items.Type == "string" {
			ectx.AddOverHead(ItemToolPropertiesTypeArrayOfString, 1)
		} else {
			ectx.AddOverHead(ItemToolPropertiesTypeArray, 1)
		}
	case "object":
		ectx.AddOverHead(ItemToolPropertiesTypeObject, 1)
	case "number":
		ectx.AddOverHead(ItemToolPropertiesTypeNumber, 1)
	case "integer":
		ectx.AddOverHead(ItemToolPropertiesTypeInt, 1)
	case "boolean":
		ectx.AddOverHead(ItemToolPropertiesTypeBool, 1)
	}
}

func addOpenAPISchemaAdditionalPropertiesTypeOverhead(ectx *EstimateContext, schemaType string) {
	switch schemaType {
	case "string":
		ectx.AddOverHead(ItemTooladditionalPropertiesTypeString, 1)
	default:
		ectx.AddOverHead(ItemTooladditionalPropertiesTypeUnknown, 1)
	}
}

func countRequiredStringProperties(schema map[string]any) int {
	required := map[string]bool{}
	if rawRequired, ok := schema["required"].([]any); ok {
		for _, item := range rawRequired {
			if text, ok := item.(string); ok {
				required[text] = true
			}
		}
	}

	total := 0
	if properties, ok := schema["properties"].(map[string]any); ok {
		for name, rawProperty := range properties {
			property, ok := rawProperty.(map[string]any)
			if !ok {
				continue
			}
			if required[name] && schemaHasStringType(property["type"]) {
				total++
			}
			total += countRequiredStringProperties(property)
		}
	}
	for _, key := range []string{"items"} {
		if nested, ok := schema[key].(map[string]any); ok {
			total += countRequiredStringProperties(nested)
		}
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if variants, ok := schema[key].([]any); ok {
			for _, rawVariant := range variants {
				if variant, ok := rawVariant.(map[string]any); ok {
					total += countRequiredStringProperties(variant)
				}
			}
		}
	}
	return total
}

func schemaHasStringType(raw any) bool {
	switch v := raw.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "string")
	case []any:
		for _, item := range v {
			if text, ok := item.(string); ok && strings.EqualFold(strings.TrimSpace(text), "string") {
				return true
			}
		}
	case []string:
		for _, text := range v {
			if strings.EqualFold(strings.TrimSpace(text), "string") {
				return true
			}
		}
	}
	return false
}
