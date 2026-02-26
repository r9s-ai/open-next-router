package main

import (
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"
)

func TestReplaceGeneratedRegion(t *testing.T) {
	original := "A\n" + beginMarker + "\nold\n" + endMarker + "\nB\n"
	got, err := replaceGeneratedRegion(original, "new-line")
	if err != nil {
		t.Fatalf("replaceGeneratedRegion error: %v", err)
	}
	if !strings.Contains(got, "new-line") {
		t.Fatalf("expected generated body in output, got: %q", got)
	}
	if strings.Contains(got, "old") {
		t.Fatalf("old content should be replaced, got: %q", got)
	}
}

func TestReplaceGeneratedRegion_MissingMarkers(t *testing.T) {
	if _, err := replaceGeneratedRegion("no markers", "x"); err == nil {
		t.Fatalf("expected error when markers are missing")
	}
}

func TestRenderReference(t *testing.T) {
	spec, err := dslspec.LoadBuiltinSpec()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	locale, err := dslspec.LoadBuiltinLocale("en")
	if err != nil {
		t.Fatalf("load locale: %v", err)
	}
	out := renderReference(spec, locale)
	if !strings.Contains(out, "| Directive | Args | Modes | Repeatable | Summary |") {
		t.Fatalf("expected markdown table header, got: %q", out)
	}
	if !strings.Contains(out, "`syntax`") {
		t.Fatalf("expected at least one directive row, got: %q", out)
	}
	if !strings.Contains(out, "Semantics depend on block context.") {
		t.Fatalf("expected ambiguous-summary fallback in output, got: %q", out)
	}
}
