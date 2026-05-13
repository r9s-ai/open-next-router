package usageestimate

import (
	"bytes"
	"encoding/json"
)

func extractRequestText(api string, body []byte, limit int) *tokenEstimateContext {
	return extractRequestTextFromParsed(api, parseRequestBody(body, nil, limit))
}
func parseRequestBody(body []byte, root map[string]any, limit int) parsedRequestBody {
	body = clampBytes(body, limit)
	if root != nil {
		return parsedRequestBody{raw: body, obj: root, root: root}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return parsedRequestBody{raw: body}
	}
	var obj any
	if err := json.Unmarshal(body, &obj); err != nil {
		return parsedRequestBody{raw: body, err: err}
	}
	m, _ := obj.(map[string]any)
	return parsedRequestBody{raw: body, obj: obj, root: m}
}
func extractRequestTextFromParsed(api string, parsed parsedRequestBody) *tokenEstimateContext {
	ctx := &tokenEstimateContext{}
	if len(bytes.TrimSpace(parsed.raw)) == 0 {
		return ctx
	}
	if parsed.err != nil {
		ctx.text = string(bytes.TrimSpace(parsed.raw))
		return ctx
	}
	m := parsed.root
	if m == nil {
		return ctx
	}

	switch normalizeAPI(api) {
	case apiChatCompletions:
		return stringfyOpenaiChatCompletionsRequest(m)
	case apiResponses:
		return stringfyOpenaiResponsesRequest(m)
	case apiEmbeddings:
		// embeddings uses "input".
		if v, ok := m["input"]; ok {
			return &tokenEstimateContext{text: stringifyAny(v)}
		}
	case apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
		// Gemini native request: contents[].parts[].text
		if v, ok := m["contents"]; ok {
			return &tokenEstimateContext{text: stringifyGeminiContents(v)}
		}
	case apiMessages:
		return stringifyAnthropicRequest(m)

	}
	if v, ok := m["prompt"]; ok {
		return &tokenEstimateContext{text: stringifyAny(v)}
	}
	if v, ok := m["input"]; ok {
		return &tokenEstimateContext{text: stringifyAny(v)}
	}
	return ctx
}
func countMessages(reqBody []byte, limit int) int {
	return countMessagesFromParsed(parseRequestBody(reqBody, nil, limit))
}
func countMessagesFromParsed(parsed parsedRequestBody) int {
	if parsed.root == nil {
		return 0
	}
	v, ok := parsed.root["messages"].([]any)
	if !ok {
		return 0
	}
	return len(v)
}
