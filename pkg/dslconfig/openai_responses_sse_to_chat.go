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

const (
	responsesFunctionCallType = "function_call"
	finishReasonStop          = "stop"
)

// TransformOpenAIResponsesSSEToChatCompletionsSSE converts OpenAI Responses SSE stream into
// OpenAI Chat Completions "data: {...}\n\n" chunks, ending with "data: [DONE]\n\n".
//
// Best-effort behavior:
//   - "response.output_text.delta" -> chat delta.content
//   - Tool call events -> chat delta.tool_calls (arguments deltas), aligned with new-api behavior
//   - "response.completed" (or payload containing "response") emits a terminal chunk + [DONE]
func TransformOpenAIResponsesSSEToChatCompletionsSSE(r io.Reader, w io.Writer) error {
	s := &responsesSSEToChatState{w: w}
	p := &sseEventParser{}

	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if ev, ok, perr := p.FeedLine(line); perr != nil {
				return perr
			} else if ok && ev != nil {
				if err := s.HandleEvent(ev); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				// Flush trailing buffered event.
				if ev, ok := p.Flush(); ok && ev != nil {
					if err := s.HandleEvent(ev); err != nil {
						return err
					}
				}
				return nil
			}
			return err
		}
	}
}

type sseEvent struct {
	Event string
	Data  []byte
}

type sseEventParser struct {
	curEvent string
	curData  [][]byte
}

func (p *sseEventParser) FeedLine(line []byte) (*sseEvent, bool, error) {
	if p == nil {
		return nil, false, nil
	}
	trim := bytes.TrimSpace(line)
	if len(trim) == 0 {
		ev, ok := p.Flush()
		return ev, ok, nil
	}
	if bytes.HasPrefix(trim, []byte("event:")) {
		p.curEvent = strings.TrimSpace(string(bytes.TrimSpace(bytes.TrimPrefix(trim, []byte("event:")))))
		return nil, false, nil
	}
	if bytes.HasPrefix(trim, []byte("data:")) {
		p.curData = append(p.curData, bytes.TrimSpace(bytes.TrimPrefix(trim, []byte("data:"))))
		return nil, false, nil
	}
	return nil, false, nil
}

func (p *sseEventParser) Flush() (*sseEvent, bool) {
	if p == nil {
		return nil, false
	}
	if len(p.curData) == 0 {
		p.curEvent = ""
		return nil, false
	}
	payload := bytes.TrimSpace(bytes.Join(p.curData, []byte{'\n'}))
	ev := strings.TrimSpace(p.curEvent)
	p.curData = p.curData[:0]
	p.curEvent = ""

	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil, false
	}
	return &sseEvent{Event: ev, Data: payload}, true
}

type responsesSSEToChatState struct {
	w io.Writer

	chatID  string
	created int64
	model   string

	sentStart   bool
	sawText     bool
	sawToolCall bool

	toolCallIndexByID           map[string]int
	toolCallNameByID            map[string]string
	toolCallArgsByID            map[string]string
	toolCallNameSent            map[string]bool
	toolCallCanonicalIDByItemID map[string]string
}

func (s *responsesSSEToChatState) HandleEvent(ev *sseEvent) error {
	if s == nil || ev == nil {
		return nil
	}
	root, ok := s.parseEventJSON(ev.Data)
	if !ok || root == nil {
		return nil
	}
	eventName := s.resolveEventName(ev.Event, root)
	s.updateCommonFields(eventName, root)

	if handled, err := s.handleTypedEvent(strings.ToLower(strings.TrimSpace(eventName)), root); handled {
		return err
	}

	// Final response: either an explicit completed event, or any payload with "response" object / status.
	if s.shouldEmitFinal(eventName, root) {
		return s.emitFinalFrom(root)
	}
	return nil
}

func (s *responsesSSEToChatState) parseEventJSON(payload []byte) (map[string]any, bool) {
	var obj any
	if err := json.Unmarshal(payload, &obj); err != nil {
		// Ignore malformed event payloads.
		return nil, false
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return nil, false
	}
	return root, true
}

func (s *responsesSSEToChatState) resolveEventName(eventLine string, root map[string]any) string {
	if strings.TrimSpace(eventLine) != "" {
		return strings.TrimSpace(eventLine)
	}
	return strings.TrimSpace(coerceString(root["type"]))
}

