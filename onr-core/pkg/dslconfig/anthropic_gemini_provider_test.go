package dslconfig

import (
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestValidateProviderFile_AnthropicUsesPathSpecificUsageModes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "config", "providers", "anthropic.conf")
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	cases := []struct {
		api       string
		stream    bool
		dimension string
		unit      string
		path      string
	}{
		{
			api:       "chat.completions",
			stream:    false,
			dimension: "input",
			unit:      "token",
			path:      "$.usage.prompt_tokens",
		},
		{
			api:       "claude.messages",
			stream:    false,
			dimension: "cache_write",
			unit:      "token",
			path:      "$.usage.cache_creation.ephemeral_5m_input_tokens",
		},
		{
			api:       "claude.messages",
			stream:    true,
			dimension: "cache_write",
			unit:      "token",
			path:      "$.message.usage.cache_creation.ephemeral_5m_input_tokens",
		},
		{
			api:       "claude.messages",
			stream:    false,
			dimension: "server_tool.web_search",
			unit:      "call",
			path:      "$.usage.server_tool_use.web_search_requests",
		},
		{
			api:       "claude.messages",
			stream:    true,
			dimension: "server_tool.web_search",
			unit:      "call",
			path:      "$.message.usage.server_tool_use.web_search_requests",
		},
	}

	for _, tc := range cases {
		meta := &dslmeta.Meta{API: tc.api, IsStream: tc.stream}
		cfg, ok := pf.Usage.Select(meta)
		if !ok {
			t.Fatalf("expected usage config for api=%q stream=%v", tc.api, tc.stream)
		}
		if got := normalizeUsageMode(cfg.Mode); got != usageModeCustom {
			t.Fatalf("api=%q stream=%v usage mode=%q want=%q", tc.api, tc.stream, cfg.Mode, usageModeCustom)
		}
		facts := cfg.CompiledFacts(meta)
		found := false
		for _, fact := range facts {
			if fact.Dimension != tc.dimension || fact.Unit != tc.unit {
				continue
			}
			if tc.path != "" && fact.Path != tc.path {
				continue
			}
			found = true
			break
		}
		if !found {
			t.Fatalf("api=%q stream=%v compiled facts missing expected fact: %#v", tc.api, tc.stream, facts)
		}
	}
}

func TestValidateProviderFile_GeminiUsesPathSpecificUsageModes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "config", "providers", "gemini.conf")
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	cases := []struct {
		api       string
		stream    bool
		dimension string
		unit      string
		path      string
	}{
		{
			api:       "gemini.generateContent",
			stream:    false,
			dimension: "input",
			unit:      "token",
			path:      "$.usageMetadata.promptTokenCount",
		},
		{
			api:       "gemini.streamGenerateContent",
			stream:    true,
			dimension: "output",
			unit:      "token",
			path:      "$.usageMetadata.candidatesTokenCount",
		},
		{
			api:       "chat.completions",
			stream:    false,
			dimension: "input",
			unit:      "token",
			path:      "$.usage.prompt_tokens",
		},
	}

	for _, tc := range cases {
		meta := &dslmeta.Meta{API: tc.api, IsStream: tc.stream}
		cfg, ok := pf.Usage.Select(meta)
		if !ok {
			t.Fatalf("expected usage config for api=%q stream=%v", tc.api, tc.stream)
		}
		if got := normalizeUsageMode(cfg.Mode); got != usageModeCustom {
			t.Fatalf("api=%q stream=%v usage mode=%q want=%q", tc.api, tc.stream, cfg.Mode, usageModeCustom)
		}
		facts := cfg.CompiledFacts(meta)
		found := false
		for _, fact := range facts {
			if fact.Dimension == tc.dimension && fact.Unit == tc.unit && fact.Path == tc.path {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("api=%q stream=%v compiled facts missing expected fact: %#v", tc.api, tc.stream, facts)
		}
	}
}

