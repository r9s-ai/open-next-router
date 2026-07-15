package usageestimate

func extractAnthropicMessagesRequest(ectx *EstimateContext, bodyMap map[string]any) {
	ectx.AddOverHead(ItemPromptBase, 1)
	if thinking, ok := bodyMap["thinking"].(map[string]any); ok {

		if v, ok := thinking["type"].(string); ok && v == "enabled" {
			ectx.AddOverHead(ItemThinkingBlock, 1)
			if v, ok := intFromAny(thinking["budget_tokens"]); ok {
				ectx.MaxThinkingTokens = v
			}
		}

	}
	if v, ok := intFromAny(bodyMap["max_tokens"]); ok {
		ectx.MaxTokens = v
	}
	if choiceMap, ok := stringMapFromAny(bodyMap["tool_choice"]); ok {
		ectx.ToolChoice = choiceMap
		switch choiceMap["type"] {
		case "any":
			ectx.AddOverHead(ItemToolChoiceAny, 1)
		case "auto":
			ectx.AddOverHead(ItemToolChoiceAuto, 1)
		case "none":
			ectx.AddOverHead(ItemToolChoiceNone, 1)
		default:
			ectx.AddOverHead(ItemToolChoiceToolName, 1)
		}
		//disable_parallel_tool_use not count now

	}

	if system, ok := bodyMap["system"]; ok {
		msg := EstimateMessage{
			Role:    "system",
			Content: extractAnthropicMessagesContent(ectx, system),
		}
		ectx.Messages = append(ectx.Messages, msg)
		ectx.AddOverHead(ItemSystemMessage, 1)

	}

	if messages, ok := bodyMap["messages"].([]any); ok {
		for _, item := range messages {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}

			msg := EstimateMessage{}
			if role, ok := m["role"].(string); ok {
				msg.Role = role
				switch msg.Role {
				case "user":
					ectx.AddOverHead(ItemRoleUser, 1)
				case "assistant":
					ectx.AddOverHead(ItemRoleAssistant, 1)
				case "system":
					ectx.AddOverHead(ItemRoleSystem, 1)
				}
			}
			msg.Content = extractAnthropicMessagesContent(ectx, m["content"])
			ectx.Messages = append(ectx.Messages, msg)
		}

	}

	if tools, ok := bodyMap["tools"].([]any); ok {
		estimateTool := extractAnthropicTools(ectx, tools)
		ectx.Tools = append(ectx.Tools, estimateTool...)
		addToolDefinitionOverheads(ectx, estimateTool)

	}
}

func extractAnthropicMessagesContent(ectx *EstimateContext, anthropicContent any) []EstimateMessagesContent {
	switch t := anthropicContent.(type) {
	case string:
		return []EstimateMessagesContent{{Type: "text", Text: t}}
	case []any:
		var out []EstimateMessagesContent
		for _, item := range t {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, ok := block["type"].(string)
			if !ok {
				continue
			}
			switch blockType {
			case "text":
				if text, ok := block["text"].(string); ok {
					out = append(out, EstimateMessagesContent{Type: "text", Text: text})
				}
			case "thinking":
				if text, ok := block["thinking"].(string); ok {
					signature := stringValue(block["signature"])
					if signature != "" {
						ectx.AddOverHead(ItemThinkingSignature, countTextLenTokens(signature))
					}
					out = append(out, EstimateMessagesContent{
						Type:      "thinking",
						Text:      text,
						Signature: signature,
					})
				}
			case "redacted_thinking":
				out = append(out, EstimateMessagesContent{Type: "redacted_thinking", Raw: block})
				ectx.AddOverHead(ItemHiddenReasoningBlock, 1)
			case "tool_use", "server_tool_use":

				toolUseBlock := extractAntthropicMessagesToolCall(ectx, block)
				ectx.AddOverHead(ItemFunctionCall, 1)
				out = append(out, toolUseBlock)

			case "tool_result", "web_search_tool_result":
				ectx.AddOverHead(ItemFunctionCallResult, 1)
				toolResultBlock := extractAntthropicMessagesToolResult(ectx, block)
				out = append(out, toolResultBlock)
			case "image":
				out = append(out, EstimateMessagesContent{Type: "image"})
				ectx.AddOverHead(ItemImageBlock, 1)
			case "document":
				out = append(out, EstimateMessagesContent{Type: "document"})
				ectx.AddOverHead(ItemDocumentBlock, 1)
			default:
				ectx.AddOverHead(ItemUnknownBlcok, 1)

			}
		}
		return out
	}
	return []EstimateMessagesContent{}
}

