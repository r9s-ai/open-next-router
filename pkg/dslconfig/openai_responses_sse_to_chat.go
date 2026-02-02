package dslconfig

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// TransformOpenAIResponsesSSEToChatCompletionsSSE converts OpenAI Responses SSE stream into
// OpenAI Chat Completions "data: {...}\n\n" chunks, ending with "data: [DONE]\n\n".
//
// Best-effort behavior:
//   - Any event whose JSON payload has a string field "delta" is treated as text delta.
//   - Tool call events are mapped into chat delta.tool_calls with argument deltas (new-api aligned).
//   - The "response.completed" (or any payload containing a "response" object) is used to emit a final chunk
//     with finish_reason and usage when available.
func TransformOpenAIResponsesSSEToChatCompletionsSSE(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)

	var (
		curEvent string
		curData  [][]byte

		chatID  string
		created int64
		model   string

		sentStart   bool
		sawText     bool
		sawToolCall bool

		toolCallIndexByID           = map[string]int{}
		toolCallNameByID            = map[string]string{}
		toolCallArgsByID            = map[string]string{}
		toolCallNameSent            = map[string]bool{}
		toolCallCanonicalIDByItemID = map[string]string{}
	)

	nowCreated := func() int64 {
		if created > 0 {
			return created
		}
		return time.Now().Unix()
	}
	ensureChatID := func(seed string) string {
		if strings.TrimSpace(chatID) != "" {
			return chatID
		}
		s := strings.TrimSpace(seed)
		if s == "" {
			s = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		if !strings.HasPrefix(s, "chatcmpl_") {
			s = "chatcmpl_" + s
		}
		chatID = s
		return chatID
	}

	sendStartIfNeeded := func() error {
		if sentStart {
			return nil
		}
		chunk := map[string]any{
			"id":      ensureChatID(""),
			"object":  "chat.completion.chunk",
			"created": nowCreated(),
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{},
				},
			},
		}
		if strings.TrimSpace(model) != "" {
			chunk["model"] = strings.TrimSpace(model)
		}
		if err := writeSSEDataJSON(w, chunk); err != nil {
			return err
		}
		sentStart = true
		return nil
	}

	sendTextDelta := func(delta string) error {
		if strings.TrimSpace(delta) == "" {
			return nil
		}
		// Prefer assistant text over tool calls (new-api aligned).
		sawText = true
		chunk := map[string]any{
			"id":      ensureChatID(""),
			"object":  "chat.completion.chunk",
			"created": nowCreated(),
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{"content": delta},
				},
			},
		}
		if strings.TrimSpace(model) != "" {
			chunk["model"] = strings.TrimSpace(model)
		}
		if err := writeSSEDataJSON(w, chunk); err != nil {
			return err
		}
		sentStart = true
		return nil
	}

	sendToolCallDelta := func(callID string, name string, argsDelta string) error {
		if strings.TrimSpace(callID) == "" {
			return nil
		}
		if sawText {
			// Prefer streaming assistant text over tool calls to match non-stream behavior.
			return nil
		}
		if err := sendStartIfNeeded(); err != nil {
			return err
		}
		idx, ok := toolCallIndexByID[callID]
		if !ok {
			idx = len(toolCallIndexByID)
			toolCallIndexByID[callID] = idx
		}
		if strings.TrimSpace(name) != "" {
			toolCallNameByID[callID] = strings.TrimSpace(name)
		}
		name = toolCallNameByID[callID]

		fn := map[string]any{
			"arguments": argsDelta,
		}
		if name != "" && !toolCallNameSent[callID] {
			fn["name"] = name
			toolCallNameSent[callID] = true
		}

		tool := map[string]any{
			"index":    idx,
			"id":       callID,
			"type":     "function",
			"function": fn,
		}
		chunk := map[string]any{
			"id":      ensureChatID(""),
			"object":  "chat.completion.chunk",
			"created": nowCreated(),
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{tool},
					},
				},
			},
		}
		if strings.TrimSpace(model) != "" {
			chunk["model"] = strings.TrimSpace(model)
		}
		if err := writeSSEDataJSON(w, chunk); err != nil {
			return err
		}
		sawToolCall = true
		return nil
	}

	flush := func() error {
		if len(curData) == 0 {
			curEvent = ""
			return nil
		}
		payload := bytes.TrimSpace(bytes.Join(curData, []byte{'\n'}))
		curData = curData[:0]
		ev := strings.TrimSpace(curEvent)
		curEvent = ""

		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}

		// Parse JSON payload.
		var obj any
		if err := json.Unmarshal(payload, &obj); err != nil {
			// Ignore malformed event payloads.
			return nil
		}
		root, _ := obj.(map[string]any)
		if root == nil {
			return nil
		}

		// Determine event name (some servers omit "event:" lines and only send "type" in payload).
		eventName := strings.TrimSpace(ev)
		if eventName == "" {
			eventName = strings.TrimSpace(coerceString(root["type"]))
		}

		// Many events carry response id/model at top-level or within "response".
		if rid := strings.TrimSpace(coerceString(root["response_id"])); rid != "" {
			ensureChatID(rid)
		}
		if rid := strings.TrimSpace(coerceString(root["id"])); rid != "" && strings.HasPrefix(strings.ToLower(eventName), "response.") {
			ensureChatID(rid)
		}
		if m := strings.TrimSpace(coerceString(root["model"])); m != "" {
			model = m
		}
		if ca := coerceInt64(root["created_at"]); ca > 0 {
			created = ca
		}

		switch strings.ToLower(strings.TrimSpace(eventName)) {
		case "response.created":
			if resp, ok := root["response"].(map[string]any); ok && resp != nil {
				if m := strings.TrimSpace(coerceString(resp["model"])); m != "" {
					model = m
				}
				if ca := coerceInt64(resp["created_at"]); ca > 0 {
					created = ca
				}
				if rid := strings.TrimSpace(coerceString(resp["id"])); rid != "" {
					ensureChatID(rid)
				}
			}

		case "response.output_text.delta":
			if d := strings.TrimSpace(coerceString(root["delta"])); d != "" {
				return sendTextDelta(d)
			}

		case "response.output_item.added", "response.output_item.done":
			item, _ := root["item"].(map[string]any)
			if item == nil {
				break
			}
			if strings.TrimSpace(coerceString(item["type"])) != "function_call" {
				break
			}
			itemID := strings.TrimSpace(coerceString(item["id"]))
			callID := strings.TrimSpace(coerceString(item["call_id"]))
			if callID == "" {
				callID = itemID
			}
			if itemID != "" && callID != "" {
				toolCallCanonicalIDByItemID[itemID] = callID
			}
			name := strings.TrimSpace(coerceString(item["name"]))
			if name != "" {
				toolCallNameByID[callID] = name
			}
			newArgs := coerceString(item["arguments"])
			prevArgs := toolCallArgsByID[callID]
			argsDelta := ""
			if newArgs != "" {
				if strings.HasPrefix(newArgs, prevArgs) {
					argsDelta = newArgs[len(prevArgs):]
				} else {
					argsDelta = newArgs
				}
				toolCallArgsByID[callID] = newArgs
			}
			return sendToolCallDelta(callID, name, argsDelta)

		case "response.function_call_arguments.delta":
			itemID := strings.TrimSpace(coerceString(root["item_id"]))
			callID := toolCallCanonicalIDByItemID[itemID]
			if callID == "" {
				callID = itemID
			}
			if callID == "" {
				break
			}
			delta := coerceString(root["delta"])
			if delta != "" {
				toolCallArgsByID[callID] += delta
			}
			return sendToolCallDelta(callID, "", delta)

		case "response.function_call_arguments.done":
			// no-op

		case "response.completed":
			// fallthrough to final handling below
		}

		// Final response: either an explicit completed event, or any payload with "response" object.
		// Emit a terminal chunk with finish_reason/usage when possible.
		if strings.Contains(strings.ToLower(eventName), "completed") || root["response"] != nil || strings.TrimSpace(coerceString(root["status"])) != "" {
			full := root
			if inner, ok := root["response"].(map[string]any); ok && inner != nil {
				full = inner
			}
			mapped, err := mapOpenAIResponsesObjectToChat(full)
			if err != nil {
				return nil
			}
			ensureChatID(coerceString(mapped["id"]))
			if m := strings.TrimSpace(coerceString(mapped["model"])); m != "" {
				model = m
			}
			if c := coerceInt64(mapped["created"]); c > 0 {
				created = c
			}

			// Build final chunk using mapped info + streaming context.
			finishReason := ""
			if choices, ok := mapped["choices"].([]any); ok && len(choices) > 0 {
				if cm, ok := choices[0].(map[string]any); ok && cm != nil {
					finishReason = strings.TrimSpace(coerceString(cm["finish_reason"]))
				}
			}
			if strings.TrimSpace(finishReason) == "" {
				finishReason = "stop"
			}
			// new-api aligned: when only tool calls were emitted (no text), use tool_calls.
			if sawToolCall && !sawText {
				finishReason = "tool_calls"
			}
			if err := sendStartIfNeeded(); err != nil {
				return err
			}
			finalChunk := map[string]any{
				"id":      ensureChatID(""),
				"object":  "chat.completion.chunk",
				"created": nowCreated(),
				"choices": []any{
					map[string]any{
						"index":         0,
						"delta":         map[string]any{},
						"finish_reason": finishReason,
					},
				},
			}
			if strings.TrimSpace(model) != "" {
				finalChunk["model"] = strings.TrimSpace(model)
			}
			if u, ok := mapped["usage"].(map[string]any); ok && u != nil {
				finalChunk["usage"] = u
			}
			if err := writeSSEDataJSON(w, finalChunk); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
				return err
			}
		}
		return nil
	}

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			trim := bytes.TrimSpace(line)
			if len(trim) == 0 {
				if err := flush(); err != nil {
					return err
				}
			} else if bytes.HasPrefix(trim, []byte("event:")) {
				curEvent = strings.TrimSpace(string(bytes.TrimSpace(bytes.TrimPrefix(trim, []byte("event:")))))
			} else if bytes.HasPrefix(trim, []byte("data:")) {
				curData = append(curData, bytes.TrimSpace(bytes.TrimPrefix(trim, []byte("data:"))))
			}
		}

		if err != nil {
			if err == io.EOF {
				// Flush trailing event.
				if err := flush(); err != nil {
					return err
				}
				return nil
			}
			return err
		}
	}
}

func writeSSEDataJSON(w io.Writer, obj any) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, "data: "); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n\n"); err != nil {
		return err
	}
	return nil
}
