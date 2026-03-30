package dslconfig

import (
	"net/http"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestProviderHeadersApply_TableDriven(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	cfg := ProviderHeaders{
		Defaults: PhaseHeaders{
			Request: []HeaderOp{
				{Op: "header_set", NameExpr: `"X-Default"`, ValueExpr: `"d1"`},
				{Op: "header_set", NameExpr: `"X-Remove"`, ValueExpr: `"yes"`},
			},
		},
		Matches: []MatchHeaders{
			{
				API:    "chat.completions",
				Stream: boolPtr(true),
				Headers: PhaseHeaders{
					Request: []HeaderOp{
						{Op: "header_set", NameExpr: `"X-Stream"`, ValueExpr: `"s1"`},
						{Op: "header_del", NameExpr: `"X-Remove"`},
					},
				},
			},
			{
				API:    "chat.completions",
				Stream: boolPtr(false),
				Headers: PhaseHeaders{
					Request: []HeaderOp{
						{Op: "header_set", NameExpr: `"X-NonStream"`, ValueExpr: `"n1"`},
					},
				},
			},
		},
	}

	cases := []struct {
		name      string
		meta      *dslmeta.Meta
		wantKey   string
		wantValue string
	}{
		{
			name:      "match_stream_true",
			meta:      &dslmeta.Meta{API: "chat.completions", IsStream: true},
			wantKey:   "X-Stream",
			wantValue: "s1",
		},
		{
			name:      "match_stream_false",
			meta:      &dslmeta.Meta{API: "chat.completions", IsStream: false},
			wantKey:   "X-NonStream",
			wantValue: "n1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			cfg.Apply(tc.meta, nil, h)

			if got := h.Get("X-Default"); got != "d1" {
				t.Fatalf("X-Default=%q want=%q", got, "d1")
			}
			if got := h.Get(tc.wantKey); got != tc.wantValue {
				t.Fatalf("%s=%q want=%q", tc.wantKey, got, tc.wantValue)
			}
			if got := h.Get("X-Remove"); got != "" && tc.meta.IsStream {
				t.Fatalf("expected X-Remove deleted for stream match, got %q", got)
			}
		})
	}
}

func TestProviderHeadersApply_FilterHeaderValues(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	cfg := ProviderHeaders{
		Defaults: PhaseHeaders{
			Request: []HeaderOp{
				{Op: "header_set", NameExpr: `"anthropic-beta"`, ValueExpr: `"keep, context-1m-2025-08-07, fast-mode-fast, final"`},
				{Op: "header_filter_values", NameExpr: `"anthropic-beta"`, Patterns: []string{"context-1m-*", "fast-mode-*"}, Separator: ","},
			},
		},
		Matches: []MatchHeaders{{API: "claude.messages", Stream: boolPtr(false)}},
	}

	h := http.Header{}
	cfg.Apply(&dslmeta.Meta{API: "claude.messages", IsStream: false}, nil, h)

	if got := h.Get("anthropic-beta"); got != "keep, final" {
		t.Fatalf("anthropic-beta=%q want=%q", got, "keep, final")
	}
}

func TestProviderHeadersApply_FilterHeaderValuesCustomSeparatorAndDelete(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	cfg := ProviderHeaders{
		Matches: []MatchHeaders{
			{
				API:    "chat.completions",
				Stream: boolPtr(true),
				Headers: PhaseHeaders{
					Request: []HeaderOp{
						{Op: "header_filter_values", NameExpr: `"x-feature-flags"`, Patterns: []string{"exp-*", "debug", "*beta*"}, Separator: ";"},
					},
				},
			},
		},
	}

	h := http.Header{}
	h.Set("x-feature-flags", "exp-a ; keep ; debug ; foo-beta-bar")
	cfg.Apply(&dslmeta.Meta{API: "chat.completions", IsStream: true}, nil, h)
	if got := h.Get("x-feature-flags"); got != "keep" {
		t.Fatalf("x-feature-flags=%q want=%q", got, "keep")
	}

	h.Set("x-feature-flags", "exp-a ; debug")
	cfg.Apply(&dslmeta.Meta{API: "chat.completions", IsStream: true}, nil, h)
	if got := h.Get("x-feature-flags"); got != "" {
		t.Fatalf("expected x-feature-flags removed, got %q", got)
	}
}

func TestProviderHeadersApply_FilterHeaderValuesRunsAfterSetHeader(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	cfg := ProviderHeaders{
		Defaults: PhaseHeaders{
			Request: []HeaderOp{
				{Op: "header_set", NameExpr: `"x-feature-flags"`, ValueExpr: `"exp-one; keep; debug"`},
				{Op: "header_filter_values", NameExpr: `"x-feature-flags"`, Patterns: []string{"exp-*", "debug"}, Separator: ";"},
			},
		},
		Matches: []MatchHeaders{{API: "chat.completions", Stream: boolPtr(false)}},
	}

	h := http.Header{}
	cfg.Apply(&dslmeta.Meta{API: "chat.completions", IsStream: false}, nil, h)

	if got := h.Get("x-feature-flags"); got != "keep" {
		t.Fatalf("x-feature-flags=%q want=%q", got, "keep")
	}
}

