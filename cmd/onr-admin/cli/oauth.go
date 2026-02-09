package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultOpenAIAuthURL  = "https://auth.openai.com/oauth/authorize"
	defaultOpenAITokenURL = "https://auth.openai.com/oauth/token"
	defaultOpenAIClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
)

func newOAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oauth",
		Short: "OAuth helper",
	}
	cmd.AddCommand(newOAuthRefreshTokenCmd())
	return cmd
}

type oauthRefreshTokenOptions struct {
	noBrowser    bool
	callbackPort int
	timeout      time.Duration
	authURL      string
	tokenURL     string
	clientID     string
	redirectURI  string
}

func newOAuthRefreshTokenCmd() *cobra.Command {
	opts := oauthRefreshTokenOptions{
		callbackPort: 2468,
		timeout:      5 * time.Minute,
		authURL:      defaultOpenAIAuthURL,
		tokenURL:     defaultOpenAITokenURL,
		clientID:     defaultOpenAIClientID,
	}
	cmd := &cobra.Command{
		Use:   "refresh-token",
		Short: "Get OpenAI OAuth refresh_token",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := runOpenAIRefreshTokenFlow(cmd.Context(), opts)
			if err != nil {
				return err
			}
			fmt.Println(token)
			printRefreshTokenNextSteps(token)
			return nil
		},
	}
	fs := cmd.Flags()
	fs.BoolVar(&opts.noBrowser, "no-browser", false, "do not auto open browser")
	fs.IntVar(&opts.callbackPort, "callback-port", 2468, "local OAuth callback port")
	fs.DurationVar(&opts.timeout, "timeout", 5*time.Minute, "timeout waiting for OAuth callback")
	fs.StringVar(&opts.authURL, "auth-url", defaultOpenAIAuthURL, "OAuth authorize URL")
	fs.StringVar(&opts.tokenURL, "token-url", defaultOpenAITokenURL, "OAuth token URL")
	fs.StringVar(&opts.clientID, "client-id", defaultOpenAIClientID, "OAuth client_id")
	fs.StringVar(&opts.redirectURI, "redirect-uri", "", "OAuth redirect_uri override (default http://localhost:<callback-port>/auth/callback)")
	return cmd
}

type pkceCodes struct {
	Verifier  string
	Challenge string
}

type oauthCallback struct {
	Code  string
	State string
	Err   string
}

type openAITokenResp struct {
	RefreshToken string `json:"refresh_token"`
}

func runOpenAIRefreshTokenFlow(ctx context.Context, opts oauthRefreshTokenOptions) (string, error) {
	if opts.callbackPort <= 0 {
		return "", errors.New("callback-port must be > 0")
	}
	if opts.timeout <= 0 {
		return "", errors.New("timeout must be > 0")
	}
	pkce, err := generatePKCE()
	if err != nil {
		return "", err
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return "", err
	}

	redirectURI := strings.TrimSpace(opts.redirectURI)
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("http://localhost:%d/auth/callback", opts.callbackPort)
	}
	cbURL, err := url.Parse(redirectURI)
	if err != nil {
		return "", fmt.Errorf("invalid redirect-uri: %w", err)
	}
	if !isLoopbackHost(cbURL.Hostname()) {
		return "", errors.New("redirect-uri host must be localhost/loopback")
	}

	cbCh := make(chan oauthCallback, 1)
	srv, err := startOAuthCallbackServer(opts.callbackPort, cbCh)
	if err != nil {
		return "", err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(stopCtx)
	}()

	authURL, err := buildOpenAIAuthURL(opts.authURL, opts.clientID, redirectURI, state, pkce.Challenge)
	if err != nil {
		return "", err
	}
	fmt.Printf("Complete OpenAI OAuth login in your browser:\n%s\n", authURL)
	if !opts.noBrowser {
		_ = openBrowser(authURL)
	}

	waitCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	cb, err := waitOAuthCallback(waitCtx, cbCh)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cb.Err) != "" {
		return "", fmt.Errorf("oauth callback error: %s", cb.Err)
	}
	if strings.TrimSpace(cb.State) != strings.TrimSpace(state) {
		return "", errors.New("oauth state mismatch")
	}
	if strings.TrimSpace(cb.Code) == "" {
		return "", errors.New("oauth callback code is empty")
	}

	return exchangeOpenAIRefreshToken(waitCtx, opts.tokenURL, opts.clientID, redirectURI, cb.Code, pkce.Verifier)
}

