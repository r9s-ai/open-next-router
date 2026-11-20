package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestSelectedUsageReusesCompiledRules(t *testing.T) {
	path := filepath.Join("..", "..", "..", "config", "providers", "openai.conf")
	provider, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatal(err)
	}

	speech := &dslmeta.Meta{API: "audio.speech"}
	first, ok := provider.Usage.Select(speech)
	if !ok || first.compiled == nil {
		t.Fatal("expected precompiled audio speech usage")
	}
	second, ok := provider.Usage.Select(speech)
	if !ok || first.compiled != second.compiled {
		t.Fatal("selected rules were compiled again")
	}
	if !UsesDerivedUsagePath(speech, first, "$.audio_duration_seconds") ||
		!UsesDerivedUsagePath(speech, first, "$.audio_estimated_tokens") ||
		UsesDerivedUsagePath(speech, first, "$.output_text_tokens") {
		t.Fatal("incorrect derived paths for audio speech")
	}

	chat := &dslmeta.Meta{API: "chat.completions"}
	chatCfg, ok := provider.Usage.Select(chat)
	if !ok || chatCfg.compiled == nil || chatCfg.compiled == first.compiled {
		t.Fatal("expected a separate compiled chat config")
	}
	if UsesDerivedUsagePath(chat, chatCfg, "$.audio_duration_seconds") {
		t.Fatal("audio path leaked into chat config")
	}
}

func TestSelectedUsageReloadReplacesCompiledRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.conf")
	registry := NewRegistry()
	writeProvider := func(derivedPath string) {
		t.Helper()
		content := fmt.Sprintf(`syntax "next-router/0.1";
provider "usage-reload" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    metrics {
      usage_extract custom;
      usage_fact input token source=derived path=%q;
    }
  }
}`, derivedPath)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := registry.ReloadFromFile(path); err != nil {
			t.Fatal(err)
		}
	}

	meta := &dslmeta.Meta{API: "chat.completions"}
	writeProvider("$.first")
	firstProvider, _ := registry.GetProvider("usage-reload")
	first, ok := firstProvider.Usage.Select(meta)
	if !ok || !UsesDerivedUsagePath(meta, first, "$.first") {
		t.Fatal("first compiled provider is missing its rule")
	}
	writeProvider("$.second")
	secondProvider, _ := registry.GetProvider("usage-reload")
	second, ok := secondProvider.Usage.Select(meta)
	if !ok || !UsesDerivedUsagePath(meta, second, "$.second") || UsesDerivedUsagePath(meta, second, "$.first") {
		t.Fatal("reload kept a stale compiled rule")
	}
	if !UsesDerivedUsagePath(meta, first, "$.first") || UsesDerivedUsagePath(meta, first, "$.second") {
		t.Fatal("reload changed an in-flight provider snapshot")
	}
}

