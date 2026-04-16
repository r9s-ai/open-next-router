package dslconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestExtractFinishReason_OpenAI(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions"}
	body := []byte(`{"choices":[{"index":0,"finish_reason":"stop"}]}`)
	_, cfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	cfg := FinishReasonExtractConfig{Mode: "custom", FinishReasonPath: "$.meta.finish"}
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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
	_, cfg := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)
	v, err := ExtractFinishReason(meta, cfg, body)
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

func TestExtractFinishReason_CustomPathEventFilter(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	cfg := FinishReasonExtractConfig{Mode: "custom"}
	cfg.addFinishReasonPathRule("$.response.incomplete_details.reason", false, "response.incomplete", true)
	cfg.addFinishReasonPathRule("$.response.status", true, "response.completed", true)

	body := []byte(`{"type":"response.completed","response":{"status":"completed","incomplete_details":{"reason":"max_output_tokens"}}}`)
	v, err := ExtractFinishReason(meta, cfg, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "max_output_tokens" {
		t.Fatalf("unexpected finish_reason without event context: %q", v)
	}

	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	v, err = extractFinishReasonFromRootWithEvent(meta, cfg, "response.completed", root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "completed" {
		t.Fatalf("unexpected finish_reason with event context: %q", v)
	}
}

func TestExtractFinishReason_CustomPathEventFilter_SSEFramedBodyPrefersEvent(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	cfg := FinishReasonExtractConfig{Mode: "custom"}
	cfg.addFinishReasonPathRule("$.response.incomplete_details.reason", false, "response.incomplete", true)
	cfg.addFinishReasonPathRule("$.response.status", true, "response.completed", true)

	body := []byte(
		"event: response.completed\n" +
			`data: {"type":"response.completed","response":{"status":"completed","incomplete_details":{"reason":"max_output_tokens"}}}` + "\n\n",
	)
	v, err := ExtractFinishReason(meta, cfg, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "completed" {
		t.Fatalf("unexpected finish_reason from SSE body: %q", v)
	}
}

func TestExtractFinishReason_CustomPathEventFilter_InvalidJSONStillErrors(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	cfg := FinishReasonExtractConfig{Mode: "custom"}
	cfg.addFinishReasonPathRule("$.response.status", false, "response.completed", true)

	_, err := ExtractFinishReason(meta, cfg, []byte("eventish but not json"))
	if err == nil || !strings.Contains(err.Error(), "invalid json") {
		t.Fatalf("err=%v, want invalid json", err)
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

func TestValidateProviderFile_FinishReasonPathEventOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom_finish_event.conf")
	content := []byte(`
syntax "next-router/0.1";

provider "custom_finish_event" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_path "$.response.status" event="response.completed" event_optional=true;
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
	if got, want := len(rules), 1; got != want {
		t.Fatalf("path rules len=%d want=%d", got, want)
	}
	if rules[0].Event != "response.completed" || !rules[0].EventOptional {
		t.Fatalf("unexpected event options: %+v", rules[0])
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

func TestValidateProviderFile_FinishReasonPathEventOptionalRequiresEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid_finish_event.conf")
	content := []byte(`
syntax "next-router/0.1";

provider "invalid_finish_event" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_path "$.response.status" event_optional=true;
    }
  }
}
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "event_optional requires event") {
		t.Fatalf("ValidateProviderFile err=%v, want event_optional requires event", err)
	}
}

func TestValidateProviderFile_GlobalFinishReasonModePreset(t *testing.T) {
	rootDir := t.TempDir()
	providersDir := filepath.Join(rootDir, "providers")
	if err := os.MkdirAll(providersDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "onr.conf"), []byte(`
syntax "next-router/0.1";

include modes/*.conf;
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	modesDir := filepath.Join(rootDir, "modes")
	if err := os.MkdirAll(modesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll modes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modesDir, "finish_reason_modes.conf"), []byte(`
syntax "next-router/0.1";

finish_reason_mode "shared_finish" {
  finish_reason_path "$.meta.finish";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile finish_reason_modes.conf: %v", err)
	}
	providerPath := filepath.Join(providersDir, "demo.conf")
	if err := os.WriteFile(providerPath, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_extract shared_finish;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}

	pf, err := ValidateProviderFile(providerPath)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", providerPath, err)
	}

	cfg, ok := pf.Finish.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected finish_reason config selected")
	}
	if cfg.Mode != usageModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, usageModeCustom)
	}
	rules := cfg.finishReasonPathConfigs()
	if got, want := len(rules), 1; got != want {
		t.Fatalf("path rules len=%d want=%d", got, want)
	}
	if got := rules[0].Path; got != "$.meta.finish" {
		t.Fatalf("path=%q want=%q", got, "$.meta.finish")
	}
}

func TestValidateProviderFile_FinishReasonModeNamedOpenAIWorksAsNormalPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	content := []byte(`
syntax "next-router/0.1";

finish_reason_mode "openai" {
  finish_reason_path "$.meta.finish";
}

provider "demo" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_extract openai;
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
	if cfg.Mode != usageModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, usageModeCustom)
	}
	got, err := ExtractFinishReason(&dslmeta.Meta{API: "chat.completions"}, cfg, []byte(`{"meta":{"finish":"tool_calls"},"choices":[{"finish_reason":"stop"}]}`))
	if err != nil {
		t.Fatalf("ExtractFinishReason: %v", err)
	}
	if got != "tool_calls" {
		t.Fatalf("finish_reason=%q want=tool_calls", got)
	}
}

func TestValidateProviderFile_FinishReasonModeRequiresGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	content := []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_extract openai;
    }
  }
}
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateProviderFile_ProviderMetricsFinishReasonPathImplicitCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	content := []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics {
      finish_reason_path "$.meta.finish";
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
	if got := normalizeFinishReasonMode(cfg.Mode); got != usageModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, usageModeCustom)
	}
	got, err := ExtractFinishReason(&dslmeta.Meta{API: "chat.completions"}, cfg, []byte(`{"meta":{"finish":"tool_calls"},"choices":[{"finish_reason":"stop"}]}`))
	if err != nil {
		t.Fatalf("ExtractFinishReason: %v", err)
	}
	if got != "tool_calls" {
		t.Fatalf("finish_reason=%q want=tool_calls", got)
	}
}
