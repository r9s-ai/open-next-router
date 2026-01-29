package usageestimate

import (
	"testing"

	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
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
