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

	usage, cachedTokens, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

	usage, _, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

func TestValidateProviderFile_UsageFactBuiltinModeAllowed(t *testing.T) {
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
      usage_extract anthropic;
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

func TestValidateProviderFile_UsageFactAllowedWithBuiltinMode(t *testing.T) {
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
      usage_extract anthropic;
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

func TestValidateProviderFile_UsageFactRequiresExplicitCustomMode(t *testing.T) {
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

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected validation error")
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
      usage_fact image.generate image path="$.usage.image_count";
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

	usage, _, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

	usage, _, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

	usage, _, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

	usage, cached, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

	usage, _, err := ExtractUsage(nil, pf.Usage.Defaults, body)
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

func TestExtractUsage_BuiltinOpenAIUsageFactOverrideCacheRead(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg := UsageExtractConfig{
		Mode: usageModeOpenAI,
		facts: []usageFactConfig{
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

	usage, cached, err := ExtractUsage(meta, cfg, body)
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

func TestExtractUsage_BuiltinAnthropicUsageFactOverrideCacheWrite(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: false}
	cfg := UsageExtractConfig{
		Mode: usageModeAnthropic,
		facts: []usageFactConfig{
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

	usage, _, err := ExtractUsage(meta, cfg, body)
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

	usage, _, err := ExtractUsage(nil, cfg, body)
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

	usage, _, err := ExtractUsage(meta, cfg, body)
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

func TestExtractUsage_UsageFactDerivedSourceWithNonJSONResponse(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeOpenAI,
		facts: []usageFactConfig{
			{Dimension: "audio.tts", Unit: "second", Source: "derived", Path: "$.audio_duration_seconds"},
		},
	}

	meta := &dslmeta.Meta{
		DerivedUsage: map[string]any{
			"audio_duration_seconds": 1.608,
		},
	}

	usage, _, err := ExtractUsage(meta, cfg, []byte("fake-audio-binary-response"))
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
      usage_extract openai;
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
