package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestValidateProviderFile_UsageFactCustomFirstExtract(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "anthropic" {}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token count_path="$.usage.input_events[*]" type="prompt" status="accepted";
      usage_fact output token sum_path="$.usage.output_chunks[*].tokens";
      usage_fact cache_read token path="$.usage.cache_read_input_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_events": [
	      {"type": "prompt", "status": "accepted"},
	      {"type": "prompt", "status": "rejected"}
	    ],
	    "output_chunks": [
	      {"tokens": 100},
	      {"tokens": 70}
	    ],
	    "cache_read_input_tokens": 0,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 6802,
	      "ephemeral_1h_input_tokens": 0
	    },
	    "cache_creation_input_tokens": 6802
	  }
	}`)

	usage, cachedTokens, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 1; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 170; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := cachedTokens, 0; got != want {
		t.Fatalf("cachedTokens got %d, want %d", got, want)
	}
	if usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 6802; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
	if usage.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := usage.FlatFields["cache_write_ttl_5m_tokens"], 6802; got != want {
		t.Fatalf("cache_write_ttl_5m_tokens got %v, want %v", got, want)
	}
	if got, want := usage.FlatFields["cache_write_ttl_1h_tokens"], 0; got != want {
		t.Fatalf("cache_write_ttl_1h_tokens got %v, want %v", got, want)
	}
	if got, want := usage.TotalTokens, 171; got != want {
		t.Fatalf("TotalTokens got %d, want %d", got, want)
	}
}

func TestValidateProviderFile_UsageFactExpr(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "anthropic" {}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token expr="$.usage.input_tokens + $.usage.extra_input";
      usage_fact output token path="$.usage.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "extra_input": 2,
	    "output_tokens": 7
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 12; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := usage.TotalTokens, 19; got != want {
		t.Fatalf("TotalTokens got %d, want %d", got, want)
	}
}

func TestValidateProviderFile_UsageModeImplicitCustomWhenFactsPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract shared_tokens;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got, want := normalizeUsageMode(pf.Usage.Defaults.Mode), usageModeCustom; got != want {
		t.Fatalf("usage mode got %q, want %q", got, want)
	}
	facts := pf.Usage.Defaults.CompiledFacts(nil)
	if len(facts) != 2 {
		t.Fatalf("compiled facts got %d, want 2: %#v", len(facts), facts)
	}
}

func TestExtractUsage_UsageFactFilterPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "anthropic" {}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usageMetadata.promptTokensDetails[?(@.modality==\"TEXT\")].tokenCount";
      usage_fact output token path="$.usageMetadata.candidatesTokenCount";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usageMetadata": {
	    "promptTokenCount": 81,
	    "promptTokensDetails": [
	      {"modality": "TEXT", "tokenCount": 5},
	      {"modality": "AUDIO", "tokenCount": 76}
	    ],
	    "candidatesTokenCount": 7
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 5; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageFactGeminiMultimodalInputTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "anthropic" {}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usageMetadata.promptTokensDetails[?(@.modality==\"TEXT\")].tokenCount";
      usage_fact input.image token path="$.usageMetadata.promptTokensDetails[?(@.modality==\"IMAGE\")].tokenCount";
      usage_fact input.video token path="$.usageMetadata.promptTokensDetails[?(@.modality==\"VIDEO\")].tokenCount";
      usage_fact input.audio token path="$.usageMetadata.promptTokensDetails[?(@.modality==\"AUDIO\")].tokenCount";
      usage_fact output token path="$.usageMetadata.candidatesTokenCount";
      usage_fact output token path="$.usageMetadata.thoughtsTokenCount";
      usage_fact output.image token path="$.usageMetadata.candidatesTokensDetails[?(@.modality==\"IMAGE\")].tokenCount";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usageMetadata": {
	    "promptTokensDetails": [
	      {"modality": "TEXT", "tokenCount": 5},
	      {"modality": "IMAGE", "tokenCount": 12},
	      {"modality": "VIDEO", "tokenCount": 34},
	      {"modality": "AUDIO", "tokenCount": 76}
	    ],
	    "candidatesTokensDetails": [
	      {"modality": "IMAGE", "tokenCount": 1120}
	    ],
	    "candidatesTokenCount": 40,
	    "thoughtsTokenCount": 553
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 5; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 593; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if usage.FlatFields == nil {
		t.Fatalf("expected flat fields")
	}
	if got, want := usage.FlatFields["input_image_tokens"], 12; got != want {
		t.Fatalf("input_image_tokens got %v, want %v", got, want)
	}
	if got, want := usage.FlatFields["input_video_tokens"], 34; got != want {
		t.Fatalf("input_video_tokens got %v, want %v", got, want)
	}
	if got, want := usage.FlatFields["input_audio_tokens"], 76; got != want {
		t.Fatalf("input_audio_tokens got %v, want %v", got, want)
	}
	if got, want := usage.FlatFields["output_image_tokens"], 1120; got != want {
		t.Fatalf("output_image_tokens got %v, want %v", got, want)
	}
}

func TestExtractUsage_UsageFactFilterPathSingleQuotedString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "anthropic" {}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path='$.usageMetadata.promptTokenCount';
      usage_fact input.audio token path='$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount';
      usage_fact output token path='$.usageMetadata.candidatesTokenCount';
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usageMetadata": {
	    "promptTokenCount": 81,
	    "promptTokensDetails": [
	      {"modality": "TEXT", "tokenCount": 5},
	      {"modality": "AUDIO", "tokenCount": 76}
	    ],
	    "candidatesTokenCount": 7
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 81; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.FlatFields["input_audio_tokens"], 76; got != want {
		t.Fatalf("input_audio_tokens got %v, want %v", got, want)
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
}

func TestValidateProviderFile_UsageFactRejectsReverseInputModality(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact image.input token path="$.usage.image_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected image.input token to be rejected")
	}
}

func TestValidateProviderFile_UsageFactAllowedWithUserPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "anthropic" {}
usage_mode "shared_usage" {
  usage_fact output token path="$.usage.output_tokens";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract shared_usage;
      usage_fact input token path="$.usage.input_tokens";
    }
	}
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
}

func TestValidateProviderFile_UsageFactAllowedWithNamedPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "tool_usage" {
  usage_fact input token path="$.usage.input_tokens";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract tool_usage;
      usage_fact server_tool.web_search call count_path="$.tool_results[*]" type="web_search_call" status="completed";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
}

func TestValidateProviderFile_UsageFactImplicitCustomMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Usage.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("Usage.Select: no config selected")
	}
	if cfg.Mode != usageModeCustom {
		t.Fatalf("expected implicit custom mode, got %q", cfg.Mode)
	}
	facts := cfg.CompiledFacts(&dslmeta.Meta{API: "chat.completions"})
	if len(facts) != 2 {
		t.Fatalf("expected 2 compiled facts, got %d", len(facts))
	}
}

func TestValidateProviderFile_UsageFactRejectsMultiplePrimitives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens" expr="$.usage.extra_input";
      usage_fact output token path="$.usage.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateProviderFile_UsageFactTypeRequiresCountPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens" type="prompt";
      usage_fact output token path="$.usage.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateProviderFile_UsageFactRejectsUnsupportedDimension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
	    metrics {
	      usage_extract custom;
	      usage_fact unsupported.metric unit path="$.usage.image_count";
	      usage_fact output token path="$.usage.output_tokens";
	    }
	  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateProviderFile_UsageFactAllowsOutputImageToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact output token path="$.usage.output_tokens";
      usage_fact output.image token path="$.usage.output_image_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "output_tokens": 7,
	    "output_image_tokens": 1120
	  }
	}`)
	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := usage.TotalTokens, 7; got != want {
		t.Fatalf("TotalTokens got %d, want %d", got, want)
	}
	if usage.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := usage.FlatFields["output_image_tokens"], 1120; got != want {
		t.Fatalf("output_image_tokens got %v, want %v", got, want)
	}
	var found bool
	for _, fact := range usage.DebugFacts {
		if fact.Dimension == "output.image" && fact.Unit == "token" && fact.Quantity == 1120 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected output.image token debug fact, got %#v", usage.DebugFacts)
	}
}

