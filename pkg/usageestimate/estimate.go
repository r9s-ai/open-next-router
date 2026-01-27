package usageestimate

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/edgefn/open-next-router/pkg/dslconfig"
)

const (
	StageUpstream           = "upstream"
	StageEstimateBoth       = "estimate_both"
	StageEstimatePrompt     = "estimate_prompt"
	StageEstimateCompletion = "estimate_completion"
)

type Input struct {
	API   string
	Model string

	UpstreamUsage *dslconfig.Usage

	// Upstream request/response bodies (JSON for non-stream, SSE for stream).
	RequestBody  []byte
	ResponseBody []byte
	StreamTail   []byte
}

type Output struct {
	Usage *dslconfig.Usage
	Stage string
}

func Estimate(cfg *Config, in Input) Output {
	if cfg == nil || !cfg.IsAPIEnabled(in.API) {
		u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
		return Output{Usage: u, Stage: stage}
	}

	u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
	if u != nil && stage == StageUpstream {
		return Output{Usage: u, Stage: stage}
	}

	// Decide whether to estimate.
	if u != nil && !cfg.EstimateWhenMissingOrZero {
		return Output{Usage: u, Stage: stage}
	}
	if u != nil && !isAllZero(u) {
		// Partial usage is present; do not estimate unless it's all-zero.
		return Output{Usage: u, Stage: stage}
	}
	if u == nil && !cfg.EstimateWhenMissingOrZero {
		return Output{Usage: nil, Stage: ""}
	}

	reqText := extractRequestText(in.API, in.RequestBody, cfg.MaxRequestBytes)
	respText := ""
	if len(in.StreamTail) > 0 {
		respText = extractStreamText(in.API, in.StreamTail, cfg.MaxStreamCollectBytes)
	} else {
		respText = extractResponseText(in.API, in.ResponseBody, cfg.MaxResponseBytes)
	}

	est := &dslconfig.Usage{
		InputTokens:  EstimateTokenByModel(in.Model, reqText),
		OutputTokens: EstimateTokenByModel(in.Model, respText),
	}
	est.TotalTokens = est.InputTokens + est.OutputTokens

	// Best-effort overhead for OpenAI-style chat messages.
	if strings.ToLower(strings.TrimSpace(in.API)) == "chat.completions" {
		msgCount := countMessages(in.RequestBody, cfg.MaxRequestBytes)
		if msgCount > 0 {
			est.InputTokens += msgCount*3 + 3
			est.TotalTokens = est.InputTokens + est.OutputTokens
		}
	}

	return Output{Usage: est, Stage: StageEstimateBoth}
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

func extractRequestText(api string, body []byte, limit int) string {
	body = clampBytes(body, limit)
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}
	var obj any
	if err := json.Unmarshal(body, &obj); err != nil {
		return string(bytes.TrimSpace(body))
	}
	m, _ := obj.(map[string]any)
	if m == nil {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(api)) {
	case "embeddings", "responses":
		// responses can use "input", embeddings uses "input".
		if v, ok := m["input"]; ok {
			return stringifyAny(v)
		}
	case "gemini.generatecontent", "gemini.streamgeneratecontent":
		// Gemini native request: contents[].parts[].text
		if v, ok := m["contents"]; ok {
			return stringifyGeminiContents(v)
		}
	}

	if v, ok := m["messages"]; ok {
		return stringifyMessages(v)
	}
	if v, ok := m["prompt"]; ok {
		return stringifyAny(v)
	}
	if v, ok := m["input"]; ok {
		return stringifyAny(v)
	}
	return ""
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

	switch strings.ToLower(strings.TrimSpace(api)) {
	case "chat.completions":
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
	case "claude.messages":
		// content[].text
		if v, ok := m["content"]; ok {
			if arr, ok := v.([]any); ok {
				var b strings.Builder
				for _, it := range arr {
					im, _ := it.(map[string]any)
					if im == nil {
						continue
					}
					if s, ok := im["text"].(string); ok {
						b.WriteString(s)
						b.WriteByte('\n')
					}
				}
				return b.String()
			}
		}
	case "responses":
		// best-effort: output_text or any nested "text"
		if s, ok := m["output_text"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	case "gemini.generatecontent", "gemini.streamgeneratecontent":
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

func extractStreamText(api string, sse []byte, limit int) string {
	sse = clampBytes(sse, limit)
	if len(bytes.TrimSpace(sse)) == 0 {
		return ""
	}
	events := bytes.Split(sse, []byte("\n\n"))
	var out strings.Builder
	for _, ev := range events {
		lines := bytes.Split(ev, []byte("\n"))
		var dataLines [][]byte
		for _, raw := range lines {
			line := bytes.TrimRight(raw, "\r")
			if bytes.HasPrefix(line, []byte("data:")) {
				dataLines = append(dataLines, bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
			}
		}
		if len(dataLines) == 0 {
			continue
		}
		payload := bytes.TrimSpace(bytes.Join(dataLines, []byte("\n")))
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		var obj any
		if err := json.Unmarshal(payload, &obj); err != nil {
			continue
		}
		m, _ := obj.(map[string]any)
		if m == nil {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(api)) {
		case "chat.completions":
			if v, ok := m["choices"].([]any); ok {
				for _, it := range v {
					cm, _ := it.(map[string]any)
					if cm == nil {
						continue
					}
					if d, ok := cm["delta"].(map[string]any); ok {
						if s, ok := d["content"].(string); ok && s != "" {
							out.WriteString(s)
						}
					}
					if s, ok := cm["text"].(string); ok && s != "" {
						out.WriteString(s)
					}
				}
				continue
			}
		case "responses":
			// OpenAI Responses SSE uses various event types; best-effort pick "delta".
			if s, ok := m["delta"].(string); ok && s != "" {
				out.WriteString(s)
				continue
			}
		case "claude.messages":
			// Anthropic SSE commonly contains delta.text.
			if d, ok := m["delta"].(map[string]any); ok {
				if s, ok := d["text"].(string); ok && s != "" {
					out.WriteString(s)
					continue
				}
			}
		}

		collectTextFields(&out, obj, 0, 6)
	}
	return out.String()
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
		if s, ok := t["text"].(string); ok {
			return s
		}
		var b strings.Builder
		collectTextFields(&b, t, 0, 4)
		return b.String()
	default:
		return ""
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
	reqBody = clampBytes(reqBody, limit)
	if len(bytes.TrimSpace(reqBody)) == 0 {
		return 0
	}
	var obj map[string]any
	if err := json.Unmarshal(reqBody, &obj); err != nil {
		return 0
	}
	v, ok := obj["messages"].([]any)
	if !ok {
		return 0
	}
	return len(v)
}
