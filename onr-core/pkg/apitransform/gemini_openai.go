package apitransform

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

const (
	openAIRoleAssistant = "assistant"
	openAIRoleSystem    = "system"
	finishContentFilter = "content_filter"
)

// MapOpenAIChatCompletionsToGeminiGenerateContentRequest maps OpenAI chat request JSON
// to Gemini generateContent request JSON.
func MapOpenAIChatCompletionsToGeminiGenerateContentRequest(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "openai request")
	if err != nil {
		return nil, err
	}
	out, err := MapOpenAIChatCompletionsToGeminiGenerateContentRequestObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapOpenAIChatCompletionsToGeminiGenerateContentRequestObject maps OpenAI chat request
// object to Gemini generateContent request object.
func MapOpenAIChatCompletionsToGeminiGenerateContentRequestObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	out := apitypes.JSONObject{}
	if model := strings.TrimSpace(jsonutil.CoerceString(root["model"])); model != "" {
		out["model"] = model
	}

	contents := make([]any, 0, 4)
	messages, _ := root["messages"].([]any)
	for _, raw := range messages {
		msg, _ := raw.(map[string]any)
		if msg == nil {
			continue
		}
		role := strings.TrimSpace(jsonutil.CoerceString(msg["role"]))
		switch role {
		case openAIRoleSystem:
			text := strings.TrimSpace(jsonutil.CoerceString(msg["content"]))
			if text == "" {
				continue
			}
			out["system_instruction"] = apitypes.JSONObject{
				"parts": []any{apitypes.JSONObject{"text": text}},
			}
		case openAIRoleAssistant:
			partItems := openAIMessageToGeminiParts(msg)
			if len(partItems) == 0 {
				continue
			}
			contents = append(contents, apitypes.JSONObject{
				"role":  "model",
				"parts": partItems,
			})
		default:
			partItems := openAIMessageToGeminiParts(msg)
			if len(partItems) == 0 {
				continue
			}
			contents = append(contents, apitypes.JSONObject{
				"role":  "user",
				"parts": partItems,
			})
		}
	}
	if len(contents) > 0 {
		out["contents"] = contents
	}

	cfg := apitypes.JSONObject{}
	if v := jsonutil.CoerceInt(root["max_tokens"]); v > 0 {
		cfg["maxOutputTokens"] = v
	}
	if v, ok := root["temperature"].(float64); ok {
		cfg["temperature"] = v
	}
	if v, ok := root["top_p"].(float64); ok {
		cfg["topP"] = v
	}
	if v := jsonutil.CoerceInt(root["n"]); v > 0 {
		cfg["candidateCount"] = v
	}
	if len(cfg) > 0 {
		out["generation_config"] = cfg
	}
	return out, nil
}

func openAIMessageToGeminiParts(msg map[string]any) []any {
	out := make([]any, 0, 2)
	if toolCalls, _ := msg["tool_calls"].([]any); len(toolCalls) > 0 {
		for _, raw := range toolCalls {
			tc, _ := raw.(map[string]any)
			if tc == nil {
				continue
			}
			fn, _ := tc["function"].(map[string]any)
			name := strings.TrimSpace(jsonutil.CoerceString(fn["name"]))
			if name == "" {
				continue
			}
			args := apitypes.JSONObject{}
			if rawArgs := strings.TrimSpace(jsonutil.CoerceString(fn["arguments"])); rawArgs != "" {
				var v any
				if err := json.Unmarshal([]byte(rawArgs), &v); err == nil {
					if m, ok := v.(map[string]any); ok && m != nil {
						args = m
					}
				}
			}
			out = append(out, apitypes.JSONObject{
				"functionCall": apitypes.JSONObject{
					"name": name,
					"args": args,
				},
			})
		}
		return out
	}
	if c := strings.TrimSpace(jsonutil.CoerceString(msg["content"])); c != "" {
		out = append(out, apitypes.JSONObject{"text": c})
	}
	return out
}

// MapGeminiGenerateContentToOpenAIChatCompletions maps Gemini request JSON to OpenAI chat request JSON.
func MapGeminiGenerateContentToOpenAIChatCompletions(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "gemini request")
	if err != nil {
		return nil, err
	}
	out, err := MapGeminiGenerateContentToOpenAIChatCompletionsObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapGeminiGenerateContentToOpenAIChatCompletionsObject maps Gemini request object to OpenAI chat request object.