func TestValidateProviderFile_UsageFactRejectsImageOutputToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact image.output token path="$.usage.output_image_tokens";
      usage_fact output token path="$.usage.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestExtractUsage_UsageFactFallbackOnlyUsesTotalWhenSpecificFactsMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "output_tokens": 20,
	    "cache_creation_input_tokens": 30
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 30; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageFactCacheWriteTotalUsesAggregateBeforeTTLDetailFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m" fallback=true;
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "output_tokens": 20,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 7,
	      "ephemeral_1h_input_tokens": 9
	    },
	    "cache_creation_input_tokens": 30
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 30; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
	if usage.FlatFields != nil {
		t.Fatalf("FlatFields got %v, want nil when aggregate cache_write matched", usage.FlatFields)
	}
}

func TestExtractUsage_UsageFactCacheWriteTotalFallsBackToTTLDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m" fallback=true;
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "output_tokens": 20,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 7,
	      "ephemeral_1h_input_tokens": 9
	    }
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 16; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
	if got, want := usage.FlatFields["cache_write_ttl_5m_tokens"], 7; got != want {
		t.Fatalf("cache_write_ttl_5m_tokens got %v, want %v", got, want)
	}
	if got, want := usage.FlatFields["cache_write_ttl_1h_tokens"], 9; got != want {
		t.Fatalf("cache_write_ttl_1h_tokens got %v, want %v", got, want)
	}
}

