package usageestimate

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/streamtext"
)

const (
	StageUpstream           = "upstream"
	StageEstimateBoth       = "estimate_both"
	StageEstimatePrompt     = "estimate_prompt"
	StageEstimateCompletion = "estimate_completion"
)

const (
	apiChatCompletions = "chat.completions"
	apiEmbeddings      = "embeddings"
	apiResponses       = "responses"
	apiMessages        = "claude.messages"

	apiGeminiGenerateContent       = "gemini.generatecontent"
	apiGeminiStreamGenerateContent = "gemini.streamgeneratecontent"
)

type Input struct {
	API   string
	Model string

	UpstreamUsage *dslconfig.Usage

	// Upstream request/response bodies (JSON for non-stream, SSE for stream).
	RequestBody  []byte
	RequestRoot  map[string]any
	ResponseBody []byte
	StreamTail   []byte
}

type Output struct {
	Usage *dslconfig.Usage
	Stage string
}

type parsedRequestBody struct {
	raw  []byte
	obj  any
	root map[string]any
	err  error
}

func Estimate(cfg *Config, in Input) Output {
	if cfg == nil || !cfg.IsAPIEnabled(in.API) {
		u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
		return Output{Usage: u, Stage: stage}
	}

	u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
	if u != nil {
		if !cfg.EstimateWhenMissingOrZero {
			return Output{Usage: u, Stage: stage}
		}
		// Upstream usage exists; optionally estimate missing fields (common in streaming).
		if stage == StageUpstream {
			if outU, outStage := estimateMissingFields(cfg, in, u); outStage != StageUpstream {
				return Output{Usage: outU, Stage: outStage}
			}
			return Output{Usage: u, Stage: stage}
		}
		// All-zero (or effectively missing) usage: allow estimation.
		if !isAllZero(u) {
			return Output{Usage: u, Stage: stage}
		}
	}
	if u == nil && !cfg.EstimateWhenMissingOrZero {
		return Output{Usage: nil, Stage: ""}
	}

	reqParsed := parseRequestBody(in.RequestBody, in.RequestRoot, cfg.MaxRequestBytes)
	reqText, numTools := extractRequestTextFromParsed(in.API, reqParsed)
	respText := ""
	if len(in.StreamTail) > 0 {
		respText = extractStreamText(in.API, in.StreamTail, cfg.MaxStreamCollectBytes)
	} else {
		respText = extractResponseText(in.API, in.ResponseBody, cfg.MaxResponseBytes)
	}

	est := &dslconfig.Usage{
		InputTokens:  EstimateTokenByModel(in.Model, reqText, numTools),
		OutputTokens: EstimateTokenByModel(in.Model, respText, numTools),
	}
	est.TotalTokens = est.InputTokens + est.OutputTokens

	// Best-effort overhead for OpenAI-style chat messages.
	if normalizeAPI(in.API) == apiChatCompletions {
		msgCount := countMessagesFromParsed(reqParsed)
		if msgCount > 0 {
			est.InputTokens += msgCount*3 + 3
			est.TotalTokens = est.InputTokens + est.OutputTokens
		}
	}

	return Output{Usage: est, Stage: StageEstimateBoth}
}

func estimateMissingFields(cfg *Config, in Input, u *dslconfig.Usage) (*dslconfig.Usage, string) {
	if cfg == nil || u == nil {
		return u, StageUpstream
	}
	needPrompt := u.InputTokens == 0
	needCompletion := u.OutputTokens == 0
	if !needPrompt && !needCompletion {
		return u, StageUpstream
	}

	reqParsed := parseRequestBody(in.RequestBody, in.RequestRoot, cfg.MaxRequestBytes)
	reqText := ""
	numTools := 0
	if needPrompt {
		reqText, numTools = extractRequestTextFromParsed(in.API, reqParsed)
		if strings.TrimSpace(reqText) == "" {
			needPrompt = false
		}
	}

	respText := ""
	if needCompletion {
		if len(in.StreamTail) > 0 {
			respText = extractStreamText(in.API, in.StreamTail, cfg.MaxStreamCollectBytes)
		} else {
			respText = extractResponseText(in.API, in.ResponseBody, cfg.MaxResponseBytes)
		}
		if strings.TrimSpace(respText) == "" {
			needCompletion = false
		}
	}

	if !needPrompt && !needCompletion {
		return u, StageUpstream
	}

	out := *u
	if needPrompt {
		out.InputTokens = EstimateTokenByModel(in.Model, reqText, numTools)
	}
	if needCompletion {
		out.OutputTokens = EstimateTokenByModel(in.Model, respText, numTools)
	}
	out.TotalTokens = out.InputTokens + out.OutputTokens

	// Best-effort overhead for OpenAI-style chat messages only when prompt is estimated.
	if needPrompt && normalizeAPI(in.API) == apiChatCompletions {
		msgCount := countMessagesFromParsed(reqParsed)
		if msgCount > 0 {
			out.InputTokens += msgCount*3 + 3
			out.TotalTokens = out.InputTokens + out.OutputTokens
		}
	}

	switch {
	case needPrompt && needCompletion:
		return &out, StageEstimateBoth
	case needPrompt:
		return &out, StageEstimatePrompt
	case needCompletion:
		return &out, StageEstimateCompletion
	default:
		return u, StageUpstream
	}
}