func MapGeminiGenerateContentToOpenAIChatCompletionsObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	out := apitypes.JSONObject{}
	messages := mapGeminiContentsToOpenAIMessages(root["contents"])
	messages = prependGeminiSystemInstruction(firstObject(root, "system_instruction", "systemInstruction"), messages)
	if len(messages) > 0 {
		out["messages"] = messages
	}

	if model := strings.TrimSpace(jsonutil.CoerceString(root["model"])); model != "" {
		out["model"] = model
	}
	if stream, ok := root["stream"].(bool); ok {
		out["stream"] = stream
	}
	if so, _ := root["stream_options"].(map[string]any); so != nil {
		out["stream_options"] = so
	}

	if cfg := firstObject(root, "generation_config", "generationConfig"); cfg != nil {
		if v, ok := cfg["temperature"].(float64); ok {
			out["temperature"] = v
		}
		if v, ok := cfg["topP"].(float64); ok {
			out["top_p"] = v
		}
		if v := jsonutil.CoerceInt(cfg["maxOutputTokens"]); v > 0 {
			out["max_tokens"] = v
		}
		if v := jsonutil.CoerceInt(cfg["candidateCount"]); v > 0 {
			out["n"] = v
		}
	}

	return out, nil
}

func firstObject(root apitypes.JSONObject, keys ...string) map[string]any {
	for _, key := range keys {
		if m, _ := root[key].(map[string]any); m != nil {
			return m
		}
	}
	return nil
}

func mapGeminiContentsToOpenAIMessages(rawContents any) []any {
	contents, _ := rawContents.([]any)
	messages := make([]any, 0, len(contents)+1)
	for _, raw := range contents {
		content, _ := raw.(map[string]any)
		if content == nil {
			continue
		}
		msg, ok := mapOneGeminiContent(content)
		if ok {
			messages = append(messages, msg)
		}
	}
	return messages
}

func mapOneGeminiContent(content map[string]any) (apitypes.JSONObject, bool) {
	role := convertGeminiRoleToOpenAI(jsonutil.CoerceString(content["role"]))
	parts, _ := content["parts"].([]any)
	if len(parts) == 0 {
		return nil, false
	}
	media, toolCalls := mapGeminiParts(parts)
	msg := apitypes.JSONObject{"role": role}
	switch {
	case len(toolCalls) > 0:
		msg["content"] = ""
		msg["tool_calls"] = toolCalls
	case len(media) == 1:
		if partType, partText := textPartFromAny(media[0]); partType == chatContentTypeText {
			msg["content"] = partText
		} else {
			msg["content"] = media
		}
	default:
		msg["content"] = media
	}
	return msg, true
}

func mapGeminiParts(parts []any) ([]any, []any) {
	media := make([]any, 0, len(parts))
	toolCalls := make([]any, 0, 2)
	for _, pr := range parts {
		p, _ := pr.(map[string]any)
		if p == nil {
			continue
		}
		if mapped, ok := mapGeminiPartToMedia(p); ok {
			media = append(media, mapped)
			continue
		}
		if call, ok := mapGeminiPartToToolCall(p, len(toolCalls)); ok {
			toolCalls = append(toolCalls, call)
		}
	}
	return media, toolCalls
}

func mapGeminiPartToMedia(p map[string]any) (apitypes.JSONObject, bool) {
	if t := jsonutil.CoerceString(p["text"]); t != "" {
		return apitypes.JSONObject{"type": chatContentTypeText, "text": t}, true
	}
	if inline, _ := p["inlineData"].(map[string]any); inline != nil {
		mime := jsonutil.CoerceString(inline["mimeType"])
		data := jsonutil.CoerceString(inline["data"])
		if data != "" {
			return apitypes.JSONObject{
				"type": "image_url",
				"image_url": apitypes.JSONObject{
					"url": fmt.Sprintf("data:%s;base64,%s", mime, data),
				},
			}, true
		}
	}
	if file, _ := p["fileData"].(map[string]any); file != nil {
		uri := jsonutil.CoerceString(file["fileUri"])
		if uri != "" {
			return apitypes.JSONObject{
				"type": "image_url",
				"image_url": apitypes.JSONObject{
					"url": uri,
				},
			}, true
		}
	}
	return nil, false
}

