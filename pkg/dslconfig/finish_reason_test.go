package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
)

func TestExtractFinishReason_OpenAI(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions"}
	body := []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "openai"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "stop" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_Anthropic(t *testing.T) {
	const finishReasonEndTurn = "end_turn"
	meta := &dslmeta.Meta{API: "claude.messages"}
	body := []byte(`{"stop_reason":"` + finishReasonEndTurn + `"}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "anthropic"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != finishReasonEndTurn {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_AnthropicStreamDelta(t *testing.T) {
	const finishReasonEndTurn = "end_turn"
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	body := []byte(`{"type":"message_delta","delta":{"stop_reason":"` + finishReasonEndTurn + `","stop_sequence":null},"usage":{"output_tokens":12}}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "anthropic"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != finishReasonEndTurn {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_Gemini(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.generateContent"}
	body := []byte(`{"candidates":[{"finishReason":"STOP"}]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "gemini"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "STOP" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_CustomPath(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions"}
	body := []byte(`{"x":{"y":[{"z":"length"}]}}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "custom", FinishReasonPath: "$.x.y[0].z"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "length" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}
