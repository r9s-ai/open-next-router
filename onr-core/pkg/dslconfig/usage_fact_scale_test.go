package dslconfig

import (
	"testing"
)

func TestParseUsageFactScale(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    metrics {
      usage_extract custom;
      usage_fact audio.tts second path="$.extra_info.audio_length" scale=0.001;
    }
  }
}`
	_, _, _, _, _, usage, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	facts := usage.Matches[0].Extract.facts
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Scale != 0.001 {
		t.Fatalf("expected scale 0.001, got %v", facts[0].Scale)
	}
}

func TestParseUsageFactScaleRejectsNonPositive(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    metrics {
      usage_extract custom;
      usage_fact audio.tts second path="$.extra_info.audio_length" scale=0;
    }
  }
}`
	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err == nil {
		t.Fatalf("expected error for scale=0")
	}
}

func TestEvaluateUsageFactWithScale(t *testing.T) {
	fact := usageFactConfig{
		Dimension: "audio.tts",
		Unit:      "second",
		Path:      "$.extra_info.audio_length",
		Scale:     0.001,
	}
	respRoot := map[string]any{
		"extra_info": map[string]any{"audio_length": 3500},
	}
	q, matched := evaluateUsageFactWithEvent("", nil, respRoot, nil, nil, false, fact)
	if !matched {
		t.Fatalf("expected fact to match")
	}
	if q != 3.5 {
		t.Fatalf("expected 3.5 seconds, got %v", q)
	}

	// path 缺失时不匹配、不缩放
	q, matched = evaluateUsageFactWithEvent("", nil, map[string]any{"other": 1}, nil, nil, false, fact)
	if matched || q != 0 {
		t.Fatalf("expected no match for missing path, got q=%v matched=%v", q, matched)
	}
}
