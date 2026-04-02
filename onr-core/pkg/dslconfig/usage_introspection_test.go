package dslconfig

import "testing"

func TestUsageExtractConfigDeclaredFacts(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: "custom",
		facts: []usageFactConfig{
			{
				Dimension: "server_tool.web_search",
				Unit:      "call",
				CountPath: "$.output[*]",
				Type:      "web_search_call",
				Status:    "completed",
			},
			{
				Dimension: "audio.tts",
				Unit:      "second",
				Source:    "derived",
				Path:      "$.audio_duration_seconds",
			},
		},
	}

	facts := cfg.DeclaredFacts()
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
	if facts[0].Dimension != "server_tool.web_search" || facts[0].Unit != "call" || facts[0].CountPath != "$.output[*]" {
		t.Fatalf("unexpected fact[0]: %#v", facts[0])
	}
	if facts[1].Dimension != "audio.tts" || facts[1].Unit != "second" || facts[1].Source != "derived" {
		t.Fatalf("unexpected fact[1]: %#v", facts[1])
	}
}