func TestSelectedUsageConcurrentReadsDuringReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.conf")
	registry := NewRegistry()
	writeProvider := func(derivedPath string) {
		t.Helper()
		content := fmt.Sprintf(`syntax "next-router/0.1";
provider "usage-concurrent" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    metrics {
      usage_extract custom;
      usage_fact input token source=derived path=%q;
    }
  }
}`, derivedPath)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := registry.ReloadFromFile(path); err != nil {
			t.Fatal(err)
		}
	}
	writeProvider("$.first")
	var wg sync.WaitGroup
	errors := make(chan string, 8)
	for range 8 {
		wg.Go(func() {
			meta := &dslmeta.Meta{API: "chat.completions"}
			for range 100 {
				provider, ok := registry.GetProvider("usage-concurrent")
				if !ok {
					errors <- "provider missing during reload"
					return
				}
				cfg, ok := provider.Usage.Select(meta)
				if !ok {
					errors <- "usage missing during reload"
					return
				}
				first := UsesDerivedUsagePath(meta, cfg, "$.first")
				second := UsesDerivedUsagePath(meta, cfg, "$.second")
				if first == second {
					errors <- "selected config mixed rules from different reloads"
					return
				}
			}
		})
	}
	for range 10 {
		writeProvider("$.second")
		writeProvider("$.first")
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestSelectedUsageMergesDefaultsBeforeCompiling(t *testing.T) {
	p := ProviderUsage{
		Defaults: UsageExtractConfig{Mode: "custom", facts: []usageFactConfig{{Dimension: "input", Unit: "token", Source: "derived", Path: "$.input_text_tokens"}}},
		Matches:  []MatchUsage{{API: "audio.speech", Extract: UsageExtractConfig{facts: []usageFactConfig{{Dimension: "output", Unit: "token", Source: "derived", Path: "$.audio_estimated_tokens"}}}}},
	}
	p.prepareSelectedUsage()
	cfg, ok := p.Select(&dslmeta.Meta{API: "audio.speech"})
	if !ok || !UsesDerivedUsagePath(nil, cfg, "$.input_text_tokens") || !UsesDerivedUsagePath(nil, cfg, "$.audio_estimated_tokens") {
		t.Fatal("merged rules omitted a derived path")
	}
	defaultCfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok || !UsesDerivedUsagePath(nil, defaultCfg, "$.input_text_tokens") || UsesDerivedUsagePath(nil, defaultCfg, "$.audio_estimated_tokens") {
		t.Fatal("match rule leaked into defaults")
	}
}

func TestSelectedUsageExtractionMatchesUncompiledConfig(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		api      string
		root     map[string]any
		derived  map[string]any
	}{
		{
			name:     "OpenAI chat with cached tokens",
			provider: "openai.conf",
			api:      "chat.completions",
			root:     map[string]any{"usage": map[string]any{"prompt_tokens": float64(11), "completion_tokens": float64(5), "total_tokens": float64(16), "prompt_tokens_details": map[string]any{"cached_tokens": float64(2)}}},
		},
		{
			name:     "Anthropic messages with cache usage",
			provider: "anthropic.conf",
			api:      "claude.messages",
			root:     map[string]any{"usage": map[string]any{"input_tokens": float64(11), "output_tokens": float64(5), "cache_read_input_tokens": float64(2)}},
		},
		{
			name:     "OpenAI audio derived usage",
			provider: "openai.conf",
			api:      "audio.speech",
			root:     map[string]any{},
			derived:  map[string]any{"audio_duration_seconds": float64(1), "audio_estimated_tokens": float64(21), "input_text_tokens": float64(4)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "config", "providers", tc.provider)
			provider, err := ValidateProviderFile(path)
			if err != nil {
				t.Fatal(err)
			}
			meta := &dslmeta.Meta{API: tc.api, DerivedUsage: tc.derived}
			selected, ok := provider.Usage.Select(meta)
			if !ok {
				t.Fatal("usage config not selected")
			}
			uncompiled := *selected
			uncompiled.compiled = nil
			got, gotCached, err := ExtractUsageObject(meta, selected, tc.root)
			if err != nil {
				t.Fatal(err)
			}
			want, wantCached, err := ExtractUsageObject(meta, &uncompiled, tc.root)
			if err != nil {
				t.Fatal(err)
			}
			sortFacts := func(facts []UsageFact) {
				sort.Slice(facts, func(i, j int) bool {
					left := facts[i].Dimension + "\x00" + facts[i].Unit + "\x00" + facts[i].Path
					right := facts[j].Dimension + "\x00" + facts[j].Unit + "\x00" + facts[j].Path
					return left < right
				})
			}
			sortFacts(got.DebugFacts)
			sortFacts(want.DebugFacts)
			if !reflect.DeepEqual(got, want) || gotCached != wantCached {
				t.Fatalf("compiled usage=%+v cached=%d; uncompiled usage=%+v cached=%d", got, gotCached, want, wantCached)
			}
		})
	}
}

func BenchmarkSelectedUsageDerivedPathQueries(b *testing.B) {
	meta := &dslmeta.Meta{API: "audio.speech"}
	cfg, _ := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	paths := [...]string{"$.audio_duration_seconds", "$.audio_estimated_tokens", "$.input_text_tokens", "$.output_text_tokens", "$.request_audio_duration_seconds"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			_ = UsesDerivedUsagePath(meta, cfg, path)
		}
	}
}

func BenchmarkUncompiledUsageDerivedPathQueries(b *testing.B) {
	meta := &dslmeta.Meta{API: "audio.speech"}
	selected, _ := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	cfg := *selected
	cfg.compiled = nil
	paths := [...]string{"$.audio_duration_seconds", "$.audio_estimated_tokens", "$.input_text_tokens", "$.output_text_tokens", "$.request_audio_duration_seconds"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			_ = UsesDerivedUsagePath(meta, &cfg, path)
		}
	}
}

func BenchmarkSelectedUsageEmptyDerivedPathQueries(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions"}
	cfg, _ := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = UsesDerivedUsagePath(meta, cfg, "$.input_text_tokens")
		_ = UsesDerivedUsagePath(meta, cfg, "$.output_text_tokens")
		_ = UsesDerivedUsagePath(meta, cfg, "$.request_audio_duration_seconds")
	}
}

func BenchmarkUncompiledUsageEmptyDerivedPathQueries(b *testing.B) {
	meta := &dslmeta.Meta{API: "chat.completions"}
	selected, _ := mustLoadProviderMatchConfigsTB(b, "openai.conf", meta.API, meta.IsStream)
	cfg := *selected
	cfg.compiled = nil
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = UsesDerivedUsagePath(meta, &cfg, "$.input_text_tokens")
		_ = UsesDerivedUsagePath(meta, &cfg, "$.output_text_tokens")
		_ = UsesDerivedUsagePath(meta, &cfg, "$.request_audio_duration_seconds")
	}
}