func TestProviderHeadersApply_PassHeader(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	cfg := ProviderHeaders{
		Matches: []MatchHeaders{
			{
				API:    "claude.messages",
				Stream: boolPtr(true),
				Headers: PhaseHeaders{
					Request: []HeaderOp{
						{Op: "header_pass", NameExpr: `"anthropic-beta"`},
					},
				},
			},
		},
	}

	src := http.Header{}
	src.Add("Anthropic-Beta", "context-1m-123213, asdb")
	dst := http.Header{}
	cfg.Apply(&dslmeta.Meta{API: "claude.messages", IsStream: true}, src, dst)

	if got := dst.Values("Anthropic-Beta"); len(got) != 1 || got[0] != "context-1m-123213, asdb" {
		t.Fatalf("Anthropic-Beta=%#v", got)
	}
}

func TestProviderHeadersApply_PassHeaderOrder(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	cfg := ProviderHeaders{
		Matches: []MatchHeaders{
			{
				API:    "claude.messages",
				Stream: boolPtr(false),
				Headers: PhaseHeaders{
					Request: []HeaderOp{
						{Op: "header_pass", NameExpr: `"anthropic-beta"`},
						{Op: "header_del", NameExpr: `"anthropic-beta"`},
					},
				},
			},
		},
	}

	src := http.Header{}
	src.Set("Anthropic-Beta", "keep")
	dst := http.Header{}
	cfg.Apply(&dslmeta.Meta{API: "claude.messages", IsStream: false}, src, dst)

	if got := dst.Get("Anthropic-Beta"); got != "" {
		t.Fatalf("expected Anthropic-Beta deleted, got %q", got)
	}
}

func TestParsePassHeader(t *testing.T) {
	t.Parallel()

	conf := `
syntax "next-router/0.1";

provider "demo" {
  match api = "claude.messages" {
    request {
      pass_header "anthropic-beta";
    }
  }
}
`

	_, headers, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	if len(headers.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(headers.Matches))
	}
	ops := headers.Matches[0].Headers.Request
	if len(ops) != 1 {
		t.Fatalf("expected 1 header op, got %d", len(ops))
	}
	if ops[0].Op != "header_pass" {
		t.Fatalf("unexpected op: %q", ops[0].Op)
	}
	if ops[0].NameExpr != `"anthropic-beta"` {
		t.Fatalf("unexpected name expr: %q", ops[0].NameExpr)
	}
}

func TestParsePassHeaderRequiresName(t *testing.T) {
	t.Parallel()

	conf := `
syntax "next-router/0.1";

provider "demo" {
  match api = "claude.messages" {
    request {
      pass_header;
    }
  }
}
`

	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "pass_header", "expects header name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFilterHeaderValues(t *testing.T) {
	t.Parallel()

	conf := `
syntax "next-router/0.1";

provider "demo" {
  match api = "claude.messages" {
    request {
      filter_header_values "anthropic-beta" "context-1m-*" "fast-mode-*";
      filter_header_values "x-feature-flags" "exp-*" "debug" separator=";";
    }
  }
}
`

	_, headers, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	if len(headers.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(headers.Matches))
	}
	ops := headers.Matches[0].Headers.Request
	if len(ops) != 2 {
		t.Fatalf("expected 2 filter ops, got %d", len(ops))
	}
	if got := ops[0].Op; got != "header_filter_values" {
		t.Fatalf("ops[0].Op=%q want=%q", got, "header_filter_values")
	}
	if got := ops[0].NameExpr; got != `"anthropic-beta"` {
		t.Fatalf("rule[0].Name=%q want=%q", got, "anthropic-beta")
	}
	if got := ops[0].Separator; got != "," {
		t.Fatalf("rule[0].Separator=%q want=%q", got, ",")
	}
	if len(ops[0].Patterns) != 2 || ops[0].Patterns[0] != "context-1m-*" || ops[0].Patterns[1] != "fast-mode-*" {
		t.Fatalf("unexpected ops[0].Patterns: %#v", ops[0].Patterns)
	}
	if got := ops[1].Separator; got != ";" {
		t.Fatalf("rule[1].Separator=%q want=%q", got, ";")
	}
	if len(ops[1].Patterns) != 2 || ops[1].Patterns[0] != "exp-*" || ops[1].Patterns[1] != "debug" {
		t.Fatalf("unexpected ops[1].Patterns: %#v", ops[1].Patterns)
	}
}

func TestParseFilterHeaderValuesRequiresPattern(t *testing.T) {
	t.Parallel()

	conf := `
syntax "next-router/0.1";

provider "demo" {
  match api = "claude.messages" {
    request {
      filter_header_values "anthropic-beta";
    }
  }
}
`

	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "filter_header_values", "at least one pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProviderHeadersRejectsEmptyFilterSeparator(t *testing.T) {
	t.Parallel()

	headers := ProviderHeaders{
		Matches: []MatchHeaders{
			{
				API: "claude.messages",
				Headers: PhaseHeaders{
					Request: []HeaderOp{
						{Op: "header_filter_values", NameExpr: `"anthropic-beta"`, Patterns: []string{"context-1m-*"}, Separator: ""},
					},
				},
			},
		},
	}

	err := validateProviderHeaders("demo.conf", "demo", headers)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if got := err.Error(); got == "" || !containsAll(got, "separator", "non-empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