func mapGeminiPartToToolCall(p map[string]any, idx int) (apitypes.JSONObject, bool) {
	fc, _ := p["functionCall"].(map[string]any)
	if fc == nil {
		return nil, false
	}
	name := strings.TrimSpace(jsonutil.CoerceString(fc["name"]))
	if name == "" {
		return nil, false
	}
	args := "{}"
	if fc["args"] != nil {
		if b, err := json.Marshal(fc["args"]); err == nil {
			args = string(b)
		}
	}
	return apitypes.JSONObject{
		"id":   fmt.Sprintf("call_%d", idx+1),
		"type": chatRoleFunction,
		"function": apitypes.JSONObject{
			"name":      name,
			"arguments": args,
		},
	}, true
}

func prependGeminiSystemInstruction(rawSystem any, messages []any) []any {
	sys, _ := rawSystem.(map[string]any)
	if sys == nil {
		return messages
	}
	parts, _ := sys["parts"].([]any)
	if len(parts) == 0 {
		return messages
	}
	var text []string
	for _, pr := range parts {
		p, _ := pr.(map[string]any)
		if p == nil {
			continue
		}
		if t := strings.TrimSpace(jsonutil.CoerceString(p["text"])); t != "" {
			text = append(text, t)
		}
	}
	if len(text) == 0 {
		return messages
	}
	return append([]any{apitypes.JSONObject{"role": openAIRoleSystem, "content": strings.Join(text, "\n")}}, messages...)
}

func textPartFromAny(v any) (partType string, partText string) {
	switch m0 := v.(type) {
	case map[string]any:
		return jsonutil.CoerceString(m0["type"]), jsonutil.CoerceString(m0["text"])
	case apitypes.JSONObject:
		return jsonutil.CoerceString(m0["type"]), jsonutil.CoerceString(m0["text"])
	default:
		return "", ""
	}
}

// MapGeminiGenerateContentToOpenAIChatCompletionsResponse maps Gemini response JSON to OpenAI chat response JSON.
func MapGeminiGenerateContentToOpenAIChatCompletionsResponse(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "gemini response")
	if err != nil {
		return nil, err
	}
	out, err := MapGeminiGenerateContentToOpenAIChatCompletionsResponseObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapGeminiGenerateContentToOpenAIChatCompletionsResponseObject maps Gemini response object to OpenAI chat response object.
func MapGeminiGenerateContentToOpenAIChatCompletionsResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	candidates, _ := root["candidates"].([]any)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("candidates is required")
	}

	choices := make([]any, 0, len(candidates))
	for i, raw := range candidates {
		cand, _ := raw.(map[string]any)
		if cand == nil {
			continue
		}
		content, _ := cand["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		msg := apitypes.JSONObject{
			"role":    openAIRoleAssistant,
			"content": "",
		}
		toolCalls := make([]any, 0, 2)
		textBuf := strings.Builder{}
		for _, pr := range parts {
			p, _ := pr.(map[string]any)
			if p == nil {
				continue
			}
			if t := jsonutil.CoerceString(p["text"]); t != "" {
				textBuf.WriteString(t)
				continue
			}
			if fc, _ := p["functionCall"].(map[string]any); fc != nil {
				name := strings.TrimSpace(jsonutil.CoerceString(fc["name"]))
				if name == "" {
					continue
				}
				argStr := "{}"
				if fc["args"] != nil {
					if b, err := json.Marshal(fc["args"]); err == nil {
						argStr = string(b)
					}
				}
				toolCalls = append(toolCalls, apitypes.JSONObject{
					"id":   "call_" + strconv.Itoa(len(toolCalls)+1),
					"type": chatRoleFunction,
					"function": apitypes.JSONObject{
						"name":      name,
						"arguments": argStr,
					},
				})
			}
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		} else {
			msg["content"] = textBuf.String()
		}

		idx := jsonutil.CoerceInt(cand["index"])
		if idx < 0 {
			idx = i
		}
		choices = append(choices, apitypes.JSONObject{
			"index":         idx,
			"message":       msg,
			"finish_reason": mapGeminiFinishToOpenAI(jsonutil.CoerceString(cand["finishReason"])),
		})
	}

	if len(choices) == 0 {
		return nil, fmt.Errorf("candidates is required")
	}

	usage := apitypes.JSONObject{}
	if u, _ := root["usageMetadata"].(map[string]any); u != nil {
		p := jsonutil.CoerceInt(u["promptTokenCount"])
		c := jsonutil.CoerceInt(u["candidatesTokenCount"])
		t := jsonutil.CoerceInt(u["totalTokenCount"])
		if t == 0 {
			t = p + c
		}
		usage["prompt_tokens"] = p
		usage["completion_tokens"] = c
		usage["total_tokens"] = t
	}

	out := apitypes.JSONObject{
		"id":      "chatcmpl_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"choices": choices,
	}
	if model := strings.TrimSpace(jsonutil.CoerceString(root["modelVersion"])); model != "" {
		out["model"] = model
	} else if model := strings.TrimSpace(jsonutil.CoerceString(root["model"])); model != "" {
		out["model"] = model
	}
	if len(usage) > 0 {
		out["usage"] = usage
	}
	return out, nil
}

