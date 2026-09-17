package dsllang

import (
	"strings"
	"testing"
)

func TestJavaScriptToolingPreservesBody(t *testing.T) {
	body := "{\n const regex = /[{}]/;\n const t = ` a ${ {x:1}.x }  `;\n // include missing.conf; }\n ctx.state.x = t;\n}"
	source := `provider "p" { defaults {request {request_by_js_block ` + body + ` after_req_map {request_by_js off;} } } }`
	formatted := FormatText(source, FormatOptions{TabSize: 2, InsertSpaces: true})
	if !strings.Contains(formatted, body) {
		t.Fatalf("JS modified: %s", formatted)
	}
	if second := FormatText(formatted, FormatOptions{TabSize: 2, InsertSpaces: true}); second != formatted {
		t.Fatalf("formatter not idempotent: %s", second)
	}
	if diagnostics := AnalyzeSyntax(source); len(diagnostics) > 0 {
		t.Fatalf("JS treated as DSL: %v", diagnostics)
	}
	if diagnostics := AnalyzeSyntax(strings.Replace(source, "ctx.state.x = t;", "const = ;", 1)); len(diagnostics) == 0 {
		t.Fatal("missing JS diagnostic")
	}
}
