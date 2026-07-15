package usageestimate

import (
	"bytes"
	"encoding/json"
	"strings"
)

func normalizeAPI(api string) string {
	return strings.ToLower(strings.TrimSpace(api))
}
func clampBytes(b []byte, limit int) []byte {
	if limit <= 0 || len(b) <= limit {
		return b
	}
	return b[:limit]
}
func intFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
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

// func ExtractResponsesToEstimateContext(in Input, ectx *EstimateContext) (*EstimateContext, bool) {
// 	switch normalizeAPI(in.API) {
// 	case apiChatCompletions:
// 	}
// }

func ExtractStructToEstimateContext(ectx *EstimateContext, bodyMap map[string]any) {
	if ectx.Direction == EstimateInput {
		extractRequestToEstimateContext(ectx, bodyMap)
	} else {
		extractResponsesToEstimateContext(ectx, bodyMap)
	}

}

func extractRequestToEstimateContext(ectx *EstimateContext, bodyMap map[string]any) {

	switch normalizeAPI(ectx.API) {
	case apiChatCompletions:
		extractOpenAIChatRequest(ectx, bodyMap)
	case apiResponses:
		extractOpenAIResponsesRequest(ectx, bodyMap)
	case apiMessages:
		extractAnthropicMessagesRequest(ectx, bodyMap)
	case apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
		extractGeminiRequest(ectx, bodyMap)
	case apiEmbeddings:
		extractEmbeddingRequest(ectx, bodyMap)
	}
}

func addToolDefinitionOverheads(ectx *EstimateContext, tools []EstimateTool) {
	count := countNonWebSearchTools(tools)
	if count == 0 {
		return
	}
	ectx.AddOverHead(ItemToolSection, 1)
	ectx.AddOverHead(ItemToolDefinition, count)
}

func countNonWebSearchTools(tools []EstimateTool) int {
	count := 0
	for _, tool := range tools {
		if isWebSearchTool(tool) {
			continue
		}
		count++
	}
	return count
}

func isWebSearchTool(tool EstimateTool) bool {
	return isWebSearchToolName(tool.Type) || isWebSearchToolName(tool.Name)
}

func isWebSearchToolName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
	return normalized == "googlesearch" ||
		normalized == "google_search" ||
		normalized == "websearch" ||
		normalized == "web_search" ||
		strings.HasPrefix(normalized, "web_search_")
}

func extractResponsesToEstimateContext(ectx *EstimateContext, bodyMap map[string]any) {
	switch normalizeAPI(ectx.API) {
	case apiChatCompletions:
		extractOpenAIChatResponse(ectx, bodyMap)
	case apiResponses:
		extractOpenAIResponsesResponse(ectx, bodyMap)
	case apiMessages:
		extractAnthropicMessagesResponse(ectx, bodyMap)
	case apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
		extractGeminiResponse(ectx, bodyMap)
	case apiEmbeddings:
		extractEmbeddingResponse(ectx, bodyMap)
	}
}