func TestValidateProviderFile_UsageFactAudioSecondAllowed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact audio.stt second path="$.usage.audio_seconds";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got := len(pf.Usage.Defaults.facts); got != 3 {
		t.Fatalf("facts len=%d want=3", got)
	}
}

func TestExtractUsage_UsageFactFallbackDoesNotDoubleCountSpecificCacheWriteFacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "output_tokens": 20,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 7,
	      "ephemeral_1h_input_tokens": 9
	    },
	    "cache_creation_input_tokens": 16
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 16; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageFactAnthropicOneHourTTLOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 2,
	    "output_tokens": 3,
	    "cache_creation": {
	      "ephemeral_1h_input_tokens": 24
	    },
	    "cache_creation_input_tokens": 24
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 24; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
	if usage.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := usage.FlatFields["cache_write_ttl_1h_tokens"], 24; got != want {
		t.Fatalf("cache_write_ttl_1h_tokens got %v, want %v", got, want)
	}
}

func TestExtractUsage_UsageFactAllCoreFieldsPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact cache_read token path="$.usage.cache_read_input_tokens";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
      usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
      usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 12,
	    "output_tokens": 7,
	    "cache_read_input_tokens": 5,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 8,
	      "ephemeral_1h_input_tokens": 2
	    },
	    "cache_creation_input_tokens": 10
	  }
	}`)

	usage, cached, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 12; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := cached, 5; got != want {
		t.Fatalf("cached got %d, want %d", got, want)
	}
	if usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CachedTokens, 5; got != want {
		t.Fatalf("CachedTokens got %d, want %d", got, want)
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 10; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageFactServerToolWebSearchCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
      usage_fact server_tool.web_search call count_path="$.tool_results[*]" type="web_search_call" status="completed";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 4,
	    "output_tokens": 6
	  },
	  "tool_results": [
	    {"type":"web_search_call","status":"completed"},
	    {"type":"web_search_call","status":"failed"},
	    {"type":"web_search_call","status":"completed"}
	  ]
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &pf.Usage.Defaults, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if usage.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := usage.FlatFields["server_tool_web_search_calls"], 2; got != want {
		t.Fatalf("server_tool_web_search_calls got %v, want %v", got, want)
	}
}

func TestExtractUsage_CustomUsageFactOverrideCacheRead(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.usage.prompt_tokens"},
			{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens", Fallback: true},
			{Dimension: "output", Unit: "token", Path: "$.usage.completion_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Fallback: true},
			{Dimension: "cache_read", Unit: "token", Path: "$.usage.cached_tokens"},
		},
	}

	body := []byte(`{
	  "usage": {
	    "prompt_tokens": 8,
	    "completion_tokens": 9,
	    "prompt_tokens_details": {
	      "cached_tokens": 5
	    },
	    "cached_tokens": 11
	  }
	}`)

	usage, cached, err := ExtractUsage(meta, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := cached, 11; got != want {
		t.Fatalf("cached got %d, want %d", got, want)
	}
	if got, want := usage.InputTokenDetails.CachedTokens, 11; got != want {
		t.Fatalf("CachedTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_CustomUsageFactOverrideCacheWrite(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: false}
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.usage.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"},
			{Dimension: "cache_write", Unit: "token", Path: "$.override.cache_write_tokens"},
		},
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "output_tokens": 7,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 8,
	      "ephemeral_1h_input_tokens": 2
	    },
	    "cache_creation_input_tokens": 10
	  },
	  "override": {
	    "cache_write_tokens": 33
	  }
	}`)

	usage, _, err := ExtractUsage(meta, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil || usage.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if got, want := usage.InputTokenDetails.CacheWriteTokens, 33; got != want {
		t.Fatalf("CacheWriteTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_CustomLegacyFieldsCompiledIntoFacts(t *testing.T) {
	inExpr := mustParseUsageExpr(t, "$.usage.prompt_tokens + $.usage.extra_input")
	cfg := UsageExtractConfig{
		Mode:                usageModeCustom,
		InputTokensExpr:     inExpr,
		OutputTokensPath:    "$.usage.output_tokens",
		CacheReadTokensPath: "$.usage.cached_tokens",
	}

	body := []byte(`{
	  "usage": {
	    "prompt_tokens": 7,
	    "extra_input": 2,
	    "output_tokens": 5,
	    "cached_tokens": 4
	  }
	}`)

	usage, cached, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 9; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 5; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := cached, 4; got != want {
		t.Fatalf("cached got %d, want %d", got, want)
	}
	if usage.InputTokenDetails == nil || usage.InputTokenDetails.CachedTokens != 4 {
		t.Fatalf("expected cached token details, got=%+v", usage.InputTokenDetails)
	}
}

func TestExtractUsage_CustomLegacyOverrideDoesNotDoubleCount(t *testing.T) {
	outExpr := mustParseUsageExpr(t, "$.usage.alt_output_tokens")
	cfg := UsageExtractConfig{
		Mode:             usageModeCustom,
		InputTokensPath:  "$.usage.prompt_tokens",
		OutputTokensExpr: outExpr,
	}

	body := []byte(`{
	  "usage": {
	    "prompt_tokens": 8,
	    "output_tokens": 9,
	    "alt_output_tokens": 13
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{API: "chat.completions", IsStream: false}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 8; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 13; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := usage.TotalTokens, 21; got != want {
		t.Fatalf("TotalTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageFactSameDimensionDeclarationOrder(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.usage.first_input_tokens"},
			{Dimension: "input", Unit: "token", Path: "$.usage.second_input_tokens"},
			{Dimension: "input", Unit: "token", Path: "$.usage.total_input_tokens", Fallback: true},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"},
		},
	}

	body := []byte(`{
	  "usage": {
	    "first_input_tokens": 3,
	    "second_input_tokens": 4,
	    "total_input_tokens": 99,
	    "output_tokens": 5
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 7; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	inputFacts := make([]UsageFact, 0, len(usage.DebugFacts))
	for _, fact := range usage.DebugFacts {
		if fact.Dimension == "input" && fact.Unit == "token" {
			inputFacts = append(inputFacts, fact)
		}
	}
	if got, want := len(inputFacts), 2; got != want {
		t.Fatalf("input debug facts len got %d, want %d", got, want)
	}
	if got, want := inputFacts[0].Path, "$.usage.first_input_tokens"; got != want {
		t.Fatalf("first input fact path got %q, want %q", got, want)
	}
	if got, want := inputFacts[0].Quantity, 3.0; got != want {
		t.Fatalf("first input fact quantity got %v, want %v", got, want)
	}
	if got, want := inputFacts[1].Path, "$.usage.second_input_tokens"; got != want {
		t.Fatalf("second input fact path got %q, want %q", got, want)
	}
	if got, want := inputFacts[1].Quantity, 4.0; got != want {
		t.Fatalf("second input fact quantity got %v, want %v", got, want)
	}
	for _, fact := range inputFacts {
		if fact.Fallback {
			t.Fatalf("unexpected fallback fact in matched input facts: %+v", fact)
		}
	}
}

func TestExtractUsage_UsageFactRequestSource(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Source: "request", Expr: mustParseUsageExpr(t, "$.n")},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens"},
			{Dimension: "image.generate", Unit: "image", Source: "request", Expr: mustParseUsageExpr(t, "$.n")},
		},
	}

	meta := &dslmeta.Meta{
		RequestBody: []byte(`{"model":"gpt-image-1","prompt":"draw a cat","n":3}`),
	}
	body := []byte(`{
	  "usage": {
	    "output_tokens": 7
	  }
	}`)

	usage, _, err := ExtractUsage(meta, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 3; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := usage.FlatFields["image_generate_images"], 3; got != want {
		t.Fatalf("image_generate_images got %v, want %v", got, want)
	}
	var requestFactFound bool
	for _, fact := range usage.DebugFacts {
		if fact.Dimension == "image.generate" && fact.Unit == "image" {
			requestFactFound = true
			if got, want := fact.Source, "request"; got != want {
				t.Fatalf("debug fact source got %q, want %q", got, want)
			}
		}
	}
	if !requestFactFound {
		t.Fatalf("expected image.generate debug fact")
	}
}

func TestExtractUsage_UsageRootDefaultSource(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.usage"},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.output_tokens"},
		},
	}

	body := []byte(`{
	  "usage": {
	    "input_tokens": 11,
	    "output_tokens": 13
	  },
	  "input_tokens": 99,
	  "output_tokens": 99
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 11; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 13; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := len(usage.DebugFacts), 2; got != want {
		t.Fatalf("DebugFacts len got %d, want %d", got, want)
	}
	for _, fact := range usage.DebugFacts {
		if got, want := fact.Source, "usage"; got != want {
			t.Fatalf("debug fact source got %q, want %q", got, want)
		}
	}
}

func TestExtractUsage_UsageRootResponseSourceBypass(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.response.usage"},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.output_tokens"},
			{Dimension: "server_tool.web_search", Unit: "call", Source: "response", Path: "$.response.tool_usage.web_search.num_requests"},
		},
	}

	body := []byte(`{
	  "response": {
	    "usage": {
	      "input_tokens": 5,
	      "output_tokens": 7
	    },
	    "tool_usage": {
	      "web_search": {
	        "num_requests": 2
	      }
	    }
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 5; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.FlatFields["server_tool_web_search_calls"], 2; got != want {
		t.Fatalf("server_tool_web_search_calls got %v, want %v", got, want)
	}
}

func TestExtractUsage_UsageRootMissingSkipsDefaultFactsButRunsResponseFacts(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.missing.usage"},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "server_tool.web_search", Unit: "call", Source: "response", Path: "$.tool_usage.web_search.num_requests"},
		},
	}

	body := []byte(`{
	  "input_tokens": 99,
	  "tool_usage": {
	    "web_search": {
	      "num_requests": 2
	    }
	  }
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got := usage.InputTokens; got != 0 {
		t.Fatalf("InputTokens got %d, want 0 because default facts require usage_root", got)
	}
	if got, want := usage.FlatFields["server_tool_web_search_calls"], 2; got != want {
		t.Fatalf("server_tool_web_search_calls got %v, want %v", got, want)
	}
}

func TestExtractUsage_UsageRootMergesMultipleRoots(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.message.usage"},
			{Path: "$.delta.usage"},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.output_tokens"},
			{Dimension: "cache_read", Unit: "token", Path: "$.cache_read_input_tokens"},
		},
	}
	root := map[string]any{
		"message": map[string]any{
			"usage": map[string]any{
				"input_tokens":            3,
				"cache_read_input_tokens": 0,
			},
		},
		"delta": map[string]any{
			"usage": map[string]any{
				"input_tokens":            99,
				"output_tokens":           9,
				"cache_read_input_tokens": 4,
			},
		},
	}

	usage, cached, err := extractUsageFromRootsWithEvent(nil, "", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 3; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 9; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
	if got, want := cached, 4; got != want {
		t.Fatalf("cached got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageRootNestedMergeDoesNotOverwriteNonZero(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.first"},
			{Path: "$.second"},
		},
		facts: []usageFactConfig{
			{Dimension: "cache_read", Unit: "token", Path: "$.input_tokens_details.cached_tokens"},
		},
	}
	root := map[string]any{
		"first": map[string]any{
			"input_tokens_details": map[string]any{
				"cached_tokens": 3,
			},
		},
		"second": map[string]any{
			"input_tokens_details": map[string]any{
				"cached_tokens": 99,
			},
		},
	}

	_, cached, err := extractUsageFromRootsWithEvent(nil, "", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent: %v", err)
	}
	if got, want := cached, 3; got != want {
		t.Fatalf("cached got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageFactDerivedSourceWithNonJSONResponse(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "audio.tts", Unit: "second", Source: "derived", Path: "$.audio_duration_seconds"},
		},
	}

	meta := &dslmeta.Meta{
		DerivedUsage: map[string]any{
			"audio_duration_seconds": 1.608,
		},
	}

	usage, _, err := ExtractUsage(meta, &cfg, []byte("fake-audio-binary-response"))
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.FlatFields["audio_tts_seconds"], 1.608; got != want {
		t.Fatalf("audio_tts_seconds got %v, want %v", got, want)
	}
	if len(usage.DebugFacts) != 1 {
		t.Fatalf("debug facts len=%d want=1", len(usage.DebugFacts))
	}
	if got, want := usage.DebugFacts[0].Source, "derived"; got != want {
		t.Fatalf("debug fact source got %q, want %q", got, want)
	}
}

func TestValidateProviderFile_UsageFactDerivedSourceAllowed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "derived-source.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "derived-source" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact audio.tts second source="derived" path="$.audio_duration_seconds";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
}

func TestValidateProviderFile_CustomDerivedOnlyUsageFactAllowed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-derived-only.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "custom-derived-only" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_fact audio.tts second source="derived" path="$.audio_duration_seconds";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}

	meta := &dslmeta.Meta{
		DerivedUsage: map[string]any{
			"audio_duration_seconds": 2.352,
		},
	}

	usage, _, err := ExtractUsage(meta, &pf.Usage.Defaults, []byte("fake-audio-binary-response"))
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.FlatFields["audio_tts_seconds"], 2.352; got != want {
		t.Fatalf("audio_tts_seconds got %v, want %v", got, want)
	}
}

func TestValidateProviderFile_UserUsageModeNamedOpenAIWorksAsNormalPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openai.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "openai" {
  usage_fact input token path="$.custom_usage.in";
  usage_fact output token path="$.custom_usage.out";
}

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got, want := normalizeUsageMode(pf.Usage.Defaults.Mode), usageModeCustom; got != want {
		t.Fatalf("usage mode got %q, want %q", got, want)
	}

	facts := pf.Usage.Defaults.CompiledFacts(nil)
	if len(facts) != 2 {
		t.Fatalf("compiled facts len=%d want=2", len(facts))
	}
	var foundInput bool
	var foundOutput bool
	for _, fact := range facts {
		if fact.Dimension == "input" && fact.Path == "$.custom_usage.in" {
			foundInput = true
		}
		if fact.Dimension == "output" && fact.Path == "$.custom_usage.out" {
			foundOutput = true
		}
	}
	if !foundInput || !foundOutput {
		t.Fatalf("unexpected compiled facts: %#v", facts)
	}
}

func TestValidateProviderFile_EmptyNamedUsageModeProducesNoSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openai.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "openai" {}

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if _, ok := pf.Usage.Select(&dslmeta.Meta{API: "chat.completions"}); ok {
		t.Fatalf("expected no usage config selected")
	}
}

func TestValidateProviderFile_UsageModeNamedOpenAIDoesNotInjectBuiltinFacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openai.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "openai" {
  usage_fact input token path="$.custom_usage.in";
  usage_fact output token path="$.custom_usage.out";
}

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got, want := normalizeUsageMode(pf.Usage.Defaults.Mode), usageModeCustom; got != want {
		t.Fatalf("usage mode got %q, want %q", got, want)
	}
	facts := pf.Usage.Defaults.CompiledFacts(nil)
	var foundInput bool
	var foundOutput bool
	for _, fact := range facts {
		if fact.Dimension == "input" && fact.Path == "$.custom_usage.in" {
			foundInput = true
		}
		if fact.Dimension == "output" && fact.Path == "$.custom_usage.out" {
			foundOutput = true
		}
	}
	if !foundInput || !foundOutput || len(facts) != 2 {
		t.Fatalf("unexpected compiled facts: %#v", facts)
	}
}

func TestValidateProviderFile_UsageModeRequiresGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openai.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateProviderFile_ProviderMetricsUsageRulesImplicitCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact input token path="$.usage.input_tokens";
      usage_fact output token path="$.usage.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}
	cfg, ok := pf.Usage.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected usage config selected")
	}
	if got := normalizeUsageMode(cfg.Mode); got != usageModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, usageModeCustom)
	}
	facts := cfg.CompiledFacts(&dslmeta.Meta{API: "chat.completions"})
	if len(facts) != 2 {
		t.Fatalf("compiled facts len=%d want=2", len(facts))
	}
}

func TestValidateProviderFile_LoadsSiblingOnrConfigUsageMode(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	path := filepath.Join(providersDir, "openai.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract shared_tokens;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	facts := pf.Usage.Defaults.CompiledFacts(nil)
	if len(facts) != 2 {
		t.Fatalf("compiled facts len=%d want=2", len(facts))
	}
	if facts[0].Path != "$.usage.input_tokens" || facts[1].Path != "$.usage.output_tokens" {
		t.Fatalf("unexpected compiled facts: %#v", facts)
	}
}

func TestUsesDerivedUsagePath_CustomDerivedAudioSpeech(t *testing.T) {
	meta := &dslmeta.Meta{API: "audio.speech", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	if !UsesDerivedUsagePath(meta, cfg, "$.audio_duration_seconds") {
		t.Fatalf("expected audio.speech preset to reference derived audio duration")
	}
	chatCfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", "chat.completions", false)
	if UsesDerivedUsagePath(&dslmeta.Meta{API: "chat.completions", IsStream: false}, chatCfg, "$.audio_duration_seconds") {
		t.Fatalf("did not expect chat.completions usage to reference derived audio duration")
	}
}

func TestValidateProviderFile_UsageFactRequestSourceInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid-request-source.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "invalid-request-source" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
	    metrics {
	      usage_extract custom;
	      usage_fact input token source="stream_event" path="$.n";
	      usage_fact output token path="$.usage.output_tokens";
	    }
	  }
	}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "unsupported source") {
		t.Fatalf("ValidateProviderFile err=%v, want unsupported source", err)
	}
}

func TestValidateProviderFile_UsageFactUsageSourceRequiresUsageRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-source-invalid.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "usage-source-invalid" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact input token source="usage" path="$.input_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "source=usage requires usage_root") {
		t.Fatalf("ValidateProviderFile err=%v, want source=usage requires usage_root", err)
	}
}

func TestValidateProviderFile_UsageFactEventOption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event-option.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "event-option" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact input token path="$.message.usage.input_tokens" event="message_start";
      usage_fact output token path="$.usage.output_tokens" event="message_delta" event_optional=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	facts := pf.Usage.Defaults.CompiledFacts(nil)
	if len(facts) != 2 {
		t.Fatalf("compiled facts len=%d want=2", len(facts))
	}
	if got, want := facts[0].Event, "message_start"; got != want {
		t.Fatalf("facts[0].Event=%q want=%q", got, want)
	}
	if got, want := facts[1].Event, "message_delta"; got != want {
		t.Fatalf("facts[1].Event=%q want=%q", got, want)
	}
	if !facts[1].EventOptional {
		t.Fatalf("facts[1].EventOptional got false want true")
	}
}

func TestValidateProviderFile_UsageRootOption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-root.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "usage-root" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_root path="$.message.usage" exclude="output_tokens|total_tokens";
      usage_fact input token path="$.input_tokens";
      usage_fact output token path="$.output_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	roots := pf.Usage.Defaults.CompiledPlan(nil).UsageRoots
	if len(roots) != 1 {
		t.Fatalf("usage roots len=%d want=1", len(roots))
	}
	if got, want := roots[0].Path, "$.message.usage"; got != want {
		t.Fatalf("usage root path=%q want=%q", got, want)
	}
	if got, want := strings.Join(roots[0].ExcludeFields, "|"), "output_tokens|total_tokens"; got != want {
		t.Fatalf("usage root exclude=%q want=%q", got, want)
	}
	facts := pf.Usage.Defaults.CompiledPlan(nil).Facts
	if len(facts) != 2 {
		t.Fatalf("facts len=%d want=2", len(facts))
	}
	for _, fact := range facts {
		if got, want := fact.Source, "usage"; got != want {
			t.Fatalf("compiled fact source got %q, want %q", got, want)
		}
	}
}

func TestValidateProviderFile_UsageRootExcludeValidation(t *testing.T) {
	tests := []struct {
		name    string
		exclude string
	}{
		{name: "empty", exclude: ""},
		{name: "jsonpath", exclude: "$.output_tokens"},
		{name: "nested", exclude: "nested.field"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "usage-root-exclude-invalid.conf")
			if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "usage-root-exclude-invalid" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_root path="$.usage" exclude="`+tc.exclude+`";
    }
  }
}
`), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "usage_root[0] exclude") {
				t.Fatalf("ValidateProviderFile err=%v, want usage_root exclude validation", err)
			}
		})
	}
}

