package usageestimate

import "testing"

func TestRuneClassification_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		r    rune
		fn   func(rune) bool
		want bool
	}{
		{name: "math_symbol_block", r: '∑', fn: isMathSymbol, want: true},
		{name: "math_symbol_ascii", r: '+', fn: isMathSymbol, want: false},
		{name: "url_delim_slash", r: '/', fn: isURLDelim, want: true},
		{name: "url_delim_question", r: '?', fn: isURLDelim, want: true},
		{name: "url_delim_percent", r: '%', fn: isURLDelim, want: true},
		{name: "url_delim_letter", r: 'a', fn: isURLDelim, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.r); got != tc.want {
				t.Fatalf("got=%v want=%v (r=%q)", got, tc.want, tc.r)
			}
		})
	}
}
