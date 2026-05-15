package usageestimate

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/streamtext"
)

func extractResponseText(api string, body []byte, limit int) string {
	return extractResponseTextForModel(api, "", body, limit)
}

// ExtractResponseTextForModel returns the same non-stream response text used by token estimation.
func ExtractResponseTextForModel(api, model string, body []byte, limit int) string {
	return extractResponseTextForModel(api, model, body, limit)
}

//gocyclo:ignore
func extractResponseTextForModel(api, model string, body []byte, limit int) string {
	body = clampBytes(body, limit)
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}
	var obj any
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	m, _ := obj.(map[string]any)
	if m == nil {
		return ""
	}

	switch normalizeAPI(api) {
	case apiChatCompletions:
		// choices[].message.content or choices[].text
		if v, ok := m["choices"]; ok {
			if arr, ok := v.([]any); ok {
				var b strings.Builder
				for _, it := range arr {
					cm, _ := it.(map[string]any)
					if cm == nil {
						continue
					}
					if msg, ok := cm["message"].(map[string]any); ok {
						if s, ok := msg["content"].(string); ok {
							b.WriteString(s)
							b.WriteByte('\n')
						}
						if toolCalls, ok := msg["tool_calls"].([]any); ok {
							var ctx tokenEstimateContext
							for _, toolCall := range toolCalls {
								stringifyOpenaiToolCall(toolCall, &b, &ctx)
							}
						}
						if functionCall, ok := msg["function_call"].(map[string]any); ok {
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
					if s, ok := cm["text"].(string); ok {
						b.WriteString(s)
						b.WriteByte('\n')
					}
				}
				return b.String()
			}
		}
	case apiMessages:
		// content[].text
		if v, ok := m["content"]; ok {
			if arr, ok := v.([]any); ok {
				var b strings.Builder
				skipThinking := isAnthropic47Model(model)
				for _, it := range arr {
					im, _ := it.(map[string]any)
					if im == nil {
						continue
					}
					if typeInfo, ok := im["type"].(string); ok {
						switch typeInfo {
						case "thinking":
							if skipThinking {
								continue
							}
							if thinkingText, ok := im[typeInfo].(string); ok {
								b.WriteString(thinkingText + "\n")
							}
						case "text":
							if thinking_text, ok := im[typeInfo].(string); ok {
								b.WriteString(thinking_text + "\n")
							}
						case "tool_use", "server_tool_use": // Extract tool call details.
							b.WriteString(typeInfo + " ")
							if toolName, ok := im["name"].(string); ok {
								b.WriteString(toolName + " ")
							}
							if input, ok := im["input"].(map[string]any); ok {
								data, _ := json.Marshal(input)
								b.Write(data)
							}
							// web_search_tool_result and redacted_thinking are skipped for now.
						}
					}
				}
				return b.String()
			}
		}
	case apiResponses:
		// best-effort: output_text or any nested "text"
		if output, ok := m["output"].([]any); ok {
			var b strings.Builder
			var ctx tokenEstimateContext
			stringifyInputs(output, &b, &ctx, 0, 10)
			return b.String()
		}
	case apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
		// Gemini native response: candidates[].content.parts[].text
		if v, ok := m["candidates"]; ok {
			return stringifyGeminiCandidates(v)
		}
	}

	// Fallback: gather any nested "text" fields.
	var out strings.Builder
	collectTextFields(&out, obj, 0, 8)
	return out.String()
}
func normalizeAPI(api string) string {
	return strings.ToLower(strings.TrimSpace(api))
}
func extractStreamText(api string, sse []byte, limit int) string {
	return streamtext.ExtractFromSSE(api, sse, limit)
}
func collectTextFields(out *strings.Builder, v any, depth, maxDepth int) {
	if out == nil || depth > maxDepth || v == nil {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			if strings.EqualFold(k, "text") {
				if s, ok := vv.(string); ok && strings.TrimSpace(s) != "" {
					out.WriteString(s)
					out.WriteByte('\n')
					continue
				}
			}
			collectTextFields(out, vv, depth+1, maxDepth)
		}
	case []any:
		for _, it := range t {
			collectTextFields(out, it, depth+1, maxDepth)
		}
	}
}
func clampBytes(b []byte, limit int) []byte {
	if limit <= 0 || len(b) <= limit {
		return b
	}
	return b[:limit]
}