func TestExtractUsage_UsageRootExcludeField(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.usage", ExcludeFields: []string{"output_tokens"}},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.output_tokens"},
		},
	}
	body := []byte(`{"usage":{"input_tokens":10,"output_tokens":9}}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 10; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 0; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageRootExcludeDoesNotAffectResponseSource(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.usage", ExcludeFields: []string{"output_tokens"}},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Source: "response", Path: "$.usage.output_tokens"},
		},
	}
	body := []byte(`{"usage":{"input_tokens":10,"output_tokens":9}}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 10; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 9; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
}

func TestExtractUsage_UsageRootMergeDoesNotMutateResponseSource(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.usage_a"},
			{Path: "$.usage_b"},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Source: "response", Path: "$.usage_a.nested.tokens"},
			{Dimension: "output", Unit: "token", Path: "$.nested.tokens"},
		},
	}
	body := []byte(`{
	  "usage_a":{"nested":{"tokens":0}},
	  "usage_b":{"nested":{"tokens":5}}
	}`)

	usage, _, err := ExtractUsage(&dslmeta.Meta{}, &cfg, body)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 0; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens, 5; got != want {
		t.Fatalf("OutputTokens got %d, want %d", got, want)
	}
}

func TestValidateProviderFile_UsageRootOnlyDoesNotSelectUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-root-only.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "usage-root-only" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_root path="$.usage";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if _, ok := pf.Usage.Select(&dslmeta.Meta{API: "chat.completions"}); ok {
		t.Fatalf("expected usage_root-only metrics to produce no usage selection")
	}
}

