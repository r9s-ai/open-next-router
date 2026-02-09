package apitransform

import (
	"bufio"
	"bytes"
	"io"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
)

// TransformOpenAIChatCompletionsSSEToClaudeMessagesSSE converts OpenAI chat SSE chunks into Claude-style SSE events.
func TransformOpenAIChatCompletionsSSEToClaudeMessagesSSE(r io.Reader, w io.Writer) error {
	return transformOpenAIChatSSE(r, func(payload []byte) ([][]byte, error) {
		events, err := MapOpenAIChatCompletionsChunkToClaudeEventsObject(bytesToObject(payload))
		if err != nil {
			return nil, err
		}
		out := make([][]byte, 0, len(events))
		for _, ev := range events {
			b, err := ev.Marshal()
			if err != nil {
				return nil, err
			}
			out = append(out, b)
		}
		return out, nil
	}, w)
}

// TransformOpenAIChatCompletionsSSEToGeminiSSE converts OpenAI chat SSE chunks into Gemini-style SSE responses.
func TransformOpenAIChatCompletionsSSEToGeminiSSE(r io.Reader, w io.Writer) error {
	return transformOpenAIChatSSE(r, func(payload []byte) ([][]byte, error) {
		obj, emit, err := MapOpenAIChatCompletionsChunkToGeminiResponseObject(bytesToObject(payload))
		if err != nil || !emit {
			return nil, err
		}
		b, err := obj.Marshal()
		if err != nil {
			return nil, err
		}
		return [][]byte{b}, nil
	}, w)
}

type ssePayloadMapper func(payload []byte) ([][]byte, error)

func transformOpenAIChatSSE(r io.Reader, mapper ssePayloadMapper, w io.Writer) error {
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

		items, err := mapper(payload)
		if err != nil {
			return err
		}
		for _, item := range items {
			if len(item) == 0 {
				continue
			}
			if _, err := w.Write([]byte("data: ")); err != nil {
				return err
			}
			if _, err := w.Write(item); err != nil {
				return err
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
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
				if ferr := flush(); ferr != nil {
					return ferr
				}
			} else if bytes.HasPrefix(trim, []byte("data:")) {
				dataLines = append(dataLines, bytes.TrimSpace(bytes.TrimPrefix(trim, []byte("data:"))))
			}
		}
		if err != nil {
			if err == io.EOF {
				return flush()
			}
			return err
		}
	}
}

func bytesToObject(payload []byte) map[string]any {
	root, err := apitypes.ParseJSONObject(payload, "openai chat chunk")
	if err != nil {
		return nil
	}
	return root
}
