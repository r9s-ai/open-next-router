package jslex

import (
	"strings"
	"testing"
)

func TestJavaScriptBoundaries(t *testing.T) {
	bodies := []string{
		`{const x = /[{}]/g; const n = 10 / 2; if(x.test("{")) { ctx.state.n=n; }}`,
		"{const x = `a${ {x: `b${1 + 2}`}.x }`; // } include bad;\n /* } */ return;}",
		`{const x="include missing.conf; }"; const include = "bad";}`,
		`{if(true) /}/.test("}"); else {} return /\/*}/;}`,
	}
	for _, body := range bodies {
		source := `provider "p" { request_by_js_block ` + body + ` request_by_js off; }`
		islands, err := Islands(source)
		if err != nil {
			t.Fatal(err)
		}
		if len(islands) != 1 || source[islands[0].Start:islands[0].End] != body {
			t.Fatalf("wrong boundary: %v", islands)
		}
		masked, err := Mask(source)
		if err != nil || len(masked) != len(source) || strings.Count(masked, "\n") != strings.Count(source, "\n") {
			t.Fatal("offsets changed")
		}
	}
	for _, source := range []string{`request_by_js_block { if( }`, "request_by_js_block { const t = `unterminated", `request_by_js_block { while(true) {}`} {
		if _, err := Islands(source); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
func FuzzBodyEnd(f *testing.F) {
	for _, s := range []string{`{}`, `{return /}/;}`, "{return `a${1}`;}"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		end, err := BodyEnd(s)
		if err == nil && (end < 2 || end > len(s) || s[end-1] != '}') {
			t.Fatalf("bad end %d", end)
		}
	})
}