func TestValidateProviderFile_UsageRootOnlyStillValidatesPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-root-only-invalid.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "usage-root-only-invalid" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_root path="usage";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "usage_root[0] path must start with $.") {
		t.Fatalf("ValidateProviderFile err=%v, want usage_root path validation", err)
	}
}

func TestValidateProviderFile_CustomUsageRootOnlyInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-usage-root-only.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "custom-usage-root-only" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_extract custom;
      usage_root path="$.usage";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "custom usage_extract requires") {
		t.Fatalf("ValidateProviderFile err=%v, want custom usage_extract requires", err)
	}
}

func TestExtractUsage_UsageFactEventFilter(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.message.usage.input_tokens", Event: "message_start"},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "message_delta"},
		},
	}
	root := map[string]any{
		"message": map[string]any{
			"usage": map[string]any{
				"input_tokens": 3,
			},
		},
		"usage": map[string]any{
			"output_tokens": 7,
		},
	}

	usage, cached, err := extractUsageFromRootsWithEvent(nil, "message_start", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent message_start: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage for message_start")
	}
	if got, want := usage.InputTokens, 3; got != want {
		t.Fatalf("InputTokens got %d want %d", got, want)
	}
	if got := usage.OutputTokens; got != 0 {
		t.Fatalf("OutputTokens got %d want 0", got)
	}
	if got := cached; got != 0 {
		t.Fatalf("cached got %d want 0", got)
	}

	usage, _, err = extractUsageFromRootsWithEvent(nil, "message_delta", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent message_delta: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage for message_delta")
	}
	if got := usage.InputTokens; got != 0 {
		t.Fatalf("InputTokens got %d want 0", got)
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d want %d", got, want)
	}
}

