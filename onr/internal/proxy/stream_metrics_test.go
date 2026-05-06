package proxy

import (
	"testing"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestStreamPerfMetrics(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	first := start.Add(500 * time.Millisecond)

	ttft, tps := streamPerfMetrics(start, first, map[string]any{
		"output_tokens": 30,
	})
	if ttft != 500 {
		t.Fatalf("expected ttft=500, got=%d", ttft)
	}
	if tps <= 0 {
		t.Fatalf("expected tps>0, got=%f", tps)
	}
}

func TestStreamPerfMetrics_MissingOutputTokens(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	first := start.Add(300 * time.Millisecond)

	ttft, tps := streamPerfMetrics(start, first, map[string]any{})
	if ttft != 300 {
		t.Fatalf("expected ttft=300, got=%d", ttft)
	}
	if tps != 0 {
		t.Fatalf("expected tps=0, got=%f", tps)
	}
}

func TestShouldTapRawSSEForMetrics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		usageCfg  *dslconfig.UsageExtractConfig
		finishCfg *dslconfig.FinishReasonExtractConfig
		want      bool
	}{
		{
			name:      "nil_configs",
			usageCfg:  nil,
			finishCfg: nil,
			want:      false,
		},
		{
			name:      "usage_config_non_anthropic_mode",
			usageCfg:  &dslconfig.UsageExtractConfig{Mode: "custom"},
			finishCfg: nil,
			want:      true,
		},
		{
			name:      "finish_config_non_anthropic_mode",
			usageCfg:  nil,
			finishCfg: &dslconfig.FinishReasonExtractConfig{Mode: "openai_chat_stream"},
			want:      true,
		},
		{
			name:      "config_present_even_without_mode",
			usageCfg:  &dslconfig.UsageExtractConfig{},
			finishCfg: nil,
			want:      true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := shouldTapRawSSEForMetrics(tc.usageCfg, tc.finishCfg)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
