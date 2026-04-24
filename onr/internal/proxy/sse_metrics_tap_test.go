package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestSSEMetricsTap_LargeSingleEventKeepsUsage(t *testing.T) {
	t.Parallel()

	agg := dslconfig.NewStreamMetricsAggregator(
		&dslmeta.Meta{API: "images.generations", IsStream: true},
		&dslconfig.UsageExtractConfig{
			Mode:             "custom",
			InputTokensPath:  "$.usage.input_tokens",
			OutputTokensPath: "$.usage.output_tokens",
		},
		&dslconfig.FinishReasonExtractConfig{},
	)
	tap := newSSEMetricsTap(agg)

	largeEvent := "event: image_generation.completed\n" +
		`data: {"type":"image_generation.completed","b64_json":"` + strings.Repeat("A", 400000) + `","usage":{"input_tokens":12,"output_tokens":4508,"total_tokens":4520}}` + "\n\n"

	if _, err := tap.Write([]byte(largeEvent[:200000])); err != nil {
		t.Fatalf("tap.Write chunk1: %v", err)
	}
	if _, err := tap.Write([]byte(largeEvent[200000:])); err != nil {
		t.Fatalf("tap.Write chunk2: %v", err)
	}
	tap.Finish()

	u, _, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 12 || u.OutputTokens != 4508 || u.TotalTokens != 4520 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
}

func TestSSEMetricsTap_ForwardsSSEEventName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "event-tap.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "event-tap" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact input token path="$.message.usage.input_tokens" event="message_start";
      usage_fact output token path="$.usage.output_tokens" event="message_delta";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pf, err := dslconfig.ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	agg := dslconfig.NewStreamMetricsAggregator(
		&dslmeta.Meta{API: "claude.messages", IsStream: true},
		&pf.Usage.Defaults,
		&dslconfig.FinishReasonExtractConfig{},
	)
	tap := newSSEMetricsTap(agg)

	stream := "" +
		"event: message_start\n" +
		`data: {"message":{"usage":{"input_tokens":12}}}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"usage":{"output_tokens":5}}` + "\n\n"

	if _, err := tap.Write([]byte(stream)); err != nil {
		t.Fatalf("tap.Write: %v", err)
	}
	tap.Finish()

	u, _, _, ok := agg.Result()
	if !ok || u == nil {
		t.Fatalf("expected usage ok")
	}
	if u.InputTokens != 12 || u.OutputTokens != 5 || u.TotalTokens != 17 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
}
