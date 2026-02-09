package pricing

import (
	"testing"
	"time"
)

func TestBuildPriceFileStableOrder(t *testing.T) {
	fetch := &FetchResult{
		URL:       "https://models.dev/api.json",
		FetchedAt: time.Date(2026, 2, 9, 8, 0, 0, 0, time.UTC),
	}
	prices := []ModelPrice{
		{Provider: "openai", Model: "gpt-4o-mini", Cost: map[string]float64{"output": 0.6, "input": 0.15}},
		{Provider: "anthropic", Model: "claude-haiku-4-5", Cost: map[string]float64{"input": 1, "output": 5}},
	}
	out := BuildPriceFile(fetch, []string{"openai", "anthropic", "openai"}, prices)
	if out.Version != "v1" {
		t.Fatalf("version=%q want=v1", out.Version)
	}
	if out.Unit != "usd_per_1m_tokens" {
		t.Fatalf("unit=%q", out.Unit)
	}
	if len(out.Source.Providers) != 2 || out.Source.Providers[0] != "anthropic" || out.Source.Providers[1] != "openai" {
		t.Fatalf("providers=%v", out.Source.Providers)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("entries len=%d want=2", len(out.Entries))
	}
	if out.Entries[0].Provider != "anthropic" || out.Entries[0].Model != "claude-haiku-4-5" {
		t.Fatalf("entry[0]=%+v", out.Entries[0])
	}
	if out.Entries[1].Provider != "openai" || out.Entries[1].Model != "gpt-4o-mini" {
		t.Fatalf("entry[1]=%+v", out.Entries[1])
	}
}
