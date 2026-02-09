package apitransform

import (
	"strings"

	"github.com/r9s-ai/open-next-router/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/pkg/jsonutil"
)

// MapOpenAIChatCompletionsChunkToGeminiResponse maps one OpenAI chat chunk JSON to one Gemini-style response JSON.
// The returned bool indicates whether this chunk has meaningful payload and should be emitted.
func MapOpenAIChatCompletionsChunkToGeminiResponse(body []byte) ([]byte, bool, error) {
	root, err := apitypes.ParseJSONObject(body, "openai chat chunk")
	if err != nil {
		return nil, false, err
	}
	out, emit, err := MapOpenAIChatCompletionsChunkToGeminiResponseObject(root)
	if err != nil || !emit {
		return nil, emit, err
	}
	b, err := out.Marshal()
	return b, true, err
}

// MapOpenAIChatCompletionsChunkToGeminiResponseObject maps one OpenAI chat chunk object to one Gemini response object.
func MapOpenAIChatCompletionsChunkToGeminiResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, bool, error) {
	choices, _ := root["choices"].([]any)

	candidates := make([]any, 0, len(choices))
	hasPayload := false
	for _, raw := range choices {
		ch, _ := raw.(map[string]any)
		if ch == nil {
			continue
		}
		idx := jsonutil.CoerceInt(ch["index"])
		delta, _ := ch["delta"].(map[string]any)
		finish := strings.TrimSpace(jsonutil.CoerceString(ch["finish_reason"]))

		parts := make([]any, 0, 2)
		if delta != nil {
			if content := jsonutil.CoerceString(delta["content"]); content != "" {
				parts = append(parts, apitypes.JSONObject{"text": content})
				hasPayload = true
			}
			if toolCalls, _ := delta["tool_calls"].([]any); len(toolCalls) > 0 {
				for _, tr := range toolCalls {
					tc, _ := tr.(map[string]any)
					if tc == nil {
						continue
					}
					fn, _ := tc["function"].(map[string]any)
					name := jsonutil.CoerceString(fn["name"])
					args := apitypes.JSONObject{}
					if rawArgs := strings.TrimSpace(jsonutil.CoerceString(fn["arguments"])); rawArgs != "" {
						args["arguments"] = rawArgs
					}
					parts = append(parts, apitypes.JSONObject{
						"functionCall": apitypes.JSONObject{
							"name": name,
							"args": args,
						},
					})
				}
				hasPayload = true
			}
		}

		candidate := apitypes.JSONObject{
			"index":         idx,
			"safetyRatings": []any{},
			"content": apitypes.JSONObject{
				"role":  "model",
				"parts": parts,
			},
		}
		if finish != "" {
			candidate["finishReason"] = mapOpenAIFinishToGemini(finish)
			hasPayload = true
		}
		candidates = append(candidates, candidate)
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
		hasPayload = hasPayload || t > 0
	}

	if !hasPayload {
		return nil, false, nil
	}
	out := apitypes.JSONObject{
		"candidates": candidates,
	}
	if len(usageMeta) > 0 {
		out["usageMetadata"] = usageMeta
	}
	return out, true, nil
}
