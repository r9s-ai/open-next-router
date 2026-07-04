package dsllang

import (
	"strings"
	"testing"
)

func TestCollectDiagnosticsAndSemanticTokens(t *testing.T) {
	text := `
syntax "next-router/0.1";

provider_bad "openai" {
}
`
	diags := CollectDiagnostics("file:///tmp/providers/openai.conf", text)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for invalid directive")
	}

	legend := CollectSemanticTokenLegend()
	if len(legend.TokenTypes) == 0 {
		t.Fatalf("expected semantic token legend")
	}
	tokens := CollectSemanticTokens(strings.Replace(text, "provider_bad", "provider", 1))
	if len(tokens.Data) == 0 {
		t.Fatalf("expected semantic token data")
	}
}

func TestFormatText(t *testing.T) {
	got := FormatText(`provider "openai" { defaults { auth { auth_bearer; } } }`, FormatOptions{
		TabSize:      2,
		InsertSpaces: true,
	})
	if !strings.Contains(got, "\n  defaults {") {
		t.Fatalf("unexpected formatted text:\n%s", got)
	}
}

func TestCollectHover(t *testing.T) {
	text := "provider \"x\" {\n  match api = \"chat.completions\" {\n    upstream {\n      set_path \"/v1\";\n    }\n  }\n}\n"
	hover, ok := CollectHover(text, Position{Line: 2, Character: 6})
	if !ok || hover == nil {
		t.Fatalf("expected hover")
	}
	if hover.Word != "upstream" || hover.Block != "match" {
		t.Fatalf("unexpected hover scope: %+v", hover)
	}
	if hover.Contents.Kind != "markdown" || !strings.Contains(hover.Contents.Value, "Upstream path/query routing") {
		t.Fatalf("unexpected hover contents: %+v", hover.Contents)
	}
	if hover.Range == nil || hover.Range.Start.Line != 2 || hover.Range.Start.Character != 4 {
		t.Fatalf("unexpected hover range: %+v", hover.Range)
	}
}

func TestCollectHoverUsesBlockSpecificDoc(t *testing.T) {
	text := "provider \"x\" {\n  defaults {\n    balance {\n      set_header Authorization \"x\";\n    }\n  }\n}\n"
	hover, ok := CollectHover(text, Position{Line: 3, Character: 8})
	if !ok || hover == nil {
		t.Fatalf("expected hover")
	}
	if hover.Word != "set_header" || hover.Block != "balance" {
		t.Fatalf("unexpected hover scope: %+v", hover)
	}
	if !strings.Contains(hover.Contents.Value, "balance query request") {
		t.Fatalf("expected balance-specific hover, got: %q", hover.Contents.Value)
	}
}

func TestCollectHoverReturnsFalseForWhitespace(t *testing.T) {
	text := "provider \"x\" {\n}\n"
	if hover, ok := CollectHover(text, Position{Line: 0, Character: 9}); ok || hover != nil {
		t.Fatalf("expected no hover on whitespace, got ok=%v hover=%+v", ok, hover)
	}
}
