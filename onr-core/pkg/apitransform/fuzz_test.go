package apitransform

import (
	"bytes"
	"strings"
	"testing"
)

func FuzzTransformSSEByMode(f *testing.F) {
	for _, seed := range []string{
		"data: [DONE]\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n",
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n",
		"data: {\n",
	} {
		f.Add(seed)
	}

	modes := []string{
		"openai_responses_to_openai_chat_chunks",
		"anthropic_to_openai_chunks",
		"openai_to_anthropic_chunks",
		"openai_to_gemini_chunks",
		"gemini_to_openai_chat_chunks",
	}
	f.Fuzz(func(t *testing.T, input string) {
		for _, mode := range modes {
			var output bytes.Buffer
			if err := TransformSSEByMode(mode, strings.NewReader(input), &output); err != nil {
				continue
			}
			for _, line := range strings.Split(output.String(), "\n") {
				if line != "" && !strings.HasPrefix(line, "data: ") {
					t.Fatalf("mode %q wrote a non-SSE data line %q", mode, line)
				}
			}
		}
	})
}
