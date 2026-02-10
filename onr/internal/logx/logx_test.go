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
