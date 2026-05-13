package usageestimate

import (
	"encoding/json"
	"strings"
)

func stringfyOpenaiChatCompletionsRequest(reqBody map[string]any) *tokenEstimateContext {
	var b strings.Builder
	var ctx tokenEstimateContext
	for k, v := range reqBody {
		switch k {
		case "messages":
			b.WriteString("messages\n")
			stringifyOpenaiMessagesList(v, &b, &ctx, 0, 10)
			b.WriteString("\n")
		case "tools":
			b.WriteString("tools\n")
			toolList, ok := v.([]any)
			ctx.numTools = len(toolList)
			if !ok {
				data, _ := json.Marshal(v)
				b.WriteString(string(data) + "\n")
				break
			}
			for _, item := range toolList {
				tool, ok := item.(map[string]any)
				if !ok {
					data, _ := json.Marshal(item)
					b.WriteString(string(data) + "\n")
					break
				}
				if functionMap, ok := tool["function"].(map[string]any); ok {
					b.WriteString("function\n")
					if name, ok := functionMap["name"].(string); ok {
						b.WriteString(name + "\n")
					}
					if description, ok := functionMap["description"].(string); ok {
						b.WriteString(description + "\n")
					}
					if parameters, ok := functionMap["parameters"].(map[string]any); ok {
						data, _ := json.Marshal(parameters)
						strData := jsonNoiseReplacer.Replace(string(data))
						b.WriteString(strData + "\n")
					}
				}

			}
		default:
		}
	}
	ctx.text = b.String()
	return &ctx
}
func stringifyOpenaiMessagesList(v any, b *strings.Builder, ctx *tokenEstimateContext, deep int, maxDeep int) {
	if deep > maxDeep {
		return
	}
	switch t := v.(type) {
	case []any: // Handle content block list.
		for _, it := range t {
			stringifyOpenaiMessagesList(it, b, ctx, deep+1, maxDeep)
		}
		return
	case string: // Handle content string.
		b.WriteString(t)
		return
	case map[string]any: // Handle content block.
		stringifyOpenaiMessagesObject(t, b, ctx, deep, maxDeep)
		return

	default:
		return

	}

}
func stringifyOpenaiMessagesObject(t map[string]any, b *strings.Builder, ctx *tokenEstimateContext, deep int, maxDeep int) {
	if role, ok := t["role"].(string); ok { // Role info.
		b.WriteString("role:" + role + "\n")
	}

	if typeInfo, ok := t["type"].(string); ok {
		stringifyOpenaiMessagesContent(typeInfo, t, b, ctx, deep, maxDeep)
		return
	}

	if content, ok := t["content"].([]any); ok {
		stringifyOpenaiMessagesList(content, b, ctx, deep+1, maxDeep)
		b.WriteString("\n")
	}
	if content, ok := t["content"].(string); ok {
		b.WriteString(content + "\n")
	}
	if toolCalls, ok := t["tool_calls"].([]any); ok {
		for _, toolCall := range toolCalls {
			stringifyOpenaiToolCall(toolCall, b, ctx)
		}
	}
	if functionCall, ok := t["function_call"].(map[string]any); ok {
		ctx.numFunctionCalls += 1
		b.WriteString("function_call ")
		if name, ok := functionCall["name"].(string); ok {
			b.WriteString(name + " ")
		}
		if arguments, ok := functionCall["arguments"].(string); ok {
			b.WriteString(arguments)
		}
		b.WriteString("\n")
	}
}
func stringifyOpenaiToolCall(v any, b *strings.Builder, ctx *tokenEstimateContext) {
	toolCall, ok := v.(map[string]any)
	if !ok {
		return
	}
	typeInfo, _ := toolCall["type"].(string)
	switch typeInfo {
	case "function", "":
		ctx.numFunctionCalls += 1
		b.WriteString("function_call ")
		functionMap, _ := toolCall["function"].(map[string]any)
		if name, ok := functionMap["name"].(string); ok {
			b.WriteString(name + " ")
		}
		if arguments, ok := functionMap["arguments"].(string); ok {
			b.WriteString(arguments)
		}
		b.WriteString("\n")
	case "custom":
		ctx.numCustomToolCalls += 1
		b.WriteString("custom_tool_call ")
		customMap, _ := toolCall["custom"].(map[string]any)
		if name, ok := customMap["name"].(string); ok {
			b.WriteString(name + " ")
		}
		if input, ok := customMap["input"].(string); ok {
			b.WriteString(input)
		}
		b.WriteString("\n")
	}
}
func stringifyOpenaiMessagesContent(typeInfo string, t map[string]any, b *strings.Builder, ctx *tokenEstimateContext, deep int, maxDeep int) {
	switch typeInfo {

	case "text": // Text content.
		if text, ok := t["text"].(string); ok {
			b.WriteString(text)
		}
	case "image_url":
		// Image bytes and remote image contents are not estimated here.
	case "input_audio":
		// Audio payloads are not estimated here.
	case "file":
		// File payloads are not estimated here.
	default:
		// Unknown content parts are ignored.
	}
}

