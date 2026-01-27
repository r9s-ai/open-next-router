package trafficdump

import "testing"

func TestMaskURLIfNeeded_RedactsGeminiKey(t *testing.T) {
	in := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse&key=AIzaSy123"
	out := maskURLIfNeeded(in, true)
	if out == in {
		t.Fatalf("expected masked url, got unchanged")
	}
	if want := "key=%5BREDACTED%5D"; !contains(out, want) {
		t.Fatalf("expected %q in %q", want, out)
	}
}

func TestMaskURLIfNeeded_DoesNotChangeWhenOff(t *testing.T) {
	in := "https://example.com/path?key=abc"
	out := maskURLIfNeeded(in, false)
	if out != in {
		t.Fatalf("out=%q want=%q", out, in)
	}
}

func TestMaskURLIfNeeded_RedactsTokenLikeKeys(t *testing.T) {
	in := "https://example.com/x?access_token=abc&foo=bar"
	out := maskURLIfNeeded(in, true)
	if !contains(out, "access_token=%5BREDACTED%5D") {
		t.Fatalf("expected access_token redacted, got %q", out)
	}
	if !contains(out, "foo=bar") {
		t.Fatalf("expected foo preserved, got %q", out)
	}
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		ok := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
