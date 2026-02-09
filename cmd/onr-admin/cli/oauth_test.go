package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildOAuthAuthURL(t *testing.T) {
	t.Parallel()
	got, err := buildOAuthAuthURL(oauthAuthURLRequest{
		BaseURL:       "https://auth.openai.com/oauth/authorize",
		ClientID:      "cid",
		RedirectURI:   "http://localhost:1455/auth/callback",
		RedirectParam: "redirect_uri",
		Scope:         "openid email profile offline_access",
		State:         "state1",
		UsePKCE:       true,
		Challenge:     "challenge1",
		Params: map[string]string{
			"prompt": "login",
		},
	})
	if err != nil {
		t.Fatalf("buildOAuthAuthURL err=%v", err)
	}
	if got == "" {
		t.Fatalf("auth url is empty")
	}
}

func TestBuildOAuthAuthURL_CustomRedirectParam(t *testing.T) {
	t.Parallel()
	got, err := buildOAuthAuthURL(oauthAuthURLRequest{
		BaseURL:       "https://iflow.cn/oauth",
		ClientID:      "cid",
		RedirectURI:   "http://localhost:11451/oauth2callback",
		RedirectParam: "redirect",
		State:         "state1",
		UsePKCE:       false,
		Params: map[string]string{
			"loginMethod": "phone",
		},
	})
	if err != nil {
		t.Fatalf("buildOAuthAuthURL err=%v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse auth url err=%v", err)
	}
	q := u.Query()
	if q.Get("redirect") == "" {
		t.Fatalf("redirect should exist in query")
	}
	if q.Get("redirect_uri") != "" {
		t.Fatalf("redirect_uri should not exist for custom redirect param")
	}
}

func TestResolveOAuthProviderProfileAlias(t *testing.T) {
	t.Parallel()
	p, err := resolveOAuthProviderProfile("openai-oauth")
	if err != nil {
		t.Fatalf("resolve profile err=%v", err)
	}
	if p.Name != "openai" {
		t.Fatalf("profile name=%q want=openai", p.Name)
	}
	if p.AuthURL != defaultOpenAIAuthURL {
		t.Fatalf("auth_url=%q want=%q", p.AuthURL, defaultOpenAIAuthURL)
	}
}

func TestResolveOAuthProviderProfile_QwenKimi(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		provider string
		name     string
	}{
		{provider: "qwen", name: "qwen"},
		{provider: "kimi", name: "kimi"},
	} {
		p, err := resolveOAuthProviderProfile(tc.provider)
		if err != nil {
			t.Fatalf("resolve profile provider=%s err=%v", tc.provider, err)
		}
		if p.Name != tc.name {
			t.Fatalf("profile name=%q want=%q", p.Name, tc.name)
		}
		if p.Flow != oauthFlowDeviceCode {
			t.Fatalf("provider=%s flow=%q want=%q", tc.provider, p.Flow, oauthFlowDeviceCode)
		}
		if p.AuthURL == "" || p.TokenURL == "" || p.ClientID == "" {
			t.Fatalf("provider=%s has empty required device flow fields", tc.provider)
		}
	}
}

func TestExchangeOAuthRefreshToken_FormWithBasic(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := strings.TrimSpace(r.Header.Get("Authorization")); got == "" {
			t.Fatalf("missing basic auth header")
		}
		if ct := strings.TrimSpace(r.Header.Get("Content-Type")); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Fatalf("content-type=%q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form err=%v", err)
		}
		if got := form.Get("grant_type"); got != "authorization_code" {
			t.Fatalf("grant_type=%q", got)
		}
		if got := form.Get("client_secret"); got != "sec" {
			t.Fatalf("client_secret=%q", got)
		}
		if got := form.Get("code_verifier"); got != "verifier1" {
			t.Fatalf("code_verifier=%q", got)
		}
		_, _ = w.Write([]byte(`{"refresh_token":"rt_123"}`))
	}))
	defer srv.Close()

	got, err := exchangeOAuthRefreshToken(context.Background(), oauthTokenExchangeRequest{
		TokenURL:     srv.URL,
		ContentType:  oauthTokenContentTypeForm,
		ClientID:     "cid",
		ClientSecret: "sec",
		RedirectURI:  "http://localhost:1455/auth/callback",
		Code:         "code1",
		Verifier:     "verifier1",
		UsePKCE:      true,
		TokenBasic:   true,
	})
	if err != nil {
		t.Fatalf("exchangeOAuthRefreshToken err=%v", err)
	}
	if got != "rt_123" {
		t.Fatalf("refresh_token=%q want=rt_123", got)
	}
}

func TestRunOAuthRefreshTokenFlow_IFlowMissingClientSecret(t *testing.T) {
	t.Parallel()
	_, err := runOAuthRefreshTokenFlow(context.Background(), oauthRefreshTokenOptions{
		provider:     "iflow",
		callbackPort: 2468,
		timeout:      5 * time.Second,
	})
	if err == nil {
		t.Fatalf("expected error for missing client-secret")
	}
	if !strings.Contains(err.Error(), "ONR_OAUTH_IFLOW_CLIENT_SECRET") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestOAuthProviderClientSecretEnv(t *testing.T) {
	t.Parallel()
	if got := oauthProviderClientSecretEnv("iflow"); got != "ONR_OAUTH_IFLOW_CLIENT_SECRET" {
		t.Fatalf("env=%q", got)
	}
	if got := oauthProviderClientSecretEnv("openai-oauth"); got != "ONR_OAUTH_OPENAI_OAUTH_CLIENT_SECRET" {
		t.Fatalf("env=%q", got)
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