func stringfyOpenaiResponsesRequest(reqBody map[string]any) *tokenEstimateContext {
	var b strings.Builder
	var ctx tokenEstimateContext
	for k, v := range reqBody {
		switch k {
		case "instructions":
			b.WriteString("instructions\n")
			str, ok := v.(string)
			if !ok {
				break
			}
			b.WriteString(str + "\n")
		case "input":
			b.WriteString("input\n")
			stringifyInputs(v, &b, &ctx, 0, 10)
			b.WriteString("\n")

		case "tools":
			b.WriteString("tools\n")
			toolList, ok := v.([]any)
			ctx.numTools = len(toolList)
			if !ok {
				data, _ := json.Marshal(v)
				b.WriteString(string(data) + "\n")
				break
			}
			for _, item := range toolList {
				tool, ok := item.(map[string]any)
				if !ok {
					data, _ := json.Marshal(item)
					b.WriteString(string(data) + "\n")
					break
				}

				b.WriteString("function\n")
				if name, ok := tool["name"].(string); ok {
					b.WriteString(name + "\n")
				}
				if description, ok := tool["description"].(string); ok {
					b.WriteString(description + "\n")
				}
				if parameters, ok := tool["parameters"].(map[string]any); ok {
					data, _ := json.Marshal(parameters)
					strData := jsonNoiseReplacer.Replace(string(data))
					b.WriteString(strData + "\n")
				}
			}
		default:

		}
	}
	ctx.text = b.String()
	return &ctx

}
func stringifyInputs(v any, b *strings.Builder, ctx *tokenEstimateContext, deep int, maxDeep int) {
	if deep > maxDeep {
		return
	}
	switch t := v.(type) {
	case []any: // Handle content block list.
		for _, it := range t {
			stringifyInputs(it, b, ctx, deep+1, maxDeep)
		}
		return
	case string: // Handle content string.
		b.WriteString(t)
		return
	case map[string]any: // Handle content block.
		stringifyInputObject(t, b, ctx, deep, maxDeep)
		return

	default:
		return

	}

}
func stringifyInputObject(t map[string]any, b *strings.Builder, ctx *tokenEstimateContext, deep int, maxDeep int) {
	if role, ok := t["role"].(string); ok { // Role info.
		b.WriteString("role:" + role + "\n")
	}

	if typeInfo, ok := t["type"].(string); ok && typeInfo != "message" {
		stringifyInputContentObject(typeInfo, t, b, ctx, deep, maxDeep)
		return
	}

	if content, ok := t["content"]; ok {
		stringifyInputs(content, b, ctx, deep+1, maxDeep)
		b.WriteString("\n")
	}
}
func stringifyInputContentObject(typeInfo string, t map[string]any, b *strings.Builder, ctx *tokenEstimateContext, deep int, maxDeep int) {
	switch typeInfo {

	case "input_text", "output_text": // Text content.
		if text, ok := t["text"].(string); ok {
			b.WriteString(text)
		}
	case "reasoning":
		if summaryList, ok := t["summary"].([]any); ok {
			for _, item := range summaryList {
				if s, ok := item.(map[string]any); ok {
					typeName, _ := s["type"].(string)
					if typeName == "summary_text" {
						if text, ok := s["text"].(string); ok {
							b.WriteString(text)
						}
					}
				}
			}
		}
	case "code_interpreter_call":
		if code, ok := t["code"].(string); ok {
			b.WriteString(code)
		}
	case "input_image":
		// Image content is not extracted yet.
	case "input_file":
		// File content is not extracted yet.

	case "function_call": // Extract tool call details.
		ctx.numFunctionCalls += 1
		b.WriteString(typeInfo + " ")
		if toolName, ok := t["name"].(string); ok {
			b.WriteString(toolName + " ")
		}
		if input, ok := t["arguments"].(string); ok {
			b.Write([]byte(input))
		} else {
			data, _ := json.Marshal(input)
			b.Write(data)
		}

	case "function_call_output", "custom_tool_call_output": // Count custom outputs with function outputs for current calibration.
		ctx.numFunctionCallOutputs += 1
		b.WriteString(typeInfo + " ")
		if content, ok := t["output"]; ok {
			data, _ := json.Marshal(content)
			b.Write(data)
		}
	case "custom_tool_call":
		ctx.numCustomToolCalls += 1
		b.WriteString(typeInfo + " ")
		if toolName, ok := t["name"].(string); ok {
			b.WriteString(toolName + " ")
		}
		if input, ok := t["input"]; ok {
			data, _ := json.Marshal(input)
			b.Write(data)
		}

		// Tool call types not yet extracted for token estimation.
	case "shell_call",
		"shell_call_output",
		"computer_call",
		"computer_call_output",
		"web_search_call",
		"file_search_call",
		"mcp_call",
		"mcp_approval_request",
		"mcp_approval_response",
		"mcp_list_tools",
		"tool_search_output",
		"apply_patch_call",
		"apply_patch_call_output",
		"compaction",
		"item_reference":
		// These Responses item types are not extracted yet.

	default:
		// Unsupported Responses input item types are not extracted yet.

	}
}
