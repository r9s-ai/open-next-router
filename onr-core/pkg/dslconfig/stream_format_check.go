package dslconfig

import (
	"encoding/json"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type CheckResult struct {
	ChunkCount int
	Api        string
	EventsSeen map[string]int
	Payload    *PayloadCheckResult
}

type PayloadCheckResult struct {
	CheckedCount int
	ValidCount   int
	InvalidCount int
	Issues       []PayloadIssue
}

type PayloadIssue struct {
	Event string
	Code  string
	Path  string
}

type StreamFormatChecker struct {
	res *CheckResult
}

func NewStreamFormatChecker(meta *dslmeta.Meta) *StreamFormatChecker {
	res := &CheckResult{}
	if meta != nil {
		res.Api = meta.API
	}
	return &StreamFormatChecker{res: res}
}

func (checker *StreamFormatChecker) OnSSEEventDataJSON(event string, payload []byte) error {
	if checker.res == nil {
		return nil
	}
	if len(payload) == 0 {
		return nil
	}
	checker.res.ChunkCount += 1
	rawEvent := strings.TrimSpace(event)
	// Parse the JSON structure first and run lightweight field checks.
	//
	// The checker is observability-only and must not block SSE passthrough. Even
	// when payload parsing fails, it still records chunk_count, event.seen, and
	// payload issues so upstream access logs can show malformed upstream chunks
	// without turning the response path into an error.
	root, issues := parsePayload(rawEvent, payload)
	// Decide how this chunk should be grouped under event.seen.
	//
	// When an SSE event field exists, it is authoritative. Otherwise infer from
	// payload type / choices / candidates so data-only streams such as OpenAI chat
	// and GLM chat do not all fall into fallback_data.
	eventName := eventNameForPayload(rawEvent, root)
	checker.recordEvent(eventName)
	checker.recordPayloadCheck(eventName, issues)
	return nil
}

func (checker *StreamFormatChecker) recordEvent(event string) {
	event = strings.TrimSpace(event)
	if event == "" {
		event = "fallback_data"
	}
	if checker.res.EventsSeen == nil {
		checker.res.EventsSeen = make(map[string]int)
	}
	checker.res.EventsSeen[event] += 1
}

func (checker *StreamFormatChecker) recordPayloadCheck(event string, issues []PayloadIssue) {
	if checker.res.Payload == nil {
		checker.res.Payload = &PayloadCheckResult{}
	}
	checker.res.Payload.CheckedCount += 1
	if len(issues) == 0 {
		checker.res.Payload.ValidCount += 1
		return
	}
	checker.res.Payload.InvalidCount += 1
	for _, issue := range issues {
		if issue.Event == "" {
			issue.Event = event
		}
		checker.res.Payload.Issues = append(checker.res.Payload.Issues, issue)
	}
}

func parsePayload(rawEvent string, payload []byte) (map[string]any, []PayloadIssue) {
	// The payload must be a JSON object.
	//
	// Keep two failure classes separate:
	// 1. invalid_json: the payload is not valid JSON, such as partial JSON, plain
	//    text, or broken upstream concatenation.
	// 2. invalid_payload_root: the payload is valid JSON, but the root is not an
	//    object, such as an array, string, or number.
	//
	// Normal provider SSE data chunks should usually be objects. [DONE] is
	// filtered by the Tap layer before reaching this function.
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, []PayloadIssue{{
			Event: fallbackEventName(rawEvent),
			Code:  "invalid_json",
		}}
	}
	root, ok := value.(map[string]any)
	if !ok || root == nil {
		return nil, []PayloadIssue{{
			Event: fallbackEventName(rawEvent),
			Code:  "invalid_payload_root",
		}}
	}
	issues := payloadFieldIssues(rawEvent, root)
	return root, issues
}

