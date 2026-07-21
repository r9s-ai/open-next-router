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
	toolCallByIdx map[int]apitypes.ClaudeStreamContentBlock
	usage         *apitypes.OpenAIChatCompletionsUsage
}

// handleEvent requires a non-nil transform state and non-nil parsed SSE event.
func (s *claudeSSEToChatState) handleEvent(ev *sseEvent) error {
	var msg apitypes.ClaudeStreamMessage
	if json.Unmarshal(ev.Data, &msg) == nil {
		eventName := strings.ToLower(ev.Event)
		if eventName == "" {
			eventName = strings.ToLower(strings.TrimSpace(msg.Type))
		}

		switch eventName {
		case "message_start":
			return s.handleMessageStart(&msg)
		case "content_block_start":
			return s.handleContentBlockStart(&msg)
		case "content_block_delta":
			return s.handleContentBlockDelta(&msg)
		case "message_delta":
			return s.handleMessageDelta(&msg)
		case "message_stop":
			return s.emitDone()
		}
	}
	return nil
}

func (s *claudeSSEToChatState) handleMessageStart(msg *apitypes.ClaudeStreamMessage) error {
	if msg.Message == nil {
		return nil
	}
	if id := strings.TrimSpace(msg.Message.ID); id != "" {
		s.chatID = normalizeChatCompletionID(id)
	}
	if model := strings.TrimSpace(msg.Message.Model); model != "" {
		s.model = model
	}
	s.mergeUsage(msg.Message.Usage)
	return s.emitRole()
}

func (s *claudeSSEToChatState) handleContentBlockStart(msg *apitypes.ClaudeStreamMessage) error {
	if msg.ContentBlock == nil {
		return nil
	}
	if strings.TrimSpace(msg.ContentBlock.Type) != claudeContentTypeToolUse {
		return nil
	}
	if err := s.emitRole(); err != nil {
		return err
	}
	idx := msg.Index
	id := strings.TrimSpace(msg.ContentBlock.ID)
	name := strings.TrimSpace(msg.ContentBlock.Name)
	if name == "" {
		return nil
	}

	if s.toolCallByIdx == nil {
		s.toolCallByIdx = map[int]apitypes.ClaudeStreamContentBlock{}
	}
	s.toolCallByIdx[idx] = *msg.ContentBlock

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

func (s *claudeSSEToChatState) handleContentBlockDelta(msg *apitypes.ClaudeStreamMessage) error {
	if msg.Delta == nil {
		return nil
	}
	if err := s.emitRole(); err != nil {
		return err
	}
	switch strings.TrimSpace(msg.Delta.Type) {
	case "text_delta":
		text := msg.Delta.Text
		if strings.TrimSpace(text) == "" {
			return nil
		}
		choice := apitypes.JSONObject{
			"index": msg.Index,
			"delta": apitypes.JSONObject{
				"content": text,
			},
		}
		return s.emitChunk([]any{choice})
	case "input_json_delta":
		partial := msg.Delta.PartialJSON
		if partial == "" {
			return nil
		}
		idx := msg.Index
		tool := s.toolCallByIdx[idx]
		tc := apitypes.JSONObject{
			"index": idx,
			"function": apitypes.JSONObject{
				"arguments": partial,
			},
		}
		if id := strings.TrimSpace(tool.ID); id != "" {
			tc["id"] = id
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

func (s *claudeSSEToChatState) handleMessageDelta(msg *apitypes.ClaudeStreamMessage) error {
	s.mergeUsage(msg.Usage)
	if msg.Delta == nil {
		return nil
	}
	stopReason := strings.TrimSpace(msg.Delta.StopReason)
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
	if msg.Delta.StopDetails != nil {
		if sd, err := msg.Delta.StopDetails.ToMap(); err == nil {
			choice["stop_details"] = sd
		}
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
	if err := s.emitUsageChunk(); err != nil {
		return err
	}
	if _, err := io.WriteString(s.w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("write done: %w", err)
	}
	return nil
}

func (s *claudeSSEToChatState) mergeUsage(raw *apitypes.ClaudeUsage) {
	if raw == nil {
		return
	}
	if s.usage == nil {
		s.usage = &apitypes.OpenAIChatCompletionsUsage{}
	}
	u := s.usage
	if raw.InputTokens > 0 {
		u.PromptTokens = raw.InputTokens
	}
	if raw.OutputTokens > 0 {
		u.CompletionTokens = raw.OutputTokens
	}
	if raw.CacheReadInputTokens > 0 {
		if u.PromptTokensDetails == nil {
			u.PromptTokensDetails = &apitypes.OpenAITokenDetails{}
		}
		u.PromptTokensDetails.CachedTokens = raw.CacheReadInputTokens
		u.PromptTokens += raw.CacheReadInputTokens
	}
	if raw.CacheCreationInputTokens > 0 {
		if u.PromptTokensDetails == nil {
			u.PromptTokensDetails = &apitypes.OpenAITokenDetails{}
		}
		u.PromptTokensDetails.CacheWriteTokens = raw.CacheCreationInputTokens
		u.PromptTokens += raw.CacheCreationInputTokens
	}
	if len(raw.Iterations) > 0 {
		u.Iterations = claudeUsageIterationsToOpenAI(raw.Iterations)
	}
}

func claudeUsageIterationsToOpenAI(iterations []apitypes.ClaudeUsageByModel) []*apitypes.OpenAIUsageByModel {
	if len(iterations) == 0 {
		return nil
	}
	out := make([]*apitypes.OpenAIUsageByModel, 0, len(iterations))
	for i := range iterations {
		iteration := iterations[i]
		promptTokens := iteration.InputTokens + iteration.CacheCreationInputTokens + iteration.CacheReadInputTokens
		usage := &apitypes.OpenAIUsageByModel{
			Type:             iteration.Type,
			Model:            iteration.Model,
			PromptTokens:     promptTokens,
			CompletionTokens: iteration.OutputTokens,
			TotalTokens:      promptTokens + iteration.OutputTokens,
		}
		if iteration.CacheReadInputTokens > 0 || iteration.CacheCreationInputTokens > 0 {
			usage.PromptTokenDetails = &apitypes.OpenAITokenDetails{
				CachedTokens:     iteration.CacheReadInputTokens,
				CacheWriteTokens: iteration.CacheCreationInputTokens,
			}
		}
		out = append(out, usage)
	}
	return out
}

func (s *claudeSSEToChatState) emitUsageChunk() error {
	if s.usage == nil {
		return nil
	}
	u := *s.usage
	if u.TotalTokens <= 0 {
		if u.PromptTokens <= 0 && u.CompletionTokens <= 0 {
			return nil
		}
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	usage, err := u.ToMap()
	if err != nil {
		return fmt.Errorf("usage to map: %w", err)
	}
	if len(usage) == 0 {
		return nil
	}
	chunk := apitypes.JSONObject{
		"id":      s.chatID,
		"object":  "chat.completion.chunk",
		"created": s.created,
		"choices": []any{},
		"usage":   usage,
	}
	if strings.TrimSpace(s.model) != "" {
		chunk["model"] = s.model
	}
	// emitDone is single-shot; clear pointer defensively to avoid accidental re-emit.
	s.usage = nil
	return writeSSEDataJSON(s.w, chunk)
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
