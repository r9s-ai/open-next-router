package usageestimate

import (
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestEstimate_WhenMissingUsage_EstimateBoth(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:   "chat.completions",
		Model: "gpt-4o-mini",
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"hello"}]
		}`),
		ResponseBody: []byte(`{
			"id":"x",
			"choices":[{"index":0,"message":{"role":"assistant","content":"world"}}]
		}`),
	})

	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage = %q, want %q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.TotalTokens <= 0 {
		t.Fatalf("total_tokens = %d, want > 0", out.Usage.TotalTokens)
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens = %d, want > 0", out.Usage.InputTokens)
	}
}

func TestEstimate_WhenUpstreamUsagePresent_Upstream(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 12 {
		t.Fatalf("usage total_tokens = %#v, want 12", out.Usage)
	}
}

func TestEstimate_NormalizeTotalTokens(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 11, OutputTokens: 9, TotalTokens: 0},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 20 {
		t.Fatalf("total_tokens = %v, want 20", out.Usage)
	}
}

func TestEstimate_WhenAllZeroUsage_Estimates(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 0, TotalTokens: 0},
		RequestBody:   []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody:  []byte(`{"choices":[{"message":{"role":"assistant","content":"world"}}]}`),
	})
	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil || out.Usage.TotalTokens <= 0 {
		t.Fatalf("expected estimated usage, got %#v", out.Usage)
	}
}

func TestEstimate_WhenUpstreamMissingOutputTokens_EstimateCompletion(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	sse := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		StreamTail:    []byte(sse),
	})
	if out.Stage != StageEstimateCompletion {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateCompletion)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.InputTokens != 6 {
		t.Fatalf("input_tokens=%d want=6", out.Usage.InputTokens)
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatalf("output_tokens=%d want > 0", out.Usage.OutputTokens)
	}
	if out.Usage.TotalTokens != out.Usage.InputTokens+out.Usage.OutputTokens {
		t.Fatalf("total_tokens=%d want=%d", out.Usage.TotalTokens, out.Usage.InputTokens+out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenUpstreamMissingInputTokens_EstimatePrompt(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 8, TotalTokens: 8},
		RequestBody:   []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
	})
	if out.Stage != StageEstimatePrompt {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimatePrompt)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens != 8 {
		t.Fatalf("output_tokens=%d want=8", out.Usage.OutputTokens)
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens=%d want > 0", out.Usage.InputTokens)
	}
	if out.Usage.TotalTokens != out.Usage.InputTokens+out.Usage.OutputTokens {
		t.Fatalf("total_tokens=%d want=%d", out.Usage.TotalTokens, out.Usage.InputTokens+out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenMissingOutputTokensButNoText_DontEstimateCompletion(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		StreamTail:    []byte("data: [DONE]\n\n"),
	})
	if out.Stage != StageUpstream {
		t.Fatalf("stage=%q want=%q", out.Stage, StageUpstream)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens != 0 {
		t.Fatalf("output_tokens=%d want=0", out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenEstimationDisabled_ReturnsNilOnMissing(t *testing.T) {
	cfg := &Config{
		Enabled:                   true,
		EstimateWhenMissingOrZero: false,
		Strategy:                  "heuristic",
		MaxRequestBytes:           1024,
		MaxResponseBytes:          1024,
		MaxStreamCollectBytes:     1024,
		APIs:                      []string{"chat.completions"},
	}

	out := Estimate(cfg, Input{
		API:   "chat.completions",
		Model: "gpt-4o-mini",
	})
	if out.Stage != "" || out.Usage != nil {
		t.Fatalf("expected empty output, got stage=%q usage=%#v", out.Stage, out.Usage)
	}
}

func TestExtractStreamText_ChatCompletionsDelta(t *testing.T) {
	t.Parallel()

	sse := strings.Join([]string{
		`data: {"id":"x","choices":[{"delta":{"content":"hel"}}]}`,
		"",
		`data: {"id":"x","choices":[{"delta":{"content":"lo"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	got := extractStreamText("chat.completions", []byte(sse), 1024)
	if strings.ReplaceAll(got, "\n", "") != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}