func TestExtractUsage_UsageRootEventList(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.response.usage", Event: "response.completed|response.incomplete", EventOptional: true},
		},
		facts: []usageFactConfig{
			{Dimension: "input", Unit: "token", Path: "$.input_tokens"},
			{Dimension: "output", Unit: "token", Path: "$.output_tokens"},
		},
	}
	root := map[string]any{
		"response": map[string]any{
			"usage": map[string]any{
				"input_tokens":  5,
				"output_tokens": 8,
			},
		},
	}

	usage, _, err := extractUsageFromRootsWithEvent(nil, "response.incomplete", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.InputTokens, 5; got != want {
		t.Fatalf("InputTokens got %d want %d", got, want)
	}
	if got, want := usage.OutputTokens, 8; got != want {
		t.Fatalf("OutputTokens got %d want %d", got, want)
	}

	usage, _, err = extractUsageFromRootsWithEvent(nil, "response.created", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent mismatch: %v", err)
	}
	if usage != nil && !isAllZeroUsage(usage) {
		t.Fatalf("expected no usage for mismatched event, got %+v", *usage)
	}
}

func TestExtractUsage_UsageFactEventList(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "response.completed|response.incomplete"},
		},
	}
	root := map[string]any{
		"usage": map[string]any{
			"output_tokens": 7,
		},
	}

	usage, _, err := extractUsageFromRootsWithEvent(nil, "response.completed", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage")
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d want %d", got, want)
	}
}

