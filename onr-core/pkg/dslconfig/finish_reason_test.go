package dslconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
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

func TestExtractFinishReason_OpenAIResponsesIncompleteMaxOutputTokens(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses"}
	body := []byte(`{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "openai"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "max_output_tokens" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_OpenAIResponsesContentFilter(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses"}
	body := []byte(`{"status":"incomplete","incomplete_details":{"reason":"content_filter"},"output":[]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "openai"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "content_filter" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_OpenAIResponsesStreamEnvelope(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	body := []byte(`{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "openai"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "max_output_tokens" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_OpenAIResponsesCompletedReturnsEmpty(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses"}
	body := []byte(`{"status":"completed","output":[]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "openai"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_OpenAIPathOverrideTakesPrecedence(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses"}
	body := []byte(`{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"meta":{"finish":"tool_calls"}}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{
		Mode:             "openai",
		FinishReasonPath: "$.meta.finish",
	}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "tool_calls" {
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

func TestExtractFinishReason_AnthropicStreamMessageStartFallback(t *testing.T) {
	const finishReasonMaxTokens = "max_tokens"
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	body := []byte(`{"type":"message_start","message":{"stop_reason":"` + finishReasonMaxTokens + `"}}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "anthropic"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != finishReasonMaxTokens {
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

func TestExtractFinishReason_GeminiSnakeCaseFallback(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.generateContent"}
	body := []byte(`{"candidates":[{"finish_reason":"SAFETY"}]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "gemini"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "SAFETY" {
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

func TestExtractFinishReason_CustomPathFallback(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	cfg := FinishReasonExtractConfig{Mode: "custom"}
	cfg.addFinishReasonPath("$.delta.stop_reason", false)
	cfg.addFinishReasonPath("$.message.stop_reason", true)

	body := []byte(`{"type":"message_start","message":{"stop_reason":"max_tokens"}}`)
	v, err := ExtractFinishReason(meta, cfg, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "max_tokens" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_CustomPathPrimaryTakesPrecedenceOverFallback(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: true}
	cfg := FinishReasonExtractConfig{Mode: "custom"}
	cfg.addFinishReasonPath("$.delta.stop_reason", false)
	cfg.addFinishReasonPath("$.message.stop_reason", true)

	body := []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"message":{"stop_reason":"max_tokens"}}`)
	v, err := ExtractFinishReason(meta, cfg, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "end_turn" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestExtractFinishReason_CustomWithoutPathReturnsEmpty(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions"}
	body := []byte(`{"choices":[{"finish_reason":"stop"}]}`)
	v, err := ExtractFinishReason(meta, FinishReasonExtractConfig{Mode: "custom"}, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "" {
		t.Fatalf("unexpected finish_reason: %q", v)
	}
}

func TestValidateProviderFile_FinishReasonPathFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom_finish.conf")
	content := []byte(`
syntax "next-router/0.1";

provider "custom_finish" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_extract custom;
      finish_reason_path "$.delta.stop_reason";
      finish_reason_path "$.message.stop_reason" fallback=true;
    }
  }
}
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	cfg, ok := pf.Finish.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected finish_reason config selected")
	}
	rules := cfg.finishReasonPathConfigs()
	if got, want := len(rules), 2; got != want {
		t.Fatalf("path rules len=%d want=%d", got, want)
	}
	if rules[0].Path != "$.delta.stop_reason" || rules[0].Fallback {
		t.Fatalf("unexpected first rule: %+v", rules[0])
	}
	if rules[1].Path != "$.message.stop_reason" || !rules[1].Fallback {
		t.Fatalf("unexpected second rule: %+v", rules[1])
	}
}

func TestValidateProviderFile_FinishReasonPathFallbackRejectsInvalidPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid_finish.conf")
	content := []byte(`
syntax "next-router/0.1";

provider "invalid_finish" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_extract custom;
      finish_reason_path "delta.stop_reason" fallback=true;
    }
  }
}
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected invalid finish_reason_path to be rejected")
	}
}
