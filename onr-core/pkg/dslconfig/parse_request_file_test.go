package dslconfig

import (
	"strings"
	"testing"
)

func TestParseReqInlineFile_DefaultsAndOverrides(t *testing.T) {
	req := parseValidationConf(t, `
      req_inline_file field="image";
      req_inline_file field="mask" max_bytes=1024 max_count=1;
`)
	rules := req.Defaults.InlineFiles
	if len(rules) != 2 {
		t.Fatalf("rules=%d want 2", len(rules))
	}
	// Order is preserved because it is the order the fields land in the root,
	// and a builtin reads them positionally.
	if rules[0].Field != "image" || rules[1].Field != "mask" {
		t.Fatalf("fields=%q/%q", rules[0].Field, rules[1].Field)
	}
	if rules[0].MaxBytes != reqInlineFileDefaultMaxBytes || rules[0].MaxCount != reqInlineFileDefaultMaxCount {
		t.Fatalf("defaults not applied: %+v", rules[0])
	}
	if rules[1].MaxBytes != 1024 || rules[1].MaxCount != 1 {
		t.Fatalf("overrides not applied: %+v", rules[1])
	}
}

// The caps exist so a misconfigured rule cannot let one request pull an
// unbounded amount of client data into memory; a config that asks for more than
// the ceiling must fail loudly rather than be silently clamped.
func TestParseReqInlineFile_RejectsBadOptions(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"missing field":   {`req_inline_file max_bytes=1024;`, "requires field"},
		"zero bytes":      {`req_inline_file field="image" max_bytes=0;`, "max_bytes"},
		"bytes over cap":  {`req_inline_file field="image" max_bytes=134217728;`, "max_bytes"},
		"zero count":      {`req_inline_file field="image" max_count=0;`, "max_count"},
		"count over cap":  {`req_inline_file field="image" max_count=99;`, "max_count"},
		"unknown option":  {`req_inline_file field="image" nope=1;`, "unsupported req_inline_file option"},
		"bytes not a num": {`req_inline_file field="image" max_bytes="big";`, "max_bytes"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			conf := `
syntax "next-router/0.1";

provider "demo" {
  defaults {
    request {
      ` + tc.body + `
    }
  }
}
`
			_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
			if err == nil {
				t.Fatalf("expected a parse error for %q", tc.body)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%q should mention %q", err.Error(), tc.want)
			}
		})
	}
}

// A match block inherits the provider defaults' rules and adds its own, the
// same way validation rules and JSON ops merge.
func TestReqInlineFile_MergesDefaultsIntoMatch(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "demo" {
  defaults {
    request {
      req_inline_file field="image";
    }
  }

  match api = "images.edits" {
    request {
      req_inline_file field="mask" max_count=1;
    }
  }
}
`
	_, _, req, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	if len(req.Matches) != 1 {
		t.Fatalf("matches=%d want 1", len(req.Matches))
	}
	if got := req.Matches[0].Transform.InlineFiles; len(got) != 1 || got[0].Field != "mask" {
		t.Fatalf("match rules=%+v want just its own before merge", got)
	}
	if got := req.Defaults.InlineFiles; len(got) != 1 || got[0].Field != "image" {
		t.Fatalf("defaults rules=%+v", got)
	}
}
