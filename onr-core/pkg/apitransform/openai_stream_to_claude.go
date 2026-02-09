package apitransform

import (
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// MapOpenAIChatCompletionsChunkToClaudeEvents maps one OpenAI chat chunk JSON to Claude-style stream events (JSON array).
func MapOpenAIChatCompletionsChunkToClaudeEvents(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "openai chat chunk")
	if err != nil {
		return nil, err
	}
	events, err := MapOpenAIChatCompletionsChunkToClaudeEventsObject(root)
	if err != nil {
		return nil, err
	}
	return apitypes.JSONObject{"events": events}.Marshal()
}

// MapOpenAIChatCompletionsChunkToClaudeEventsObject maps one OpenAI chat chunk object to Claude-style stream events.
func MapOpenAIChatCompletionsChunkToClaudeEventsObject(root apitypes.JSONObject) ([]apitypes.JSONObject, error) {
	choices, _ := root["choices"].([]any)
	if len(choices) == 0 {
		return nil, fmt.Errorf("choices is required")
	}

	events := make([]apitypes.JSONObject, 0, 4)
	for _, raw := range choices {
		ch, _ := raw.(map[string]any)
		if ch == nil {
			continue
		}
		delta, _ := ch["delta"].(map[string]any)
		if delta != nil {
			if text := jsonutil.CoerceString(delta["content"]); text != "" {
				events = append(events, apitypes.JSONObject{
					"type":  "content_block_delta",
					"index": 0,
					"delta": apitypes.JSONObject{
						"type": "text_delta",
						"text": text,
					},
				})
			}
			if toolCalls, _ := delta["tool_calls"].([]any); len(toolCalls) > 0 {
				for _, tr := range toolCalls {
					tc, _ := tr.(map[string]any)
					if tc == nil {
						continue
					}
					idx := jsonutil.CoerceInt(tc["index"])
					id := jsonutil.CoerceString(tc["id"])
					fn, _ := tc["function"].(map[string]any)
					name := jsonutil.CoerceString(fn["name"])
					args := strings.TrimSpace(jsonutil.CoerceString(fn["arguments"]))
					events = append(events, apitypes.JSONObject{
						"type":  "content_block_start",
						"index": idx,
						"content_block": apitypes.JSONObject{
							"type":  "tool_use",
							"id":    id,
							"name":  name,
							"input": apitypes.JSONObject{},
						},
					})
					if args != "" {
						events = append(events, apitypes.JSONObject{
							"type":  "content_block_delta",
							"index": idx,
							"delta": apitypes.JSONObject{
								"type":         "input_json_delta",
								"partial_json": args,
							},
						})
					}
				}
			}
		}

		finish := strings.TrimSpace(jsonutil.CoerceString(ch["finish_reason"]))
		if finish != "" {
			stopReason := mapOpenAIFinishToClaudeStop(finish)
			ev := apitypes.JSONObject{
				"type": "message_delta",
				"delta": apitypes.JSONObject{
					"stop_reason": stopReason,
				},
			}
			if u, _ := root["usage"].(map[string]any); u != nil {
				inputTokens := jsonutil.GetIntByPath(u, "$.prompt_tokens")
				if inputTokens == 0 {
					inputTokens = jsonutil.GetIntByPath(u, "$.input_tokens")
				}
				outputTokens := jsonutil.GetIntByPath(u, "$.completion_tokens")
				if outputTokens == 0 {
					outputTokens = jsonutil.GetIntByPath(u, "$.output_tokens")
				}
				ev["usage"] = apitypes.JSONObject{
					"input_tokens":  inputTokens,
					"output_tokens": outputTokens,
				}
			}
			events = append(events, ev)
		}
	}
	return events, nil
}

func mapOpenAIFinishToClaudeStop(finish string) string {
	switch strings.TrimSpace(finish) {
	case "length":
		return claudeStopReasonMax
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}
