package main

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"
)

func TestMetadataSummaryByName_AmbiguousDirectives(t *testing.T) {
	_, ambiguous := metadataSummaryByName()
	for _, name := range []string{"json_set", "json_del", "set_header", "path"} {
		if !ambiguous[name] {
			t.Fatalf("expected ambiguous summary for %q", name)
		}
	}
}

func TestBuildLocale_AmbiguousSummaryOverridesExisting(t *testing.T) {
	spec := dslspec.Spec{
		Version: "next-router/0.1",
		Blocks: []dslspec.BlockSpec{
			{ID: "request", Kind: "block"},
			{ID: "response", Kind: "block"},
		},
		Directives: []dslspec.DirectiveSpec{
			{
				ID:        "json_set",
				Name:      "json_set",
				AllowedIn: []string{"request", "response"},
				Kind:      "statement",
			},
		},
	}
	existing := dslspec.LocaleBundle{
		Locale: "en",
		BlockTitles: map[string]string{
			"request":  "Request",
			"response": "Response",
		},
		DirectiveText: map[string]dslspec.DirectiveText{
			"json_set": {Summary: "OLD SUMMARY"},
		},
	}

	got := buildLocale(spec, existing, false)
	if got.DirectiveText["json_set"].Summary != "Semantics depend on block context." {
		t.Fatalf("unexpected summary: %q", got.DirectiveText["json_set"].Summary)
	}
}