func mapGeminiFinishToOpenAI(finish string) string {
	switch strings.TrimSpace(strings.ToUpper(finish)) {
	case "MAX_TOKENS":
		return finishReasonLength
	case "SAFETY", "RECITATION":
		return finishContentFilter
	case "":
		return finishReasonStop
	default:
		return finishReasonStop
	}
}

// MapOpenAIChatCompletionsToGeminiGenerateContentResponse maps OpenAI chat response JSON to Gemini response JSON.
func MapOpenAIChatCompletionsToGeminiGenerateContentResponse(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "openai response")
	if err != nil {
		return nil, err
	}
	out, err := MapOpenAIChatCompletionsToGeminiGenerateContentResponseObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapOpenAIChatCompletionsToGeminiGenerateContentResponseObject maps OpenAI chat response object to Gemini response object.
func MapOpenAIChatCompletionsToGeminiGenerateContentResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	choices, _ := root["choices"].([]any)
	if len(choices) == 0 {
		return nil, fmt.Errorf("choices is required")
	}
	candidates := make([]any, 0, len(choices))
	for _, raw := range choices {
		ch, _ := raw.(map[string]any)
		if ch == nil {
			continue
		}
		msg, _ := ch["message"].(map[string]any)
		if msg == nil {
			continue
		}
		parts := make([]any, 0, 2)
		if toolCalls, _ := msg["tool_calls"].([]any); len(toolCalls) > 0 {
			for _, tr := range toolCalls {
				tc, _ := tr.(map[string]any)
				if tc == nil {
					continue
				}
				fn, _ := tc["function"].(map[string]any)
				name := jsonutil.CoerceString(fn["name"])
				argsObj := apitypes.JSONObject{}
				if args := jsonutil.CoerceString(fn["arguments"]); args != "" {
					var v any
					if err := json.Unmarshal([]byte(args), &v); err == nil {
						if m, ok := v.(map[string]any); ok && m != nil {
							argsObj = m
						}
					}
				}
				parts = append(parts, apitypes.JSONObject{
					"functionCall": apitypes.JSONObject{
						"name": name,
						"args": argsObj,
					},
				})
			}
		} else {
			parts = append(parts, apitypes.JSONObject{"text": jsonutil.CoerceString(msg["content"])})
		}

		finish := mapOpenAIFinishToGemini(jsonutil.CoerceString(ch["finish_reason"]))
		candidates = append(candidates, apitypes.JSONObject{
			"index":        jsonutil.CoerceInt(ch["index"]),
			"finishReason": finish,
			"content": apitypes.JSONObject{
				"role":  "model",
				"parts": parts,
			},
			"safetyRatings": []any{},
		})
	}

	usageMeta := apitypes.JSONObject{}
	if u, _ := root["usage"].(map[string]any); u != nil {
		p := jsonutil.GetIntByPath(u, "$.prompt_tokens")
		if p == 0 {
			p = jsonutil.GetIntByPath(u, "$.input_tokens")
		}
		c := jsonutil.GetIntByPath(u, "$.completion_tokens")
		if c == 0 {
			c = jsonutil.GetIntByPath(u, "$.output_tokens")
		}
		t := jsonutil.GetIntByPath(u, "$.total_tokens")
		if t == 0 {
			t = p + c
		}
		usageMeta["promptTokenCount"] = p
		usageMeta["candidatesTokenCount"] = c
		usageMeta["totalTokenCount"] = t
	}

	out := apitypes.JSONObject{
		"candidates": candidates,
	}
	if len(usageMeta) > 0 {
		out["usageMetadata"] = usageMeta
	}
	return out, nil
}

func convertGeminiRoleToOpenAI(role string) string {
	switch strings.TrimSpace(role) {
	case "model":
		return openAIRoleAssistant
	case "function":
		return "function"
	case "system":
		return openAIRoleSystem
	default:
		return "user"
	}
}

func mapOpenAIFinishToGemini(finish string) string {
	switch strings.TrimSpace(finish) {
	case "length":
		return "MAX_TOKENS"
	case finishContentFilter:
		return "SAFETY"
	default:
		return "STOP"
	}
}
