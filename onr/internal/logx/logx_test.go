package logx

import (
	"strings"
	"testing"
)

func TestFormatFieldsCostFloatNoScientificNotation(t *testing.T) {
	out := formatFields(map[string]any{
		"provider":    "openai",
		"cost_input":  1.2e-06,
		"cost_output": 5.399999999999999e-06,
		"cost_total":  6.5999999999999995e-06,
	})
	if strings.Contains(out, "e-") || strings.Contains(out, "E-") {
		t.Fatalf("unexpected scientific notation: %q", out)
	}
	if !strings.Contains(out, "cost_input=0.0000012") {
		t.Fatalf("unexpected cost_input: %q", out)
	}
	if !strings.Contains(out, "cost_output=0.0000054") {
		t.Fatalf("unexpected cost_output: %q", out)
	}
	if !strings.Contains(out, "cost_total=0.0000066") {
		t.Fatalf("unexpected cost_total: %q", out)
	}
}

func TestFormatFieldsUsageExtraAfterKnownFields(t *testing.T) {
	out := formatFields(map[string]any{
		"provider":          "openai",
		"api":               "audio.speech",
		"model":             "gpt-4o-mini-tts",
		"upstream_status":   200,
		"audio_tts_seconds": 1.608,
	})
	audioPos := strings.Index(out, "audio_tts_seconds=1.608")
	statusPos := strings.Index(out, "upstream_status=200")
	if audioPos < 0 || statusPos < 0 {
		t.Fatalf("unexpected output: %q", out)
	}
	if audioPos <= statusPos {
		t.Fatalf("usage extra should be placed after known fields, got: %q", out)
	}
}
