package cli

import "testing"

func TestBuildOpenAIAuthURL(t *testing.T) {
	t.Parallel()
	got, err := buildOpenAIAuthURL(
		"https://auth.openai.com/oauth/authorize",
		"cid",
		"http://localhost:1455/auth/callback",
		"state1",
		"challenge1",
	)
	if err != nil {
		t.Fatalf("buildOpenAIAuthURL err=%v", err)
	}
	if got == "" {
		t.Fatalf("auth url is empty")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		host string
		ok   bool
	}{
		{host: "localhost", ok: true},
		{host: "127.0.0.1", ok: true},
		{host: "::1", ok: true},
		{host: "example.com", ok: false},
	} {
		if got := isLoopbackHost(tc.host); got != tc.ok {
			t.Fatalf("host=%q got=%v want=%v", tc.host, got, tc.ok)
		}
	}
}

func TestGeneratePKCE(t *testing.T) {
	t.Parallel()
	p, err := generatePKCE()
	if err != nil {
		t.Fatalf("generatePKCE err=%v", err)
	}
	if p.Verifier == "" || p.Challenge == "" {
		t.Fatalf("pkce empty: %+v", p)
	}
}
