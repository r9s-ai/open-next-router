package dslconfig

import (
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestValidateProviderFile_OpenAIIncludesImageAndAudioRoutes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "config", "providers", "openai.conf")
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	cases := map[string]string{
		"completions":          "/v1/completions",
		"images.generations":   "/v1/images/generations",
		"images.edits":         "/v1/images/edits",
		"audio.speech":         "/v1/audio/speech",
		"audio.transcriptions": "/v1/audio/transcriptions",
		"audio.translations":   "/v1/audio/translations",
	}

	for api, wantPath := range cases {
		match, ok := pf.Routing.selectMatch(api, false)
		if !ok {
			t.Fatalf("expected route match for api=%q", api)
		}
		if match.SetPath != `"`+wantPath+`"` {
			t.Fatalf("api=%q set_path=%q want=%q", api, match.SetPath, `"`+wantPath+`"`)
		}
	}

	usageCases := map[string]struct {
		dimension string
		unit      string
		path      string
		countPath string
		source    string
		expr      string
	}{
		"chat.completions": {
			dimension: "input",
			unit:      "token",
			path:      "$.usage.prompt_tokens",
		},
		"completions": {
			dimension: "input",
			unit:      "token",
			path:      "$.usage.prompt_tokens",
		},
		"responses": {
			dimension: "server_tool.web_search",
			unit:      "call",
			countPath: "$.output[*]",
		},
		"embeddings": {
			dimension: "output",
			unit:      "token",
			expr:      "0",
		},
		"images.generations": {
			dimension: "image.generate",
			unit:      "image",
			countPath: "$.data[*]",
		},
		"images.edits": {
			dimension: "image.edit",
			unit:      "image",
			countPath: "$.data[*]",
		},
		"audio.speech": {
			dimension: "audio.tts",
			unit:      "second",
			path:      "$.audio_duration_seconds",
			source:    "derived",
		},
		"audio.transcriptions": {
			dimension: "audio.stt",
			unit:      "second",
			path:      "$.usage.seconds",
		},
		"audio.translations": {
			dimension: "audio.translate",
			unit:      "second",
			path:      "$.usage.seconds",
		},
		"claude.messages": {
			dimension: "input",
			unit:      "token",
			path:      "$.usage.prompt_tokens",
		},
	}
	for api, want := range usageCases {
		usageCfg, ok := pf.Usage.Select(&dslmeta.Meta{API: api, IsStream: false})
		if !ok {
			t.Fatalf("expected usage config for %s", api)
		}
		if got := normalizeUsageMode(usageCfg.Mode); got != usageModeCustom {
			t.Fatalf("%s usage mode=%q want=%q", api, usageCfg.Mode, usageModeCustom)
		}
		facts := usageCfg.CompiledFacts(&dslmeta.Meta{API: api, IsStream: false})
		found := false
		for _, fact := range facts {
			if fact.Dimension != want.dimension || fact.Unit != want.unit {
				continue
			}
			if want.path != "" && fact.Path != want.path {
				continue
			}
			if want.countPath != "" && fact.CountPath != want.countPath {
				continue
			}
			if want.source != "" && fact.Source != want.source {
				continue
			}
			if want.expr != "" && fact.Expr != want.expr {
				continue
			}
			found = true
			break
		}
		if !found {
			t.Fatalf("%s compiled facts missing expected fact: %#v", api, facts)
		}
	}

	chatUsageCfg, ok := pf.Usage.Select(&dslmeta.Meta{API: "chat.completions", IsStream: false})
	if !ok {
		t.Fatalf("expected usage config for chat.completions")
	}
	chatFacts := chatUsageCfg.CompiledFacts(&dslmeta.Meta{API: "chat.completions", IsStream: false})
	var foundWebSearch bool
	for _, fact := range chatFacts {
		if fact.Dimension == "server_tool.web_search" && fact.Unit == "call" && fact.CountPath == "$.choices[*].message.annotations" {
			foundWebSearch = true
			break
		}
	}
	if !foundWebSearch {
		t.Fatalf("chat.completions compiled facts missing web search annotation fact: %#v", chatFacts)
	}
}

func TestValidateProviderFile_OpenAIUsesPathSpecificFinishReasonModes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "config", "providers", "openai.conf")
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
			api:      "completions",
			stream:   false,
			wantMode: usageModeCustom,
			paths:    []finishReasonPathConfig{{Path: "$.choices[*].finish_reason"}},
		},
		{
			api:      "responses",
			stream:   false,
			wantMode: usageModeCustom,
			paths: []finishReasonPathConfig{
				{Path: "$.incomplete_details.reason"},
				{Path: "$.response.incomplete_details.reason", Fallback: true},
			},
		},
		{
			api:      "claude.messages",
			stream:   true,
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
