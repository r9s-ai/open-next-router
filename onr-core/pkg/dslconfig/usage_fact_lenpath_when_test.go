package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func writeUsageFactTestProvider(t *testing.T, metrics string) ProviderFile {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	content := `
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
` + metrics + `
    }
  }
}
`
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	return pf
}

func findDebugFact(facts []UsageFact, dimension, unit string) (UsageFact, bool) {
	for _, fact := range facts {
		if fact.Dimension == dimension && fact.Unit == unit {
			return fact, true
		}
	}
	return UsageFact{}, false
}

func TestExtractUsage_UsageFactLenPathCountsRunes(t *testing.T) {
	pf := writeUsageFactTestProvider(t, `
      usage_fact output character len_path="$.text";
      usage_fact input character source=request len_path="$.input";
`)

	meta := &dslmeta.Meta{}
	meta.SetRequestRoot(map[string]any{"input": "héllo 世界"})
	// "héllo 世界" = 8 runes; response text = 12 runes.
	usage, _, err := ExtractUsage(meta, &pf.Usage.Defaults, []byte(`{"text":"Hello, world"}`))
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	outFact, ok := findDebugFact(usage.DebugFacts, "output", "character")
	if !ok || outFact.Quantity != 12 {
		t.Fatalf("output character fact: ok=%v got=%v want=12 (facts=%#v)", ok, outFact.Quantity, usage.DebugFacts)
	}
	inFact, ok := findDebugFact(usage.DebugFacts, "input", "character")
	if !ok || inFact.Quantity != 8 {
		t.Fatalf("input character fact: ok=%v got=%v want=8 (facts=%#v)", ok, inFact.Quantity, usage.DebugFacts)
	}
}

func TestExtractUsage_UsageFactWhenBranchesOnUsageType(t *testing.T) {
	pf := writeUsageFactTestProvider(t, `
      usage_root path="$.usage";
      usage_fact audio.stt second path="$.seconds" when_path="$.type" when_eq="duration";
      usage_fact audio.stt second path="$.input_token_details.audio_tokens" scale=0.048 when_path="$.type" when_eq="tokens" fallback=true;
`)

	// Branch 1: whisper-style duration usage.
	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, []byte(`{"usage":{"type":"duration","seconds":42}}`))
	if err != nil {
		t.Fatalf("ExtractUsage(duration): %v", err)
	}
	fact, ok := findDebugFact(usage.DebugFacts, "audio.stt", "second")
	if !ok || fact.Quantity != 42 {
		t.Fatalf("duration branch: ok=%v got=%v want=42", ok, fact.Quantity)
	}

	// Branch 2: gpt-4o-transcribe token usage, converted at 60/1250 s per token.
	usage, _, err = ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, []byte(`{"usage":{"type":"tokens","input_token_details":{"audio_tokens":1250}}}`))
	if err != nil {
		t.Fatalf("ExtractUsage(tokens): %v", err)
	}
	fact, ok = findDebugFact(usage.DebugFacts, "audio.stt", "second")
	if !ok || fact.Quantity != 60 {
		t.Fatalf("tokens branch: ok=%v got=%v want=60", ok, fact.Quantity)
	}

	// No usage.type at all: neither branch matches, no fact emitted.
	usage, _, err = ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, []byte(`{"usage":{"seconds":42}}`))
	if err != nil {
		t.Fatalf("ExtractUsage(no type): %v", err)
	}
	if _, ok := findDebugFact(usage.DebugFacts, "audio.stt", "second"); ok {
		t.Fatalf("expected no audio.stt fact when when_path is missing, got %#v", usage.DebugFacts)
	}
}

func TestValidateProviderFile_UsageFactLenPathMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    metrics {
      usage_extract custom;
      usage_fact output character len_path="$.text" path="$.n";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected mutual-exclusion parse error, got %v", err)
	}
}

func TestValidateProviderFile_UsageFactWhenRequiresPair(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    metrics {
      usage_extract custom;
      usage_fact audio.stt second path="$.seconds" when_path="$.type";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "when_path and when_eq") {
		t.Fatalf("expected when pair validation error, got %v", err)
	}
}
