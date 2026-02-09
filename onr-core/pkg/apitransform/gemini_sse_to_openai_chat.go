package apitransform

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// TransformGeminiSSEToOpenAIChatCompletionsSSE converts Gemini SSE responses into
// OpenAI chat.completions SSE chunks and appends a final data: [DONE].
func TransformGeminiSSEToOpenAIChatCompletionsSSE(r io.Reader, w io.Writer) error {
	s := &geminiSSEToChatState{
		w:         w,
		chatID:    "chatcmpl_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		created:   time.Now().Unix(),
		roleByIdx: map[int]bool{},
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

	roleByIdx map[int]bool
}

func (s *geminiSSEToChatState) handlePayload(payload []byte) error {
	root, err := apitypes.ParseJSONObject(payload, "gemini stream event")
	if err != nil {
		return nil
	}
	if m := strings.TrimSpace(jsonutil.CoerceString(root["modelVersion"])); m != "" {
		s.model = m
	} else if m := strings.TrimSpace(jsonutil.CoerceString(root["model"])); m != "" {
		s.model = m
	}

	candidates, _ := root["candidates"].([]any)
	if len(candidates) > 0 {
		for i, raw := range candidates {
			cand, _ := raw.(map[string]any)
			if cand == nil {
				continue
			}
			idx := jsonutil.CoerceInt(cand["index"])
			if idx < 0 {
				idx = i
			}

			content, _ := cand["content"].(map[string]any)
			parts, _ := content["parts"].([]any)
			text := geminiPartsToText(parts)
			rawFinish := strings.TrimSpace(jsonutil.CoerceString(cand["finishReason"]))
			finish := ""
			if rawFinish != "" {
				finish = mapGeminiFinishToOpenAI(rawFinish)
			}

			delta := apitypes.JSONObject{}
			if !s.roleByIdx[idx] {
				delta["role"] = openAIRoleAssistant
				s.roleByIdx[idx] = true
			}
			if text != "" {
				delta["content"] = text
			}
			if len(delta) == 0 && finish == "" {
				continue
			}

			choice := apitypes.JSONObject{
				"index": idx,
				"delta": delta,
			}
			if finish != "" {
				choice["finish_reason"] = finish
			}
			chunk := apitypes.JSONObject{
				"id":      s.chatID,
				"object":  "chat.completion.chunk",
				"created": s.created,
				"choices": []any{choice},
			}
			if s.model != "" {
				chunk["model"] = s.model
			}
			if err := writeSSEDataJSON(s.w, chunk); err != nil {
				return err
			}
		}
	}

	if usage, _ := root["usageMetadata"].(map[string]any); usage != nil {
		p := jsonutil.CoerceInt(usage["promptTokenCount"])
		c := jsonutil.CoerceInt(usage["candidatesTokenCount"])
		t := jsonutil.CoerceInt(usage["totalTokenCount"])
		if t == 0 {
			t = p + c
		}
		chunk := apitypes.JSONObject{
			"id":      s.chatID,
			"object":  "chat.completion.chunk",
			"created": s.created,
			"choices": []any{},
			"usage": apitypes.JSONObject{
				"prompt_tokens":     p,
				"completion_tokens": c,
				"total_tokens":      t,
			},
		}
		if s.model != "" {
			chunk["model"] = s.model
		}
		if err := writeSSEDataJSON(s.w, chunk); err != nil {
			return err
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

func geminiPartsToText(parts []any) string {
	var b strings.Builder
	for _, raw := range parts {
		p, _ := raw.(map[string]any)
		if p == nil {
			continue
		}
		if t := jsonutil.CoerceString(p["text"]); t != "" {
			b.WriteString(t)
		}
	}
	return b.String()
}
