package anthropic

import (
	"testing"

	anthropicapi "github.com/anthropics/anthropic-sdk-go"
)

func TestMessageText(t *testing.T) {
	msg := &anthropicapi.Message{
		Content: []anthropicapi.ContentBlockUnion{
			{Type: "text", Text: "hello"},
		},
	}
	if got := messageText(msg); got != "hello" {
		t.Fatalf("messageText = %q", got)
	}
}

func TestContentBlockDeltaText(t *testing.T) {
	event := anthropicapi.ContentBlockDeltaEvent{
		Delta: anthropicapi.RawContentBlockDeltaUnion{
			Type: "text_delta",
			Text: "he",
		},
	}
	if got := contentBlockDeltaText(event); got != "he" {
		t.Fatalf("contentBlockDeltaText = %q", got)
	}
}
