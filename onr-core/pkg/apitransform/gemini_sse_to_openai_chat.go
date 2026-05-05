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

// TransformGeminiSSEToOpenAIChatCompletionsSSE converts Gemini SSE responses into
// OpenAI chat.completions SSE chunks and appends a final data: [DONE].
func TransformGeminiSSEToOpenAIChatCompletionsSSE(r io.Reader, w io.Writer) error {
	s := &geminiSSEToChatState{
		w:       w,
		chatID:  "chatcmpl_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		created: time.Now().Unix(),
	}

	br := bufio.NewReader(r)
	var dataLines [][]byte
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := bytes.TrimSpace(bytes.Join(dataLines, []byte{'\n'}))
		dataLines = dataLines[:0]
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			return nil
		}
		return s.handlePayload(payload)
	}

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			trim := bytes.TrimSpace(line)
			if len(trim) == 0 {
				if ferr := flush(); ferr != nil {
					return ferr
				}
			} else if bytes.HasPrefix(trim, []byte("data:")) {
				dataLines = append(dataLines, bytes.TrimSpace(bytes.TrimPrefix(trim, []byte("data:"))))
			}
		}
		if err != nil {
			if err == io.EOF {
				if ferr := flush(); ferr != nil {
					return ferr
				}
				return s.emitDone()
			}
			return err
		}
	}
}

type geminiSSEToChatState struct {
	w io.Writer

	chatID  string
	created int64
	model   string
}

func (s *geminiSSEToChatState) handlePayload(payload []byte) error {
	var root apitypes.GenerateContentStreamResponse
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil
	}
	if m := strings.TrimSpace(root.ModelVersion); m != "" {
		s.model = m
	} else if m := strings.TrimSpace(root.Model); m != "" {
		s.model = m
	}

	if len(root.Candidates) > 0 {
		choices := make([]any, 0, len(root.Candidates))
		for i, cand := range root.Candidates {
			idx := cand.Index
			if idx < 0 {
				idx = i
			}
			content, toolCalls := geminiPartsToContentAndToolCalls(cand.Content.Parts)
			delta := apitypes.JSONObject{
				"role": openAIRoleAssistant,
			}
			if content != "" {
				delta["content"] = content
			}
			if len(toolCalls) > 0 {
				delta["tool_calls"] = toolCalls
			}
			finish := strings.TrimSpace(cand.FinishReason)
			if len(delta) == 1 && finish == "" {
				continue
			}
			choice := apitypes.JSONObject{
				"index": idx,
				"delta": delta,
			}
			if finish != "" {
				choice["finish_reason"] = finish
			}
			choices = append(choices, choice)
		}
		if len(choices) > 0 {
			chunk := apitypes.JSONObject{
				"id":      s.chatID,
				"object":  "chat.completion.chunk",
				"created": s.created,
				"choices": choices,
			}
			if s.model != "" {
				chunk["model"] = s.model
			}
			if usage, err := geminiUsageToChatChunkUsage(root.UsageMetadata); err != nil {
				return err
			} else if usage != nil {
				chunk["usage"] = usage
			}
			if err := writeSSEDataJSON(s.w, chunk); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *geminiSSEToChatState) emitDone() error {
	if _, err := io.WriteString(s.w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("write done: %w", err)
	}
	return nil
}

func geminiPartsToContentAndToolCalls(parts []apitypes.Part) (string, []any) {
	var b strings.Builder
	toolCalls := make([]any, 0)
	for idx, part := range parts {
		if part.Text != "" {
			b.WriteString(part.Text)
		}
		if part.FunctionCall != nil && strings.TrimSpace(part.FunctionCall.FunctionName) != "" {
			args := "{}"
			if part.FunctionCall.Arguments != nil {
				if raw, err := json.Marshal(part.FunctionCall.Arguments); err == nil {
					args = string(raw)
				}
			}
			toolCalls = append(toolCalls, apitypes.JSONObject{
				"index": idx,
				"id":    fmt.Sprintf("call_%d_%d", idx, time.Now().UnixNano()),
				"type":  chatRoleFunction,
				"function": apitypes.JSONObject{
					"name":      part.FunctionCall.FunctionName,
					"arguments": args,
				},
			})
		}
		if part.InlineData != nil {
			args, err := json.Marshal(apitypes.JSONObject{
				"mime":        part.InlineData.MimeType,
				"data_base64": part.InlineData.Data,
			})
			if err != nil {
				continue
			}
			toolCalls = append(toolCalls, apitypes.JSONObject{
				"index": idx,
				"id":    fmt.Sprintf("call_media_%d_%d", idx, time.Now().UnixNano()),
				"type":  chatRoleFunction,
				"function": apitypes.JSONObject{
					"name":      "emit_media",
					"arguments": string(args),
				},
			})
		}
	}
	return b.String(), toolCalls
}

func geminiUsageToChatChunkUsage(usage *apitypes.UsageMetadata) (apitypes.JSONObject, error) {
	if usage == nil {
		return nil, nil
	}
	completionTokens := usage.TotalTokenCount - usage.PromptTokenCount
	if completionTokens < 0 {
		completionTokens = 0
	}
	out := apitypes.JSONObject{
		"prompt_tokens":     usage.PromptTokenCount,
		"completion_tokens": completionTokens,
		"total_tokens":      usage.TotalTokenCount,
	}
	out["completion_tokens_details"] = apitypes.JSONObject{
		"reasoning_tokens": usage.ThoughtsTokenCount,
	}
	return out, nil
}
