package dslconfig

import "testing"

func TestScannerNext_ScansCommentsAndStrings(t *testing.T) {
	s := newScanner("test.conf", "# line\n// slash\n'quoted'\n\"double\"\n")

	var got []tokenKind
	for {
		tok := s.next()
		got = append(got, tok.kind)
		if tok.kind == tokEOF {
			break
		}
	}

	want := []tokenKind{
		tokComment,
		tokWhitespace,
		tokComment,
		tokWhitespace,
		tokString,
		tokWhitespace,
		tokString,
		tokWhitespace,
		tokEOF,
	}
	if len(got) != len(want) {
		t.Fatalf("token count=%d want=%d got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token[%d]=%v want=%v all=%v", i, got[i], want[i], got)
		}
	}
}

func TestScannerNext_DoesNotTreatSlashStarAsComment(t *testing.T) {
	s := newScanner("test.conf", "include providers/*.conf;\n")

	var got []tokenKind
	for {
		tok := s.next()
		got = append(got, tok.kind)
		if tok.kind == tokEOF {
			break
		}
	}

	want := []tokenKind{
		tokIdent,
		tokWhitespace,
		tokIdent,
		tokOther,
		tokOther,
		tokOther,
		tokIdent,
		tokSemicolon,
		tokWhitespace,
		tokEOF,
	}
	if len(got) != len(want) {
		t.Fatalf("token count=%d want=%d got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token[%d]=%v want=%v all=%v", i, got[i], want[i], got)
		}
	}
}

func TestFindProviderName_SingleQuotedProvider(t *testing.T) {
	name, err := findProviderName("test.conf", "provider 'gemini' { }")
	if err != nil {
		t.Fatalf("findProviderName error: %v", err)
	}
	if name != "gemini" {
		t.Fatalf("provider=%q want=gemini", name)
	}
}