func TestExtractUsage_UsageFactEventOptionalFallsBackWhenEventMissing(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "message_delta", EventOptional: true},
		},
	}
	root := map[string]any{
		"usage": map[string]any{
			"output_tokens": 7,
		},
	}

	usage, _, err := extractUsageFromRootsWithEvent(nil, "", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent empty event: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage for empty event fallback")
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d want %d", got, want)
	}

	usage, _, err = extractUsageFromRootsWithEvent(nil, "message_start", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent mismatched event: %v", err)
	}
	if usage != nil && usage.OutputTokens != 0 {
		t.Fatalf("expected mismatched non-empty event to stay filtered, got %+v", *usage)
	}
}

func TestExtractUsage_UsageFactEventOptionalFallback_DeduplicatesEquivalentRules(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "message_start", EventOptional: true},
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "message_delta", EventOptional: true},
		},
	}
	root := map[string]any{
		"usage": map[string]any{
			"output_tokens": 7,
		},
	}

	usage, _, err := extractUsageFromRootsWithEvent(nil, "", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent empty event: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage for empty event fallback")
	}
	if got, want := usage.OutputTokens, 7; got != want {
		t.Fatalf("OutputTokens got %d want %d", got, want)
	}
	if got, want := len(usage.DebugFacts), 1; got != want {
		t.Fatalf("DebugFacts len got %d want %d: %#v", got, want, usage.DebugFacts)
	}
}