func startOAuthCallbackServer(port int, cbCh chan<- oauthCallback) (*http.Server, error) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("callback port %d unavailable: %w", port, err)
	}
	_ = ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cb := oauthCallback{
			Code:  strings.TrimSpace(q.Get("code")),
			State: strings.TrimSpace(q.Get("state")),
			Err:   strings.TrimSpace(q.Get("error")),
		}
		select {
		case cbCh <- cb:
		default:
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("OAuth completed. You can close this page."))
	})

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() { _ = srv.ListenAndServe() }()
	time.Sleep(100 * time.Millisecond)
	return srv, nil
}

func waitOAuthCallback(ctx context.Context, cbCh <-chan oauthCallback) (oauthCallback, error) {
	select {
	case <-ctx.Done():
		return oauthCallback{}, fmt.Errorf("wait oauth callback timeout: %w", ctx.Err())
	case cb := <-cbCh:
		return cb, nil
	}
}

func exchangeOpenAIRefreshToken(
	ctx context.Context,
	tokenURL string,
	clientID string,
	redirectURI string,
	code string,
	verifier string,
) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", strings.TrimSpace(clientID))
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", strings.TrimSpace(redirectURI))
	form.Set("code_verifier", strings.TrimSpace(verifier))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(tokenURL), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read token response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr openAITokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("parse token response failed: %w", err)
	}
	token := strings.TrimSpace(tr.RefreshToken)
	if token == "" {
		return "", errors.New("refresh_token not found in token response")
	}
	return token, nil
}

func buildOpenAIAuthURL(baseURL string, clientID string, redirectURI string, state string, challenge string) (string, error) {
	b := strings.TrimSpace(baseURL)
	if b == "" {
		return "", errors.New("auth-url is empty")
	}
	u, err := url.Parse(b)
	if err != nil {
		return "", fmt.Errorf("invalid auth-url: %w", err)
	}
	q := u.Query()
	q.Set("client_id", strings.TrimSpace(clientID))
	q.Set("response_type", "code")
	q.Set("redirect_uri", strings.TrimSpace(redirectURI))
	q.Set("scope", "openid email profile offline_access")
	q.Set("state", strings.TrimSpace(state))
	q.Set("code_challenge", strings.TrimSpace(challenge))
	q.Set("code_challenge_method", "S256")
	q.Set("prompt", "login")
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func generatePKCE() (pkceCodes, error) {
	verifier, err := randomURLSafe(64)
	if err != nil {
		return pkceCodes{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return pkceCodes{Verifier: verifier, Challenge: challenge}, nil
}

func randomURLSafe(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random generation failed: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func openBrowser(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("empty url")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "localhost" || h == "::1" || h == "127.0.0.1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func printRefreshTokenNextSteps(token string) {
	masked := maskToken(token)
	fmt.Fprintf(os.Stderr,
		"\nNext steps:\n1. Save this token to keys.yaml (or your secret manager) as an upstream key value.\n2. Use that key in your OAuth provider channel (e.g. oauth_refresh_token $channel.key;).\n3. Restart ONR and test the Responses API.\nToken (masked): %s\n",
		masked,
	)
}

func maskToken(token string) string {
	t := strings.TrimSpace(token)
	if len(t) <= 10 {
		return t
	}
	return t[:6] + "..." + t[len(t)-4:]
}
