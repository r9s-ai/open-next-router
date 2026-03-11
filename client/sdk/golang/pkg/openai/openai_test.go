package openai

import (
	"testing"

	openaiapi "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

func TestChatCompletionText(t *testing.T) {
	resp := &openaiapi.ChatCompletion{
		Choices: []openaiapi.ChatCompletionChoice{
			{Message: openaiapi.ChatCompletionMessage{Content: "hello"}},
		},
	}
	if got := chatCompletionText(resp); got != "hello" {
		t.Fatalf("chatCompletionText = %q", got)
	}
}

func TestChatCompletionChunkText(t *testing.T) {
	chunk := openaiapi.ChatCompletionChunk{
		Choices: []openaiapi.ChatCompletionChunkChoice{
			{Delta: openaiapi.ChatCompletionChunkChoiceDelta{Content: "he"}},
		},
	}
	if got := chatCompletionChunkText(chunk); got != "he" {
		t.Fatalf("chatCompletionChunkText = %q", got)
	}
}

func TestResponseDeltaText(t *testing.T) {
	event := responses.ResponseStreamEventUnion{
		Type:  "response.output_text.delta",
		Delta: "llo",
	}
	if got := responseDeltaText(event); got != "llo" {
		t.Fatalf("responseDeltaText = %q", got)
	}
}

func TestEmbeddingResultFromResponse(t *testing.T) {
	resp := &openaiapi.CreateEmbeddingResponse{
		Data: []openaiapi.Embedding{
			{Embedding: []float64{1, 2, 3}},
		},
		Usage: openaiapi.CreateEmbeddingResponseUsage{
			PromptTokens: 5,
			TotalTokens:  8,
		},
	}

	got := embeddingResultFromResponse(resp, "text-embedding-3-small")
	if got.Dimensions != 3 {
		t.Fatalf("Dimensions = %d", got.Dimensions)
	}
	if got.PromptTokens != 5 || got.TotalTokens != 8 {
		t.Fatalf("usage = %+v", got)
	}
}