func (s *responsesSSEToChatState) updateCommonFields(eventName string, root map[string]any) {
	if s == nil || root == nil {
		return
	}

	if s.toolCallIndexByID == nil {
		s.toolCallIndexByID = map[string]int{}
		s.toolCallNameByID = map[string]string{}
		s.toolCallArgsByID = map[string]string{}
		s.toolCallNameSent = map[string]bool{}
		s.toolCallCanonicalIDByItemID = map[string]string{}
	}

	if rid := strings.TrimSpace(coerceString(root["response_id"])); rid != "" {
		s.ensureChatID(rid)
	}
	if rid := strings.TrimSpace(coerceString(root["id"])); rid != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(eventName)), "response.") {
		s.ensureChatID(rid)
	}
	if m := strings.TrimSpace(coerceString(root["model"])); m != "" {
		s.model = m
	}
	if ca := coerceInt64(root["created_at"]); ca > 0 {
		s.created = ca
	}
}

func (s *responsesSSEToChatState) handleTypedEvent(eventNameLower string, root map[string]any) (bool, error) {
	switch eventNameLower {
	case "response.created":
		s.handleResponseCreated(root)
		return true, nil
	case "response.output_text.delta":
		if d := strings.TrimSpace(coerceString(root["delta"])); d != "" {
			return true, s.sendTextDelta(d)
		}
		return true, nil
	case "response.output_item.added", "response.output_item.done":
		return true, s.handleOutputItemFunctionCall(root)
	case "response.function_call_arguments.delta":
		return true, s.handleFunctionCallArgsDelta(root)
	case "response.function_call_arguments.done":
		return true, nil
	case "response.completed":
		return false, nil // allow final emit below
	default:
		return false, nil
	}
}

func (s *responsesSSEToChatState) handleResponseCreated(root map[string]any) {
	resp, _ := root["response"].(map[string]any)
	if resp == nil {
		return
	}
	if m := strings.TrimSpace(coerceString(resp["model"])); m != "" {
		s.model = m
	}
	if ca := coerceInt64(resp["created_at"]); ca > 0 {
		s.created = ca
	}
	if rid := strings.TrimSpace(coerceString(resp["id"])); rid != "" {
		s.ensureChatID(rid)
	}
}

func (s *responsesSSEToChatState) handleOutputItemFunctionCall(root map[string]any) error {
	item, _ := root["item"].(map[string]any)
	if item == nil {
		return nil
	}
	if strings.TrimSpace(coerceString(item["type"])) != responsesFunctionCallType {
		return nil
	}

	itemID := strings.TrimSpace(coerceString(item["id"]))
	callID := strings.TrimSpace(coerceString(item["call_id"]))
	if callID == "" {
		callID = itemID
	}
	if itemID != "" && callID != "" {
		s.toolCallCanonicalIDByItemID[itemID] = callID
	}

	name := strings.TrimSpace(coerceString(item["name"]))
	if name != "" {
		s.toolCallNameByID[callID] = name
	}

	newArgs := coerceString(item["arguments"])
	prevArgs := s.toolCallArgsByID[callID]
	argsDelta := argsDeltaFromPrefix(prevArgs, newArgs)
	if newArgs != "" {
		s.toolCallArgsByID[callID] = newArgs
	}
	return s.sendToolCallDelta(callID, name, argsDelta)
}

func (s *responsesSSEToChatState) handleFunctionCallArgsDelta(root map[string]any) error {
	itemID := strings.TrimSpace(coerceString(root["item_id"]))
	callID := s.toolCallCanonicalIDByItemID[itemID]
	if callID == "" {
		callID = itemID
	}
	if callID == "" {
		return nil
	}
	delta := coerceString(root["delta"])
	if delta != "" {
		s.toolCallArgsByID[callID] += delta
	}
	return s.sendToolCallDelta(callID, "", delta)
}

func (s *responsesSSEToChatState) shouldEmitFinal(eventName string, root map[string]any) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(eventName)), "completed") {
		return true
	}
	if root == nil {
		return false
	}
	if root["response"] != nil {
		return true
	}
	if strings.TrimSpace(coerceString(root["status"])) != "" {
		return true
	}
	return false
}

func (s *responsesSSEToChatState) emitFinalFrom(root map[string]any) error {
	full := root
	if inner, ok := root["response"].(map[string]any); ok && inner != nil {
		full = inner
	}

	mapped := mapOpenAIResponsesObjectToChat(full)
	if mapped == nil {
		// Keep best-effort: do not fail the whole stream on mapping errors.
		return nil
	}

	s.ensureChatID(coerceString(mapped["id"]))
	if m := strings.TrimSpace(coerceString(mapped["model"])); m != "" {
		s.model = m
	}
	if c := coerceInt64(mapped["created"]); c > 0 {
		s.created = c
	}

	finishReason := s.finishReasonFromMapped(mapped)
	if err := s.sendStartIfNeeded(); err != nil {
		return err
	}

	finalChunk := map[string]any{
		"id":      s.ensureChatID(""),
		"object":  "chat.completion.chunk",
		"created": s.nowCreated(),
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": finishReason,
			},
		},
	}
	if strings.TrimSpace(s.model) != "" {
		finalChunk["model"] = strings.TrimSpace(s.model)
	}
	if u, ok := mapped["usage"].(map[string]any); ok && u != nil {
		finalChunk["usage"] = u
	}
	if err := writeSSEDataJSON(s.w, finalChunk); err != nil {
		return err
	}
	if _, err := io.WriteString(s.w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	return nil
}