func extractAnthropicTools(ectx *EstimateContext, tools []any) []EstimateTool {
	var out []EstimateTool
	for _, tool := range tools {
		t := EstimateTool{}
		if v, ok := tool.(map[string]any); ok {

			if text, ok := v["name"].(string); ok {
				t.Name = text
			}
			if text, ok := v["type"].(string); ok {
				t.Type = text
			}
			if text, ok := v["description"].(string); ok {
				t.Description = text
			}
			if object, ok := v["input_schema"].(map[string]any); ok {
				t.Parameters = extracOpenAPISchema(ectx, object)
			}
			out = append(out, t)
		}
	}
	return out
}

func stringMapFromAny(v any) (map[string]string, bool) {
	switch t := v.(type) {
	case map[string]string:
		return t, true
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, v := range t {
			text, ok := v.(string)
			if !ok {
				continue
			}
			out[k] = text
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func extractAntthropicMessagesToolCall(ectx *EstimateContext, toolCallBlock map[string]any) EstimateMessagesContent {
	var out EstimateMessagesContent
	if text, ok := toolCallBlock["type"].(string); ok {
		out.Type = text
	}
	if text, ok := toolCallBlock["id"].(string); ok {
		out.ID = text
	}

	if text, ok := toolCallBlock["name"].(string); ok {
		out.Name = text
	}
	if mapInput, ok := toolCallBlock["input"].(map[string]any); ok {
		out.Arguments = mapInput
		ectx.AddOverHead(ItemToolUseBlockInput, 1)
		for _, v := range mapInput {
			ectx.AddOverHead(ItemToolUseBlockInputItem, 1)
			switch v.(type) {
			case string:
				ectx.AddOverHead(ItemToolUseBlockInputString, 1)
			case float64:
				ectx.AddOverHead(ItemToolUseBlockInputInt, 1)
			case bool:
				ectx.AddOverHead(ItemToolUseBlockInputBool, 1)
			case []any:
				ectx.AddOverHead(ItemToolUseBlockInputList, 1)
			}
		}
	}

	return out
}

func extractAntthropicMessagesToolResult(ectx *EstimateContext, toolResultBlock map[string]any) EstimateMessagesContent {
	var out EstimateMessagesContent
	if text, ok := toolResultBlock["type"].(string); ok {
		out.Type = text
	}
	if text, ok := toolResultBlock["tool_use_id"].(string); ok {
		out.ID = text
	}
	if text, ok := toolResultBlock["content"].(string); ok {
		out.Text = text
	} else if blocklist, ok := toolResultBlock["content"].([]any); ok {

		out.Content = append(out.Content, extractAnthropicMessagesContent(ectx, blocklist)...)
	}

	return out
}

func extractAnthropicMessagesResponse(ectx *EstimateContext, bodyMap map[string]any) {
	msg := EstimateMessage{Role: "assistant"}
	if role, ok := bodyMap["role"].(string); ok {
		msg.Role = role
	}
	switch msg.Role {
	case "assistant":
		ectx.AddOverHead(ItemRoleAssistant, 1)
	case "user":
		ectx.AddOverHead(ItemRoleUser, 1)
	case "system":
		ectx.AddOverHead(ItemRoleSystem, 1)
	}

	msg.Content = extractAnthropicMessagesContent(ectx, bodyMap["content"])
	ectx.Messages = append(ectx.Messages, msg)
}
