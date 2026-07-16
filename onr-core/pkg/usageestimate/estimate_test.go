package usageestimate

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestEstimateUsesStreamTailForOutputFallback(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:        apiMessages,
		Model:      "claude-opus-4-8",
		StreamTail: []byte("hello world"),
	})

	if out.Usage == nil {
		t.Fatalf("Usage=nil")
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatalf("OutputTokens=%d want > 0", out.Usage.OutputTokens)
	}
	if out.Stage != StageEstimateBoth {
		t.Fatalf("Stage=%q want=%q", out.Stage, StageEstimateBoth)
	}
}

func TestEstimateUsesResponseBodyWhenStreamTailEmpty(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	responseBody := []byte(`{"content":[{"type":"text","text":"structured response"}]}`)
	want, _ := EstimateToken("claude-opus-4-8", apiMessages, responseBody, EstimateOutput)

	out := Estimate(cfg, Input{
		API:           apiMessages,
		Model:         "claude-opus-4-8",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 1},
		ResponseBody:  responseBody,
	})

	if out.Usage == nil {
		t.Fatalf("Usage=nil")
	}
	if out.Usage.OutputTokens != want {
		t.Fatalf("OutputTokens=%d want=%d", out.Usage.OutputTokens, want)
	}
	if out.Stage != StageEstimateCompletion {
		t.Fatalf("Stage=%q want=%q", out.Stage, StageEstimateCompletion)
	}
}

func TestEstimatePrefersStreamTailOverResponseBodyForOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	streamTail := []byte("fallback stream text")
	want, _ := EstimateToken("claude-opus-4-8", apiMessages, streamTail, EstimateOutput)

	out := Estimate(cfg, Input{
		API:           apiMessages,
		Model:         "claude-opus-4-8",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 1},
		ResponseBody:  []byte(`{"content":[{"type":"text","text":"structured response"}]}`),
		StreamTail:    streamTail,
	})

	if out.Usage == nil {
		t.Fatalf("Usage=nil")
	}
	if out.Usage.OutputTokens != want {
		t.Fatalf("OutputTokens=%d want=%d", out.Usage.OutputTokens, want)
	}
}