func (s *responsesSSEToChatState) finishReasonFromMapped(mapped map[string]any) string {
	finishReason := ""
	if choices, ok := mapped["choices"].([]any); ok && len(choices) > 0 {
		if cm, ok := choices[0].(map[string]any); ok && cm != nil {
			finishReason = strings.TrimSpace(coerceString(cm["finish_reason"]))
		}
	}
	if strings.TrimSpace(finishReason) == "" {
		finishReason = finishReasonStop
	}
	// new-api aligned: when only tool calls were emitted (no text), use tool_calls.
	if s.sawToolCall && !s.sawText {
		finishReason = "tool_calls"
	}
	return finishReason
}

func (s *responsesSSEToChatState) nowCreated() int64 {
	if s != nil && s.created > 0 {
		return s.created
	}
	return time.Now().Unix()
}

func (s *responsesSSEToChatState) ensureChatID(seed string) string {
	if strings.TrimSpace(s.chatID) != "" {
		return s.chatID
	}
	id := strings.TrimSpace(seed)
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if !strings.HasPrefix(id, "chatcmpl_") {
		id = "chatcmpl_" + id
	}
	s.chatID = id
	return s.chatID
}

func (s *responsesSSEToChatState) sendStartIfNeeded() error {
	if s.sentStart {
		return nil
	}
	chunk := map[string]any{
		"id":      s.ensureChatID(""),
		"object":  "chat.completion.chunk",
		"created": s.nowCreated(),
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{},
			},
		},
	}
	if strings.TrimSpace(s.model) != "" {
		chunk["model"] = strings.TrimSpace(s.model)
	}
	if err := writeSSEDataJSON(s.w, chunk); err != nil {
		return err
	}
	s.sentStart = true
	return nil
}

func (s *responsesSSEToChatState) sendTextDelta(delta string) error {
	if strings.TrimSpace(delta) == "" {
		return nil
	}
	s.sawText = true
	chunk := map[string]any{
		"id":      s.ensureChatID(""),
		"object":  "chat.completion.chunk",
		"created": s.nowCreated(),
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{"content": delta},
			},
		},
	}
	if strings.TrimSpace(s.model) != "" {
		chunk["model"] = strings.TrimSpace(s.model)
	}
	if err := writeSSEDataJSON(s.w, chunk); err != nil {
		return err
	}
	s.sentStart = true
	return nil
}

func (s *responsesSSEToChatState) sendToolCallDelta(callID string, name string, argsDelta string) error {
	if strings.TrimSpace(callID) == "" {
		return nil
	}
	if s.sawText {
		// Prefer streaming assistant text over tool calls to match non-stream behavior.
		return nil
	}
	if err := s.sendStartIfNeeded(); err != nil {
		return err
	}

	idx, ok := s.toolCallIndexByID[callID]
	if !ok {
		idx = len(s.toolCallIndexByID)
		s.toolCallIndexByID[callID] = idx
	}
	if strings.TrimSpace(name) != "" {
		s.toolCallNameByID[callID] = strings.TrimSpace(name)
	}
	name = s.toolCallNameByID[callID]

	fn := map[string]any{
		"arguments": argsDelta,
	}
	if name != "" && !s.toolCallNameSent[callID] {
		fn["name"] = name
		s.toolCallNameSent[callID] = true
	}
	tool := map[string]any{
		"index":    idx,
		"id":       callID,
		"type":     "function",
		"function": fn,
	}
	chunk := map[string]any{
		"id":      s.ensureChatID(""),
		"object":  "chat.completion.chunk",
		"created": s.nowCreated(),
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []any{tool},
				},
			},
		},
	}
	if strings.TrimSpace(s.model) != "" {
		chunk["model"] = strings.TrimSpace(s.model)
	}
	if err := writeSSEDataJSON(s.w, chunk); err != nil {
		return err
	}
	s.sawToolCall = true
	return nil
}

func argsDeltaFromPrefix(prev, next string) string {
	if strings.TrimSpace(next) == "" {
		return ""
	}
	if strings.HasPrefix(next, prev) {
		return next[len(prev):]
	}
	return next
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
