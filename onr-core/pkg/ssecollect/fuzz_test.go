package ssecollect

import (
	"context"
	"strings"
	"testing"
)

func FuzzParseAndCollect(f *testing.F) {
	for _, seed := range []string{
		"event: ping\ndata: {\"type\":\"ping\"}\n\n",
		"id: 1\r\ndata: [DONE]\r\n\r\n",
		"data: {\"type\":\"message_start\",\"message\":{\"content\":[]}}\n\n",
		"data: {\"candidates\":[]}\n\n",
		"data: {\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		events, err := Parse(context.Background(), strings.NewReader(input))
		if err == nil {
			for _, event := range events {
				if event.ID != strings.TrimSpace(event.ID) {
					t.Fatalf("event ID is not normalized: %q", event.ID)
				}
				if event.Done && strings.TrimSpace(string(event.Data)) != "[DONE]" {
					t.Fatalf("non-DONE event marked Done: %#v", event)
				}
			}
		}

		// Aggregators must reject incomplete or invalid streams with an error rather
		// than panic. They also exercise tool-call and incremental event handling.
		for _, mode := range []string{"openai_responses", "anthropic_messages", "gemini_generate_content"} {
			_, _ = CollectByMode(context.Background(), mode, strings.NewReader(input), Options{})
		}
	})
}
