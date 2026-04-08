package streamtext

import (
	"strings"
	"testing"
)

func TestExtractDeltaText(t *testing.T) {
	tests := []struct {
		name    string
		api     string
		payload string
		want    string
	}{
		{
			name:    "chat completions content and reasoning",
			api:     "chat.completions",
			payload: `{"choices":[{"delta":{"content":"Hi","reasoning_content":" there"}}]}`,
			want:    "Hi there",
		},
		{
			name:    "responses delta",
			api:     "responses",
			payload: `{"delta":"hello"}`,
			want:    "hello",
		},
		{
			name:    "claude delta text",
			api:     "claude.messages",
			payload: `{"delta":{"text":"bonjour"}}`,
			want:    "bonjour",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractDeltaText(tc.api, []byte(tc.payload)); got != tc.want {
				t.Fatalf("ExtractDeltaText()=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestExtractFromSSE_ChatCompletions(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"id":"x","choices":[{"delta":{"content":"hel"}}]}`,
		"",
		`data: {"id":"x","choices":[{"delta":{"content":"lo"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	if got := strings.ReplaceAll(ExtractFromSSE("chat.completions", []byte(sse), 1024), "\n", ""); got != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}
