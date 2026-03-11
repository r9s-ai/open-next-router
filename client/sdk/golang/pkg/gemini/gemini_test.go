package gemini

import (
	"bytes"
	"testing"

	"google.golang.org/genai"
)

func TestResponseEvents(t *testing.T) {
	img := []byte{1, 2, 3}
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "hello"},
						{InlineData: &genai.Blob{Data: img, MIMEType: "image/png"}},
					},
				},
			},
		},
	}

	events := responseEvents(resp)
	if len(events) != 2 {
		t.Fatalf("len(events) = %d", len(events))
	}
	if events[0].Type != "text" || events[0].Text != "hello" {
		t.Fatalf("unexpected text event: %+v", events[0])
	}
	if events[1].Type != "image" || events[1].ImageMIMEType != "image/png" || !bytes.Equal(events[1].ImageBytes, img) {
		t.Fatalf("unexpected image event: %+v", events[1])
	}
}
