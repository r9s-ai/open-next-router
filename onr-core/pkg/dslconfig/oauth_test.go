package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestProviderHeadersEffective_OAuthMerge(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }
	h := ProviderHeaders{
		Defaults: PhaseHeaders{
			OAuth: OAuthConfig{
				Mode:             oauthModeOpenAI,
				RefreshTokenExpr: exprChannelKey,
			},
		},
		Matches: []MatchHeaders{
			{
				API:    "chat.completions",
				Stream: boolPtr(false),
				Headers: PhaseHeaders{
					OAuth: OAuthConfig{
						TokenURLExpr: `"https://token.example.com"`,
						Form: []OAuthFormField{
							{Key: "extra", ValueExpr: `"1"`},
						},
					},
				},
			},
		},
	}

	phase, ok := h.Effective(&dslmeta.Meta{API: "chat.completions", IsStream: false, APIKey: "rk"})
	if !ok {
		t.Fatalf("Effective should match")
	}
	if got := strings.ToLower(strings.TrimSpace(phase.OAuth.Mode)); got != oauthModeOpenAI {
		t.Fatalf("mode=%q want=%q", got, oauthModeOpenAI)
	}
	resolved, rok := phase.OAuth.Resolve(&dslmeta.Meta{API: "chat.completions", APIKey: "rk"})
	if !rok {
		t.Fatalf("resolve oauth should succeed")
	}
	if got := strings.TrimSpace(resolved.TokenURL); got != "https://token.example.com" {
		t.Fatalf("token_url=%q", got)
	}
	if got := strings.TrimSpace(resolved.Form["extra"]); got != "1" {
		t.Fatalf("form extra=%q", got)
	}
}

func TestValidateProviderFile_OAuthUnknownMode(t *testing.T) {
	t.Parallel()

	path := writeProviderFile(t, "openai.conf", `
provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth {
      oauth_mode "unknown_mode";
      auth_oauth_bearer;
    }
  }
}
`)
	_, err := ValidateProviderFile(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported oauth_mode") {
		t.Fatalf("expected unsupported oauth_mode error, got: %v", err)
	}
}

func TestValidateProviderFile_OAuthCustomRequiresTokenURLAndForm(t *testing.T) {
	t.Parallel()

	path := writeProviderFile(t, "openai.conf", `
provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth {
      oauth_mode custom;
      auth_oauth_bearer;
    }
  }
}
`)
	_, err := ValidateProviderFile(path)
	if err == nil || !strings.Contains(err.Error(), "oauth_token_url is required in custom mode") {
		t.Fatalf("expected custom mode token_url error, got: %v", err)
	}
}

func TestValidateProviderFile_OAuthBuiltinOK(t *testing.T) {
	t.Parallel()

	path := writeProviderFile(t, "openai.conf", `
provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth {
      oauth_mode openai;
      oauth_refresh_token $channel.key;
      auth_oauth_bearer;
    }
  }
}
`)
	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("validate err=%v", err)
	}
}

func writeProviderFile(t *testing.T, name string, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}
	return path
}