func TestValidateProviderFile_AnthropicUsesPathSpecificFinishReasonModes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "config", "providers", "anthropic.conf")
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	cases := []struct {
		api      string
		stream   bool
		wantMode string
		paths    []finishReasonPathConfig
	}{
		{
			api:      "chat.completions",
			stream:   false,
			wantMode: usageModeCustom,
			paths:    []finishReasonPathConfig{{Path: "$.choices[*].finish_reason"}},
		},
		{
			api:      "claude.messages",
			stream:   false,
			wantMode: usageModeCustom,
			paths:    []finishReasonPathConfig{{Path: "$.stop_reason"}},
		},
		{
			api:      "claude.messages",
			stream:   true,
			wantMode: usageModeCustom,
			paths: []finishReasonPathConfig{
				{Path: "$.delta.stop_reason"},
				{Path: "$.message.stop_reason", Fallback: true},
			},
		},
	}

	for _, tc := range cases {
		meta := &dslmeta.Meta{API: tc.api, IsStream: tc.stream}
		cfg, ok := pf.Finish.Select(meta)
		if !ok {
			t.Fatalf("expected finish_reason config for api=%q stream=%v", tc.api, tc.stream)
		}
		if got := normalizeFinishReasonMode(cfg.Mode); got != tc.wantMode {
			t.Fatalf("api=%q stream=%v finish mode=%q want=%q", tc.api, tc.stream, cfg.Mode, tc.wantMode)
		}
		rules := cfg.finishReasonPathConfigs()
		if got, want := len(rules), len(tc.paths); got != want {
			t.Fatalf("api=%q stream=%v path rules len=%d want=%d", tc.api, tc.stream, got, want)
		}
		for i, want := range tc.paths {
			if rules[i].Path != want.Path || rules[i].Fallback != want.Fallback {
				t.Fatalf("api=%q stream=%v rule[%d]=%+v want=%+v", tc.api, tc.stream, i, rules[i], want)
			}
		}
	}
}

func TestValidateProviderFile_GeminiUsesPathSpecificFinishReasonModes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "config", "providers", "gemini.conf")
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	cases := []struct {
		api      string
		stream   bool
		wantMode string
		paths    []finishReasonPathConfig
	}{
		{
			api:      "gemini.generateContent",
			stream:   false,
			wantMode: usageModeCustom,
			paths: []finishReasonPathConfig{
				{Path: "$.candidates[*].finishReason"},
				{Path: "$.candidates[*].finish_reason", Fallback: true},
			},
		},
		{
			api:      "gemini.streamGenerateContent",
			stream:   true,
			wantMode: usageModeCustom,
			paths: []finishReasonPathConfig{
				{Path: "$.candidates[*].finishReason"},
				{Path: "$.candidates[*].finish_reason", Fallback: true},
			},
		},
		{
			api:      "chat.completions",
			stream:   false,
			wantMode: usageModeCustom,
			paths:    []finishReasonPathConfig{{Path: "$.choices[*].finish_reason"}},
		},
	}

	for _, tc := range cases {
		meta := &dslmeta.Meta{API: tc.api, IsStream: tc.stream}
		cfg, ok := pf.Finish.Select(meta)
		if !ok {
			t.Fatalf("expected finish_reason config for api=%q stream=%v", tc.api, tc.stream)
		}
		if got := normalizeFinishReasonMode(cfg.Mode); got != tc.wantMode {
			t.Fatalf("api=%q stream=%v finish mode=%q want=%q", tc.api, tc.stream, cfg.Mode, tc.wantMode)
		}
		rules := cfg.finishReasonPathConfigs()
		if got, want := len(rules), len(tc.paths); got != want {
			t.Fatalf("api=%q stream=%v path rules len=%d want=%d", tc.api, tc.stream, got, want)
		}
		for i, want := range tc.paths {
			if rules[i].Path != want.Path || rules[i].Fallback != want.Fallback {
				t.Fatalf("api=%q stream=%v rule[%d]=%+v want=%+v", tc.api, tc.stream, i, rules[i], want)
			}
		}
	}
}
