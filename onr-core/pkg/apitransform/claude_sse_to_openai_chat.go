package apitransform

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE converts Anthropic /v1/messages SSE
// into OpenAI chat.completions SSE chunks and appends a final data: [DONE].
func TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(r io.Reader, w io.Writer) error {
	s := &claudeSSEToChatState{
		w:       w,
		created: time.Now().Unix(),
		chatID:  "chatcmpl_" + strconv.FormatInt(time.Now().UnixNano(), 10),
	}
	p := &sseEventParser{}

	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if ev, ok, perr := p.FeedLine(line); perr != nil {
				return perr
			} else if ok && ev != nil {
				if err := s.handleEvent(ev); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				if ev, ok := p.Flush(); ok && ev != nil {
					if err := s.handleEvent(ev); err != nil {
						return err
					}
				}
				return s.emitDone()
			}
			return err
		}
	}
}

type claudeSSEToChatState struct {
	w io.Writer

	chatID  string
	created int64
	model   string

	roleSent      bool
	doneSent      bool
	toolCallByIdx map[int]claudeStreamToolCall
}

type claudeStreamToolCall struct {
	id   string
	name string
}

func (s *claudeSSEToChatState) handleEvent(ev *sseEvent) error {
	if s == nil || ev == nil {
		return nil
	}
	var anyRoot any
	if err := json.Unmarshal(ev.Data, &anyRoot); err != nil {
		return nil
	}
	root, _ := anyRoot.(map[string]any)
	if root == nil {
		return nil
	}

	eventName := strings.ToLower(strings.TrimSpace(ev.Event))
	if eventName == "" {
		eventName = strings.ToLower(strings.TrimSpace(jsonutil.CoerceString(root["type"])))
	}

	switch eventName {
	case "message_start":
		return s.handleMessageStart(root)
	case "content_block_start":
		return s.handleContentBlockStart(root)
	case "content_block_delta":
		return s.handleContentBlockDelta(root)
	case "message_delta":
		return s.handleMessageDelta(root)
	case "message_stop":
		return s.emitDone()
	default:
		return nil
	}
}

func (s *claudeSSEToChatState) handleMessageStart(root map[string]any) error {
	msg, _ := root["message"].(map[string]any)
	if msg == nil {
		return nil
	}
	if id := strings.TrimSpace(jsonutil.CoerceString(msg["id"])); id != "" {
		s.chatID = normalizeChatCompletionID(id)
	}
	if model := strings.TrimSpace(jsonutil.CoerceString(msg["model"])); model != "" {
		s.model = model
	}
	return s.emitRole()
}

func (s *claudeSSEToChatState) handleContentBlockStart(root map[string]any) error {
	contentBlock, _ := root["content_block"].(map[string]any)
	if contentBlock == nil {
		return nil
	}
	if strings.TrimSpace(jsonutil.CoerceString(contentBlock["type"])) != claudeContentTypeToolUse {
		return nil
	}
	if err := s.emitRole(); err != nil {
		return err
	}
	idx := jsonutil.CoerceInt(root["index"])
	id := strings.TrimSpace(jsonutil.CoerceString(contentBlock["id"]))
	name := strings.TrimSpace(jsonutil.CoerceString(contentBlock["name"]))
	if name == "" {
		return nil
	}

	if s.toolCallByIdx == nil {
		s.toolCallByIdx = map[int]claudeStreamToolCall{}
	}
	s.toolCallByIdx[idx] = claudeStreamToolCall{
		id:   id,
		name: name,
	}

	choice := apitypes.JSONObject{
		"index": 0,
		"delta": apitypes.JSONObject{
			"tool_calls": []any{
				apitypes.JSONObject{
					"index": idx,
					"id":    id,
					"type":  chatRoleFunction,
					"function": apitypes.JSONObject{
						"name":      name,
						"arguments": "",
					},
				},
			},
		},
	}
	return s.emitChunk([]any{choice})
}

func (s *claudeSSEToChatState) handleContentBlockDelta(root map[string]any) error {
	delta, _ := root["delta"].(map[string]any)
	if delta == nil {
		return nil
	}
	if err := s.emitRole(); err != nil {
		return err
	}
	switch strings.TrimSpace(jsonutil.CoerceString(delta["type"])) {
	case "text_delta":
		text := jsonutil.CoerceString(delta["text"])
		if strings.TrimSpace(text) == "" {
			return nil
		}
		choice := apitypes.JSONObject{
			"index": jsonutil.CoerceInt(root["index"]),
			"delta": apitypes.JSONObject{
				"content": text,
			},
		}
		return s.emitChunk([]any{choice})
	case "input_json_delta":
		partial := jsonutil.CoerceString(delta["partial_json"])
		if partial == "" {
			return nil
		}
		idx := jsonutil.CoerceInt(root["index"])
		tool := s.toolCallByIdx[idx]
		tc := apitypes.JSONObject{
			"index": idx,
			"function": apitypes.JSONObject{
				"arguments": partial,
			},
		}
		if tool.id != "" {
			tc["id"] = tool.id
			tc["type"] = chatRoleFunction
		}
		choice := apitypes.JSONObject{
			"index": 0,
			"delta": apitypes.JSONObject{
				"tool_calls": []any{tc},
			},
		}
		return s.emitChunk([]any{choice})
	default:
		return nil
	}
}

func (s *claudeSSEToChatState) handleMessageDelta(root map[string]any) error {
	delta, _ := root["delta"].(map[string]any)
	if delta == nil {
		return nil
	}
	stopReason := strings.TrimSpace(jsonutil.CoerceString(delta["stop_reason"]))
	if stopReason == "" {
		return nil
	}
	finish := mapClaudeStopToOpenAIFinish(stopReason)
	choice := apitypes.JSONObject{
		"index": 0,
		"delta": apitypes.JSONObject{},
	}
	if finish != "" {
		choice["finish_reason"] = finish
	}
	return s.emitChunk([]any{choice})
}

func (s *claudeSSEToChatState) emitRole() error {
	if s.roleSent {
		return nil
	}
	s.roleSent = true
	choice := apitypes.JSONObject{
		"index": 0,
		"delta": apitypes.JSONObject{
			"role": openAIRoleAssistant,
		},
	}
	return s.emitChunk([]any{choice})
}

func (s *claudeSSEToChatState) emitChunk(choices []any) error {
	chunk := apitypes.JSONObject{
		"id":      s.chatID,
		"object":  "chat.completion.chunk",
		"created": s.created,
		"choices": choices,
	}
	if strings.TrimSpace(s.model) != "" {
		chunk["model"] = s.model
	}
	return writeSSEDataJSON(s.w, chunk)
}

func (s *claudeSSEToChatState) emitDone() error {
	if s.doneSent {
		return nil
	}
	s.doneSent = true
	if _, err := io.WriteString(s.w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("write done: %w", err)
	}
	return nil
}

func mapClaudeStopToOpenAIFinish(stop string) string {
	switch strings.TrimSpace(stop) {
	case claudeStopReasonMax:
		return finishReasonLength
	case claudeContentTypeToolUse:
		return finishReasonToolCalls
	case "stop_sequence":
		return "content_filter"
	case "pause_turn", "refusal":
		return finishReasonStop
	default:
		return finishReasonStop
	}
}