func TestExtractUsage_UsageFactEventRequiredDoesNotFallbackWhenEventMissing(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		facts: []usageFactConfig{
			{Dimension: "output", Unit: "token", Path: "$.usage.output_tokens", Event: "message_delta"},
		},
	}
	root := map[string]any{
		"usage": map[string]any{
			"output_tokens": 7,
		},
	}

	usage, _, err := extractUsageFromRootsWithEvent(nil, "", cfg, nil, root, nil)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent empty event: %v", err)
	}
	if usage == nil {
		t.Fatalf("expected usage object for empty event")
	}
	if got := usage.OutputTokens; got != 0 {
		t.Fatalf("OutputTokens got %d want 0", got)
	}
	if len(usage.DebugFacts) != 0 {
		t.Fatalf("expected no debug facts, got %#v", usage.DebugFacts)
	}
}

func TestValidateProviderFile_UsageFactEventOptionalRequiresEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event-optional-invalid.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "event-optional-invalid" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    metrics {
      usage_fact output token path="$.usage.output_tokens" event_optional=true;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil || !strings.Contains(err.Error(), "event_optional requires event") {
		t.Fatalf("ValidateProviderFile err=%v, want event_optional requires event", err)
	}
}

func TestEvaluateUsageFactCountPath_EmptyWildcardStillMatches(t *testing.T) {
	root := map[string]any{
		"tool_results": []any{},
	}

	got, matched := evaluateUsageFactCountPath(root, "$.tool_results[*]", "web_search_call", "completed")
	if !matched {
		t.Fatalf("expected empty wildcard count_path to match")
	}
	if got != 0 {
		t.Fatalf("count got %v, want 0", got)
	}
}

func mustParseUsageExpr(t *testing.T, s string) *UsageExpr {
	t.Helper()
	expr, err := ParseUsageExpr(s)
	if err != nil {
		t.Fatalf("ParseUsageExpr(%q): %v", s, err)
	}
	return expr
}
