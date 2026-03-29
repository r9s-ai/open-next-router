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
		"images.generations":   "/v1/images/generations",
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

	usageCfg, ok := pf.Usage.Select(&dslmeta.Meta{API: "images.generations", IsStream: false})
	if !ok {
		t.Fatalf("expected usage config for images.generations")
	}
	if usageCfg.Mode != "openai" {
		t.Fatalf("images.generations usage mode=%q want=openai", usageCfg.Mode)
	}
}