func normalizeUpstreamUsage(u *dslconfig.Usage) (*dslconfig.Usage, string) {
	if u == nil {
		return nil, ""
	}
	// Copy to avoid mutating callers.
	out := *u

	// Normalize legacy OpenAI fields.
	if out.InputTokens == 0 && out.PromptTokens != 0 {
		out.InputTokens = out.PromptTokens
	}
	if out.OutputTokens == 0 && out.CompletionTokens != 0 {
		out.OutputTokens = out.CompletionTokens
	}
	if out.TotalTokens == 0 && (out.InputTokens != 0 || out.OutputTokens != 0) {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}

	if isAllZero(&out) {
		return &out, ""
	}
	return &out, StageUpstream
}

func isAllZero(u *dslconfig.Usage) bool {
	if u == nil {
		return true
	}
	return u.InputTokens == 0 && u.OutputTokens == 0 && u.TotalTokens == 0 &&
		(u.InputTokenDetails == nil || (u.InputTokenDetails.CachedTokens == 0 && u.InputTokenDetails.CacheWriteTokens == 0))
}

func extractRequestText(api string, body []byte, limit int) (string, int) {
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

func extractRequestTextFromParsed(api string, parsed parsedRequestBody) (string, int) {
	if len(bytes.TrimSpace(parsed.raw)) == 0 {
		return "", 0
	}
	if parsed.err != nil {
		return string(bytes.TrimSpace(parsed.raw)), 0
	}
	m := parsed.root
	if m == nil {
		return "", 0
	}

	switch normalizeAPI(api) {
	case apiEmbeddings, apiResponses:
		// responses can use "input", embeddings uses "input".
		if v, ok := m["input"]; ok {
			return stringifyAny(v), 0
		}
	case apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
		// Gemini native request: contents[].parts[].text
		if v, ok := m["contents"]; ok {
			return stringifyGeminiContents(v), 0
		}
	case apiMessages:
		return stringifyAnthropicRequest(m)

	}
	if v, ok := m["prompt"]; ok {
		return stringifyAny(v), 0
	}
	if v, ok := m["input"]; ok {
		return stringifyAny(v), 0
	}
	return "", 0
}

func extractResponseText(api string, body []byte, limit int) string {
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
				for _, it := range arr {
					im, _ := it.(map[string]any)
					if im == nil {
						continue
					}
					if typeInfo, ok := im["type"].(string); ok {
						switch typeInfo {
						case "thinking", "text":
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
		if s, ok := m["output_text"].(string); ok && strings.TrimSpace(s) != "" {
			return s
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

func stringifyAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, it := range t {
			s := stringifyAny(it)
			if strings.TrimSpace(s) == "" {
				continue
			}
			b.WriteString(s)
			b.WriteByte('\n')
		}
		return b.String()
	case map[string]any:
		var b strings.Builder
		if role, ok := t["role"].(string); ok {
			b.WriteString(role + " ")
		}

		if text, ok := t["text"].(string); ok {
			b.WriteString(text + " ")
		}
		if content, ok := t["content"].(string); ok {
			// Anthropic example: {"role": "user", "content": "Hello"}
			b.WriteString(content + " ")
		}

		collectTextFields(&b, t, 0, 4)
		return b.String()
	default:
		return ""
	}
}

// extract text content and tool count from Anthropic request body for token estimation
func stringifyAnthropicRequest(reqBody map[string]any) (string, int) {
	var b strings.Builder
	var numTools int
	for k, v := range reqBody {
		switch k {
		case "system":
			b.WriteString("system\n")
			// data, _ := json.Marshal(v)
			// b.WriteString(string(data) + "\n")
			stringifyAnthropicMessages(v, &b, 0, 10)
			b.WriteString("\n")
		case "tools":
			b.WriteString("tools\n")
			toolList, ok := v.([]any)
			numTools = len(toolList)
			if !ok {
				data, _ := json.Marshal(v)
				b.WriteString(string(data) + "\n")
				break
			}
			for _, item := range toolList {
				tool, ok := item.(map[string]any)
				if ok {
					data, _ := json.Marshal(tool)
					b.WriteString(string(data) + "\n")
				}
			}
		case "messages":
			b.WriteString("messages\n")
			stringifyAnthropicMessages(v, &b, 0, 10)
			b.WriteString("\n")
		}

	}
	return b.String(), numTools
}

func stringifyAnthropicMessages(v any, b *strings.Builder, deep int, maxDeep int) {
	if deep > maxDeep {
		return
	}
	switch t := v.(type) {
	case []any: // Handle content block list.
		for _, it := range t {
			stringifyAnthropicMessages(it, b, deep+1, maxDeep)
		}
		return
	case string: // Handle content string.
		b.WriteString(t)
		return
	case map[string]any: // Handle content block.
		stringifyAnthropicMessageObject(t, b, deep, maxDeep)
		return

	default:
		return

	}

}

func stringifyAnthropicMessageObject(t map[string]any, b *strings.Builder, deep int, maxDeep int) {
	if role, ok := t["role"].(string); ok { // Role info.
		b.WriteString("role:" + role + "\n")
	}

	if typeInfo, ok := t["type"].(string); ok {
		stringifyAnthropicContentBlock(typeInfo, t, b, deep, maxDeep)
		return
	}

	if content, ok := t["content"]; ok {
		stringifyAnthropicMessages(content, b, deep+1, maxDeep)
		b.WriteString("\n")
	}
}

func stringifyAnthropicContentBlock(typeInfo string, t map[string]any, b *strings.Builder, deep int, maxDeep int) {
	switch typeInfo {
	case "text": // Text content.
		if text, ok := t["text"].(string); ok {
			b.WriteString(text)
		}
	case "image": // TODO: extract image content.
		b.WriteString("")
	case "document": // TODO: extract document content.
		b.WriteString("")
	case "tool_use", "server_tool_use": // Extract tool call details.
		b.WriteString(typeInfo + " ")
		if toolName, ok := t["name"].(string); ok {
			b.WriteString(toolName + " ")
		}
		if input, ok := t["input"].(map[string]any); ok {
			data, _ := json.Marshal(input)
			b.Write(data)
		}
	case "tool_result": // Extract tool result details.
		b.WriteString(typeInfo + " ")
		if content, ok := t["content"]; ok {
			stringifyAnthropicToolResultContent(content, b, deep, maxDeep)
		}
	case "web_search_tool_result":
		// Web search result is not estimated yet.
	case "thinking": // Thinking content.
		if thinking, ok := t["thinking"].(string); ok {
			b.WriteString("thinking " + thinking)
		}
	default:
		b.WriteString("")
	}
}

func stringifyAnthropicToolResultContent(content any, b *strings.Builder, deep int, maxDeep int) {
	switch t := content.(type) {
	case string, []any:
		stringifyAnthropicMessages(t, b, deep+1, maxDeep)
	case map[string]any:
		if _, ok := t["type"].(string); ok {
			stringifyAnthropicMessages(t, b, deep+1, maxDeep)
			return
		}
		data, _ := json.Marshal(t)
		b.Write(data)
	default:
		data, _ := json.Marshal(t)
		b.Write(data)
	}
}

func stringifyMessages(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return stringifyAny(v)
	}
	var b strings.Builder
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		if c, ok := m["content"]; ok {
			s := stringifyAny(c)
			if strings.TrimSpace(s) != "" {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func stringifyGeminiContents(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return stringifyAny(v)
	}
	var b strings.Builder
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		if parts, ok := m["parts"]; ok {
			s := stringifyAny(parts)
			if strings.TrimSpace(s) != "" {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func stringifyGeminiCandidates(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return stringifyAny(v)
	}
	var b strings.Builder
	for _, it := range arr {
		m, _ := it.(map[string]any)
		if m == nil {
			continue
		}
		if content, ok := m["content"].(map[string]any); ok {
			if parts, ok := content["parts"]; ok {
				s := stringifyAny(parts)
				if strings.TrimSpace(s) != "" {
					b.WriteString(s)
					b.WriteByte('\n')
				}
			}
		}
	}
	return b.String()
}

func clampBytes(b []byte, limit int) []byte {
	if limit <= 0 || len(b) <= limit {
		return b
	}
	return b[:limit]
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
