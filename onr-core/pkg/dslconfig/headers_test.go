package dslconfig

import (
	"net/http"
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
			cfg.Apply(tc.meta, h)

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