func payloadFieldIssues(rawEvent string, root map[string]any) []PayloadIssue {
	var issues []PayloadIssue
	// Only run non-blocking lightweight field shape checks here. Do not extract
	// usage or finish_reason from this checker.
	//
	// Design boundary:
	// - This checker records whether the payload looks like a chunk for a known
	//   protocol shape.
	// - DSL metrics performs the actual usage, finish_reason, and token
	//   extraction.
	// - Therefore missing fields are not errors here; only fields that are present
	//   with clearly invalid types are reported.

	// type:
	// - OpenAI Responses / Anthropic Messages event streams usually put the event
	//   name in payload.type.
	// - If type exists but is not a string, the payload structure is malformed.
	// - If an SSE event exists and payload.type differs, the upstream event/data
	//   pair is inconsistent.
	if value, ok := root["type"]; ok {
		typ, typeOK := value.(string)
		if !typeOK {
			issues = append(issues, PayloadIssue{Code: "invalid_type_field", Path: "type"})
		} else if rawEvent != "" && typ != rawEvent {
			issues = append(issues, PayloadIssue{Event: rawEvent, Code: "event_type_mismatch", Path: "type"})
		}
	}
	// object:
	// - OpenAI chat completions commonly use object="chat.completion.chunk".
	// - Compatible APIs such as GLM/Z.ai may omit object, so missing object is not
	//   an error.
	// - Only report invalid_object_field when object exists but is not a string.
	if value, ok := root["object"]; ok {
		if _, typeOK := value.(string); !typeOK {
			issues = append(issues, PayloadIssue{Code: "invalid_object_field", Path: "object"})
		}
	}
	// choices:
	// - Chat completions streams such as OpenAI, DeepSeek, Minimax, and GLM are
	//   primarily represented through choices.
	// - choices should normally be an array; chunk/finish/usage classification is
	//   handled by eventNameForPayload.
	// - Missing choices is not an error because Responses, Anthropic, and Gemini do
	//   not use this field.
	if value, ok := root["choices"]; ok {
		if _, typeOK := value.([]any); !typeOK {
			issues = append(issues, PayloadIssue{Code: "invalid_choices_field", Path: "choices"})
		}
	}
	// candidates:
	// - Gemini native streamGenerateContent uses candidates.
	// - candidates should normally be an array; finish chunks are inferred from
	//   candidates[].finishReason.
	// - Missing candidates is not an error because OpenAI/Anthropic paths do not
	//   include this field.
	if value, ok := root["candidates"]; ok {
		if _, typeOK := value.([]any); !typeOK {
			issues = append(issues, PayloadIssue{Code: "invalid_candidates_field", Path: "candidates"})
		}
	}
	// usage:
	// - OpenAI-compatible streams may include a final usage chunk, or omit usage
	//   entirely.
	// - usage=null is common in streaming chunks and is not an error.
	// - Non-null usage should be an object; string/array/number values are
	//   structural issues.
	if value, ok := root["usage"]; ok && value != nil {
		if _, typeOK := value.(map[string]any); !typeOK {
			issues = append(issues, PayloadIssue{Code: "invalid_usage_field", Path: "usage"})
		}
	}
	return issues
}

func eventNameForPayload(rawEvent string, root map[string]any) string {
	// The SSE event field is the most reliable event name. Infer from provider
	// payload shape only when event is missing.
	//
	// Keep this priority stable:
	// 1. rawEvent: Anthropic/Responses and similar streams emit explicit events,
	//    and logs should preserve the original event name.
	// 2. payload.type: Anthropic/Responses data usually also has type, which can
	//    classify data when event is absent.
	// 3. choices: OpenAI-compatible chat completions usually omit event and must be
	//    classified through choices/finish_reason.
	// 4. candidates: Gemini native streamGenerateContent expresses chunks through
	//    candidates/finishReason.
	// 5. fallback_data: valid JSON without known protocol features, or payloads
	//    that cannot be parsed.
	if rawEvent != "" {
		return rawEvent
	}
	if root == nil {
		return "fallback_data"
	}
	if typ, ok := stringField(root, "type"); ok && typ != "" {
		return typ
	}
	if choices, ok := arrayField(root, "choices"); ok {
		return chatCompletionEventName(root, choices)
	}
	if candidates, ok := arrayField(root, "candidates"); ok {
		return geminiEventName(candidates)
	}
	return "fallback_data"
}

func fallbackEventName(rawEvent string) string {
	rawEvent = strings.TrimSpace(rawEvent)
	if rawEvent != "" {
		return rawEvent
	}
	return "fallback_data"
}

func chatCompletionEventName(root map[string]any, choices []any) string {
	// OpenAI-compatible streams usually omit event; GLM and similar APIs may also
	// omit object, so this only relies on choices/usage.
	//
	// Classification rules:
	// - choices=[] with object usage: classify as an independent usage chunk.
	// - choices[*].finish_reason as a non-empty string: classify as a finish chunk.
	// - choices[*].finish_reason as a non-string non-nil value: also classify as a
	//   finish chunk; more detailed field checks can report the shape issue later.
	// - Other choices chunks are treated as regular content delta chunks.
	if len(choices) == 0 {
		if usage, ok := root["usage"].(map[string]any); ok && usage != nil {
			return "chat.completion.usage"
		}
		return "chat.completion.chunk"
	}
	for _, item := range choices {
		choice, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finishReason, ok := choice["finish_reason"]; ok && finishReason != nil {
			if finish, ok := finishReason.(string); !ok || strings.TrimSpace(finish) != "" {
				return "chat.completion.finish"
			}
		}
	}
	return "chat.completion.chunk"
}

func geminiEventName(candidates []any) string {
	// Gemini native streamGenerateContent uses candidates[].finishReason to signal
	// finish chunks.
	//
	// Classification rules:
	// - Any candidate with a non-empty string finishReason is a finish chunk.
	// - A non-string non-nil finishReason is also treated as a finish chunk, so
	//   malformed finish signals are not missed.
	// - Missing or empty finishReason is treated as a regular content delta chunk.
	for _, item := range candidates {
		candidate, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finishReason, ok := candidate["finishReason"]; ok && finishReason != nil {
			if finish, ok := finishReason.(string); !ok || strings.TrimSpace(finish) != "" {
				return "gemini.generate_content.finish"
			}
		}
	}
	return "gemini.generate_content.chunk"
}

func stringField(root map[string]any, key string) (string, bool) {
	value, ok := root[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func arrayField(root map[string]any, key string) ([]any, bool) {
	value, ok := root[key]
	if !ok {
		return nil, false
	}
	items, ok := value.([]any)
	return items, ok
}

func (checker *StreamFormatChecker) Result() *CheckResult {
	if checker == nil {
		return nil
	}
	return checker.res
}
