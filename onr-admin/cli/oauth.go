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
	providerOpenAI = "openai"

	defaultOpenAIAuthURL = "https://auth.openai.com/oauth/authorize"
	// #nosec G101 -- OAuth endpoint URL, not a credential.
	defaultOpenAITokenURL = "https://auth.openai.com/oauth/token"
	defaultOpenAIClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	oauthTokenContentTypeForm = "form"
	oauthTokenContentTypeJSON = "json"

	oauthFlowAuthCode   = "auth_code"
	oauthFlowDeviceCode = "device_code"
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
	provider     string
	noBrowser    bool
	callbackPort int
	timeout      time.Duration
	authURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	scope        string
	redirectURI  string
	authParams   []string
	tokenType    string
	tokenBasic   bool
	noPKCE       bool
}

func newOAuthRefreshTokenCmd() *cobra.Command {
	opts := oauthRefreshTokenOptions{
		callbackPort: 2468,
		timeout:      5 * time.Minute,
	}
	cmd := &cobra.Command{
		Use:   "refresh-token",
		Short: "Get OAuth refresh_token for a provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := runOAuthRefreshTokenFlow(cmd.Context(), opts)
			if err != nil {
				return err
			}
			fmt.Println(token)
			printRefreshTokenNextSteps(opts.provider, token)
			return nil
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.provider, "provider", "p", "", "OAuth provider profile: openai|claude|gemini|antigravity|iflow|qwen|kimi|custom")
	fs.BoolVar(&opts.noBrowser, "no-browser", false, "do not auto open browser")
	fs.IntVar(&opts.callbackPort, "callback-port", 2468, "local OAuth callback port")
	fs.DurationVar(&opts.timeout, "timeout", 5*time.Minute, "timeout waiting for OAuth callback")
	fs.StringVar(&opts.authURL, "auth-url", "", "OAuth authorize URL override")
	fs.StringVar(&opts.tokenURL, "token-url", "", "OAuth token URL override")
	fs.StringVar(&opts.clientID, "client-id", "", "OAuth client_id override")
	fs.StringVar(&opts.clientSecret, "client-secret", "", "OAuth client_secret override")
	fs.StringVar(&opts.scope, "scope", "", "OAuth scope override")
	fs.StringVar(&opts.redirectURI, "redirect-uri", "", "OAuth redirect_uri override (default http://localhost:<callback-port>/auth/callback)")
	fs.StringArrayVar(&opts.authParams, "auth-param", nil, "extra OAuth authorize query param key=value (repeatable)")
	fs.StringVar(&opts.tokenType, "token-content-type", "", "token request content type: form|json")
	fs.BoolVar(&opts.tokenBasic, "token-basic-auth", false, "send token request Authorization Basic(client_id:client_secret)")
	fs.BoolVar(&opts.noPKCE, "no-pkce", false, "disable PKCE code_challenge/code_verifier")
	_ = cmd.MarkFlagRequired("provider")
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

type oauthTokenResp struct {
	RefreshToken string `json:"refresh_token"`
}

type oauthProviderProfile struct {
	Flow          string
	Name          string
	AuthURL       string
	TokenURL      string
	ClientID      string
	ClientSecret  string
	Scope         string
	AuthParams    map[string]string
	RedirectParam string
	UsePKCE       bool
	TokenType     string
	TokenBasic    bool
	DeviceGrant   string
}

func resolveOAuthProviderProfile(provider string) (oauthProviderProfile, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = providerOpenAI
	}
	switch p {
	case providerOpenAI, "openai-oauth", "codex":
		return oauthProviderProfile{
			Flow:     oauthFlowAuthCode,
			Name:     providerOpenAI,
			AuthURL:  defaultOpenAIAuthURL,
			TokenURL: defaultOpenAITokenURL,
			ClientID: defaultOpenAIClientID,
			Scope:    "openid email profile offline_access",
			AuthParams: map[string]string{
				"prompt":                     "login",
				"id_token_add_organizations": "true",
				"codex_cli_simplified_flow":  "true",
			},
			RedirectParam: "redirect_uri",
			UsePKCE:       true,
			TokenType:     oauthTokenContentTypeForm,
		}, nil
	case "claude", "anthropic":
		return oauthProviderProfile{
			Flow:          oauthFlowAuthCode,
			Name:          "claude",
			AuthURL:       "https://claude.ai/oauth/authorize",
			TokenURL:      "https://console.anthropic.com/v1/oauth/token",
			ClientID:      "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
			Scope:         "org:create_api_key user:profile user:inference",
			AuthParams:    map[string]string{"code": "true"},
			RedirectParam: "redirect_uri",
			UsePKCE:       true,
			TokenType:     oauthTokenContentTypeJSON,
		}, nil
	case "gemini":
		return oauthProviderProfile{
			Flow:     oauthFlowAuthCode,
			Name:     "gemini",
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
			Scope: strings.Join([]string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			}, " "),
			AuthParams: map[string]string{
				"access_type": "offline",
				"prompt":      "consent",
			},
			RedirectParam: "redirect_uri",
			UsePKCE:       true,
			TokenType:     oauthTokenContentTypeForm,
		}, nil
	case "antigravity":
		return oauthProviderProfile{
			Flow:     oauthFlowAuthCode,
			Name:     "antigravity",
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
			Scope: strings.Join([]string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/cclog",
				"https://www.googleapis.com/auth/experimentsandconfigs",
			}, " "),
			AuthParams: map[string]string{
				"access_type": "offline",
				"prompt":      "consent",
			},
			RedirectParam: "redirect_uri",
			UsePKCE:       false,
			TokenType:     oauthTokenContentTypeForm,
		}, nil
	case "iflow":
		return oauthProviderProfile{
			Flow:          oauthFlowAuthCode,
			Name:          "iflow",
			AuthURL:       "https://iflow.cn/oauth",
			TokenURL:      "https://iflow.cn/oauth/token",
			ClientID:      "10009311001",
			AuthParams:    map[string]string{"loginMethod": "phone", "type": "phone"},
			RedirectParam: "redirect",
			UsePKCE:       false,
			TokenType:     oauthTokenContentTypeForm,
			TokenBasic:    true,
		}, nil
	case "qwen":
		return oauthProviderProfile{
			Flow:        oauthFlowDeviceCode,
			Name:        "qwen",
			AuthURL:     "https://chat.qwen.ai/api/v1/oauth2/device/code",
			TokenURL:    "https://chat.qwen.ai/api/v1/oauth2/token",
			ClientID:    "f0304373b74a44d2b584a3fb70ca9e56",
			Scope:       "openid profile email model.completion",
			UsePKCE:     true,
			TokenType:   oauthTokenContentTypeForm,
			DeviceGrant: "urn:ietf:params:oauth:grant-type:device_code",
		}, nil
	case "kimi":
		return oauthProviderProfile{
			Flow:        oauthFlowDeviceCode,
			Name:        "kimi",
			AuthURL:     "https://auth.kimi.com/api/oauth/device_authorization",
			TokenURL:    "https://auth.kimi.com/api/oauth/token",
			ClientID:    "17e5f671-d194-4dfb-9706-5516cb48c098",
			UsePKCE:     false,
			TokenType:   oauthTokenContentTypeForm,
			DeviceGrant: "urn:ietf:params:oauth:grant-type:device_code",
		}, nil
	case "custom":
		return oauthProviderProfile{
			Flow:          oauthFlowAuthCode,
			Name:          "custom",
			AuthParams:    map[string]string{},
			RedirectParam: "redirect_uri",
			UsePKCE:       true,
			TokenType:     oauthTokenContentTypeForm,
		}, nil
	default:
		return oauthProviderProfile{}, fmt.Errorf(
			"unsupported provider %q for oauth refresh-token; supported: openai, claude, gemini, antigravity, iflow, qwen, kimi, custom",
			provider,
		)
	}
}

func runOAuthRefreshTokenFlow(ctx context.Context, opts oauthRefreshTokenOptions) (string, error) {
	profile, err := validateAndResolveOAuthProfile(opts)
	if err != nil {
		return "", err
	}
	if profile.Flow == oauthFlowDeviceCode {
		return runOAuthDeviceCodeRefreshTokenFlow(ctx, opts, profile)
	}
	return runOAuthAuthCodeRefreshTokenFlow(ctx, opts, profile)
}

func runOAuthAuthCodeRefreshTokenFlow(ctx context.Context, opts oauthRefreshTokenOptions, profile oauthProviderProfile) (string, error) {
	authParams := cloneStringMap(profile.AuthParams)
	cliAuthParams, err := parseOAuthAuthParams(opts.authParams)
	if err != nil {
		return "", err
	}
	for k, v := range cliAuthParams {
		authParams[k] = v
	}

	authURL := strings.TrimSpace(firstNonEmpty(opts.authURL, profile.AuthURL))
	tokenURL := strings.TrimSpace(firstNonEmpty(opts.tokenURL, profile.TokenURL))
	clientID := strings.TrimSpace(firstNonEmpty(opts.clientID, profile.ClientID))
	clientSecret := strings.TrimSpace(firstNonEmpty(opts.clientSecret, profile.ClientSecret))
	if clientSecret == "" {
		clientSecret = strings.TrimSpace(os.Getenv(oauthProviderClientSecretEnv(profile.Name)))
	}
	scope := strings.TrimSpace(firstNonEmpty(opts.scope, profile.Scope))
	redirectParam := strings.TrimSpace(firstNonEmpty(profile.RedirectParam, "redirect_uri"))
	usePKCE := profile.UsePKCE && !opts.noPKCE
	tokenType := strings.ToLower(strings.TrimSpace(firstNonEmpty(opts.tokenType, profile.TokenType)))
	if tokenType == "" {
		tokenType = oauthTokenContentTypeForm
	}
	if tokenType != oauthTokenContentTypeForm && tokenType != oauthTokenContentTypeJSON {
		return "", fmt.Errorf("invalid token-content-type %q, expected form|json", tokenType)
	}
	tokenBasic := profile.TokenBasic || opts.tokenBasic
	if tokenBasic && strings.TrimSpace(clientSecret) == "" {
		return "", fmt.Errorf(
			"provider %q requires client secret (token basic auth enabled): use --client-secret or set %s",
			profile.Name,
			oauthProviderClientSecretEnv(profile.Name),
		)
	}

	if authURL == "" {
		return "", errors.New("auth-url is empty")
	}
	if tokenURL == "" {
		return "", errors.New("token-url is empty")
	}
	if clientID == "" {
		return "", errors.New("client-id is empty")
	}

	var pkce pkceCodes
	if usePKCE {
		pkce, err = generatePKCE()
		if err != nil {
			return "", err
		}
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

	authLoginURL, err := buildOAuthAuthURL(oauthAuthURLRequest{
		BaseURL:       authURL,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		RedirectParam: redirectParam,
		Scope:         scope,
		State:         state,
		UsePKCE:       usePKCE,
		Challenge:     pkce.Challenge,
		Params:        authParams,
	})
	if err != nil {
		return "", err
	}
	fmt.Printf("Complete OAuth login for provider %q in your browser:\n%s\n", profile.Name, authLoginURL)
	if !opts.noBrowser {
		_ = openBrowser(authLoginURL)
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

	return exchangeOAuthRefreshToken(waitCtx, oauthTokenExchangeRequest{
		TokenURL:     tokenURL,
		ContentType:  tokenType,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Code:         cb.Code,
		Verifier:     pkce.Verifier,
		UsePKCE:      usePKCE,
		TokenBasic:   tokenBasic,
	})
}

func validateAndResolveOAuthProfile(opts oauthRefreshTokenOptions) (oauthProviderProfile, error) {
	if opts.callbackPort <= 0 {
		return oauthProviderProfile{}, errors.New("callback-port must be > 0")
	}
	if opts.timeout <= 0 {
		return oauthProviderProfile{}, errors.New("timeout must be > 0")
	}
	if strings.TrimSpace(opts.provider) == "" {
		return oauthProviderProfile{}, errors.New("missing provider: use --provider/-p")
	}
	return resolveOAuthProviderProfile(opts.provider)
}

type oauthDeviceCodeResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type oauthDeviceTokenResp struct {
	AccessToken      string  `json:"access_token"`
	RefreshToken     string  `json:"refresh_token"`
	TokenType        string  `json:"token_type"`
	ExpiresIn        float64 `json:"expires_in"`
	Error            string  `json:"error"`
	ErrorDescription string  `json:"error_description"`
}

func runOAuthDeviceCodeRefreshTokenFlow(ctx context.Context, opts oauthRefreshTokenOptions, profile oauthProviderProfile) (string, error) {
	authParams := cloneStringMap(profile.AuthParams)
	cliAuthParams, err := parseOAuthAuthParams(opts.authParams)
	if err != nil {
		return "", err
	}
	for k, v := range cliAuthParams {
		authParams[k] = v
	}

	deviceCodeURL := strings.TrimSpace(firstNonEmpty(opts.authURL, profile.AuthURL))
	tokenURL := strings.TrimSpace(firstNonEmpty(opts.tokenURL, profile.TokenURL))
	clientID := strings.TrimSpace(firstNonEmpty(opts.clientID, profile.ClientID))
	scope := strings.TrimSpace(firstNonEmpty(opts.scope, profile.Scope))
	if deviceCodeURL == "" {
		return "", errors.New("auth-url is empty")
	}
	if tokenURL == "" {
		return "", errors.New("token-url is empty")
	}
	if clientID == "" {
		return "", errors.New("client-id is empty")
	}

	usePKCE := profile.UsePKCE && !opts.noPKCE
	var pkce pkceCodes
	if usePKCE {
		pkce, err = generatePKCE()
		if err != nil {
			return "", err
		}
	}

	dc, err := requestOAuthDeviceCode(ctx, deviceCodeURL, clientID, scope, authParams, usePKCE, pkce.Challenge)
	if err != nil {
		return "", err
	}
	verifyURL := strings.TrimSpace(firstNonEmpty(dc.VerificationURIComplete, dc.VerificationURI))
	if verifyURL == "" {
		return "", errors.New("device code response missing verification URL")
	}

	fmt.Printf("Complete OAuth device login for provider %q:\n%s\n", profile.Name, verifyURL)
	if strings.TrimSpace(dc.UserCode) != "" {
		fmt.Printf("User code: %s\n", strings.TrimSpace(dc.UserCode))
	}
	if !opts.noBrowser {
		_ = openBrowser(verifyURL)
	}

	waitCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	return pollOAuthDeviceToken(waitCtx, oauthDevicePollRequest{
		Provider:     profile.Name,
		TokenURL:     tokenURL,
		ClientID:     clientID,
		DeviceCode:   strings.TrimSpace(dc.DeviceCode),
		Verifier:     strings.TrimSpace(pkce.Verifier),
		UsePKCE:      usePKCE,
		GrantType:    strings.TrimSpace(firstNonEmpty(profile.DeviceGrant, "urn:ietf:params:oauth:grant-type:device_code")),
		PollInterval: dc.Interval,
		ExpiresIn:    dc.ExpiresIn,
		TokenBasic:   profile.TokenBasic || opts.tokenBasic,
		ClientSecret: strings.TrimSpace(firstNonEmpty(opts.clientSecret, profile.ClientSecret)),
		TokenType:    oauthTokenContentTypeForm,
	})
}

func requestOAuthDeviceCode(
	ctx context.Context,
	deviceCodeURL string,
	clientID string,
	scope string,
	params map[string]string,
	usePKCE bool,
	challenge string,
) (oauthDeviceCodeResp, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(clientID))
	if strings.TrimSpace(scope) != "" {
		form.Set("scope", strings.TrimSpace(scope))
	}
	if usePKCE {
		form.Set("code_challenge", strings.TrimSpace(challenge))
		form.Set("code_challenge_method", "S256")
	}
	for k, v := range params {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		form.Set(key, strings.TrimSpace(v))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(deviceCodeURL), strings.NewReader(form.Encode()))
	if err != nil {
		return oauthDeviceCodeResp{}, fmt.Errorf("create device code request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthDeviceCodeResp{}, fmt.Errorf("device code request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return oauthDeviceCodeResp{}, fmt.Errorf("read device code response failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthDeviceCodeResp{}, fmt.Errorf("device code endpoint failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var dc oauthDeviceCodeResp
	if err := json.Unmarshal(body, &dc); err != nil {
		return oauthDeviceCodeResp{}, fmt.Errorf("parse device code response failed: %w", err)
	}
	if strings.TrimSpace(dc.DeviceCode) == "" {
		return oauthDeviceCodeResp{}, errors.New("device code response missing device_code")
	}
	return dc, nil
}

type oauthDevicePollRequest struct {
	Provider     string
	TokenURL     string
	ClientID     string
	ClientSecret string
	DeviceCode   string
	Verifier     string
	UsePKCE      bool
	GrantType    string
	PollInterval int
	ExpiresIn    int
	TokenBasic   bool
	TokenType    string
}

func pollOAuthDeviceToken(ctx context.Context, in oauthDevicePollRequest) (string, error) {
	interval := 5 * time.Second
	if in.PollInterval > 0 {
		interval = time.Duration(in.PollInterval) * time.Second
	}
	if interval < time.Second {
		interval = time.Second
	}
	deadline := time.Now().Add(15 * time.Minute)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if in.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(in.ExpiresIn) * time.Second)
		if exp.Before(deadline) {
			deadline = exp
		}
	}

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("device flow timeout for provider %q", in.Provider)
		}
		token, shouldContinue, err := exchangeOAuthDeviceTokenOnce(ctx, in)
		if err != nil {
			return "", err
		}
		if !shouldContinue {
			if strings.TrimSpace(token) == "" {
				return "", errors.New("refresh_token not found in token response")
			}
			return strings.TrimSpace(token), nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("device flow canceled: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

func exchangeOAuthDeviceTokenOnce(ctx context.Context, in oauthDevicePollRequest) (token string, shouldContinue bool, err error) {
	form := url.Values{}
	form.Set("grant_type", strings.TrimSpace(in.GrantType))
	form.Set("client_id", strings.TrimSpace(in.ClientID))
	form.Set("device_code", strings.TrimSpace(in.DeviceCode))
	if in.UsePKCE {
		form.Set("code_verifier", strings.TrimSpace(in.Verifier))
	}
	if strings.TrimSpace(in.ClientSecret) != "" {
		form.Set("client_secret", strings.TrimSpace(in.ClientSecret))
	}

	req, errReq := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(in.TokenURL), strings.NewReader(form.Encode()))
	if errReq != nil {
		return "", false, fmt.Errorf("create device token request failed: %w", errReq)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if in.TokenBasic {
		clientID := strings.TrimSpace(in.ClientID)
		clientSecret := strings.TrimSpace(in.ClientSecret)
		if clientID == "" || clientSecret == "" {
			return "", false, errors.New("token-basic-auth requires both client-id and client-secret")
		}
		raw := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
		req.Header.Set("Authorization", "Basic "+raw)
	}

	resp, errDo := http.DefaultClient.Do(req)
	if errDo != nil {
		return "", false, fmt.Errorf("device token request failed: %w", errDo)
	}
	defer func() { _ = resp.Body.Close() }()

	body, errRead := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if errRead != nil {
		return "", false, fmt.Errorf("read device token response failed: %w", errRead)
	}

	var tr oauthDeviceTokenResp
	_ = json.Unmarshal(body, &tr)
	errType := strings.TrimSpace(tr.Error)
	if errType == "" {
		var obj map[string]any
		if errUnmarshal := json.Unmarshal(body, &obj); errUnmarshal == nil {
			if v, ok := obj["error"].(string); ok {
				errType = strings.TrimSpace(v)
			}
			if strings.TrimSpace(tr.ErrorDescription) == "" {
				if v, ok := obj["error_description"].(string); ok {
					tr.ErrorDescription = strings.TrimSpace(v)
				}
			}
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && errType == "" {
		return strings.TrimSpace(tr.RefreshToken), false, nil
	}

	switch strings.ToLower(errType) {
	case "authorization_pending", "slow_down":
		return "", true, nil
	case "expired_token":
		return "", false, fmt.Errorf("device code expired for provider %q", in.Provider)
	case "access_denied":
		return "", false, fmt.Errorf("authorization denied for provider %q", in.Provider)
	}

	desc := strings.TrimSpace(tr.ErrorDescription)
	if errType != "" {
		if desc != "" {
			return "", false, fmt.Errorf("device token poll failed: %s - %s", errType, desc)
		}
		return "", false, fmt.Errorf("device token poll failed: %s", errType)
	}
	return "", false, fmt.Errorf("device token endpoint failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func startOAuthCallbackServer(port int, cbCh chan<- oauthCallback) (*http.Server, error) {
	addr := fmt.Sprintf(":%d", port)
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
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

type oauthAuthURLRequest struct {
	BaseURL       string
	ClientID      string
	RedirectURI   string
	RedirectParam string
	Scope         string
	State         string
	UsePKCE       bool
	Challenge     string
	Params        map[string]string
}

type oauthTokenExchangeRequest struct {
	TokenURL     string
	ContentType  string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Code         string
	Verifier     string
	UsePKCE      bool
	TokenBasic   bool
}

func exchangeOAuthRefreshToken(ctx context.Context, in oauthTokenExchangeRequest) (string, error) {
	tokenURL := strings.TrimSpace(in.TokenURL)
	if tokenURL == "" {
		return "", errors.New("token-url is empty")
	}
	contentType := strings.ToLower(strings.TrimSpace(in.ContentType))
	if contentType == "" {
		contentType = oauthTokenContentTypeForm
	}
	if contentType != oauthTokenContentTypeForm && contentType != oauthTokenContentTypeJSON {
		return "", fmt.Errorf("unsupported token content type %q", in.ContentType)
	}

	var (
		req *http.Request
		err error
	)
	if contentType == oauthTokenContentTypeJSON {
		body := map[string]string{
			"grant_type":   "authorization_code",
			"client_id":    strings.TrimSpace(in.ClientID),
			"code":         strings.TrimSpace(in.Code),
			"redirect_uri": strings.TrimSpace(in.RedirectURI),
		}
		if in.UsePKCE {
			body["code_verifier"] = strings.TrimSpace(in.Verifier)
		}
		if strings.TrimSpace(in.ClientSecret) != "" {
			body["client_secret"] = strings.TrimSpace(in.ClientSecret)
		}
		raw, errMarshal := json.Marshal(body)
		if errMarshal != nil {
			return "", fmt.Errorf("marshal token request failed: %w", errMarshal)
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(string(raw)))
		if err != nil {
			return "", fmt.Errorf("create token request failed: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("client_id", strings.TrimSpace(in.ClientID))
		form.Set("code", strings.TrimSpace(in.Code))
		form.Set("redirect_uri", strings.TrimSpace(in.RedirectURI))
		if in.UsePKCE {
			form.Set("code_verifier", strings.TrimSpace(in.Verifier))
		}
		if strings.TrimSpace(in.ClientSecret) != "" {
			form.Set("client_secret", strings.TrimSpace(in.ClientSecret))
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", fmt.Errorf("create token request failed: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Accept", "application/json")
	if in.TokenBasic {
		clientID := strings.TrimSpace(in.ClientID)
		clientSecret := strings.TrimSpace(in.ClientSecret)
		if clientID == "" || clientSecret == "" {
			return "", errors.New("token-basic-auth requires both client-id and client-secret")
		}
		raw := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
		req.Header.Set("Authorization", "Basic "+raw)
	}

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

	var tr oauthTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("parse token response failed: %w", err)
	}
	token := strings.TrimSpace(tr.RefreshToken)
	if token == "" {
		return "", errors.New("refresh_token not found in token response")
	}
	return token, nil
}

func buildOAuthAuthURL(in oauthAuthURLRequest) (string, error) {
	b := strings.TrimSpace(in.BaseURL)
	if b == "" {
		return "", errors.New("auth-url is empty")
	}
	u, err := url.Parse(b)
	if err != nil {
		return "", fmt.Errorf("invalid auth-url: %w", err)
	}
	q := u.Query()
	q.Set("client_id", strings.TrimSpace(in.ClientID))
	q.Set("response_type", "code")
	redirectParam := strings.TrimSpace(in.RedirectParam)
	if redirectParam == "" {
		redirectParam = "redirect_uri"
	}
	q.Set(redirectParam, strings.TrimSpace(in.RedirectURI))
	if strings.TrimSpace(in.Scope) != "" {
		q.Set("scope", strings.TrimSpace(in.Scope))
	}
	q.Set("state", strings.TrimSpace(in.State))
	if in.UsePKCE {
		challenge := strings.TrimSpace(in.Challenge)
		if challenge == "" {
			return "", errors.New("pkce enabled but code_challenge is empty")
		}
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
	}
	for k, v := range in.Params {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		q.Set(key, strings.TrimSpace(v))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func parseOAuthAuthParams(values []string) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		key, val, ok := strings.Cut(v, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --auth-param %q: expected key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid --auth-param %q: empty key", raw)
		}
		out[key] = strings.TrimSpace(val)
	}
	return out, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func oauthProviderClientSecretEnv(provider string) string {
	p := strings.ToUpper(strings.TrimSpace(provider))
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return "ONR_OAUTH_" + b.String() + "_CLIENT_SECRET"
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
		cmd = exec.CommandContext(context.Background(), "open", target)
	case "windows":
		cmd = exec.CommandContext(context.Background(), "cmd", "/c", "start", target)
	default:
		cmd = exec.CommandContext(context.Background(), "xdg-open", target)
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

func printRefreshTokenNextSteps(provider, token string) {
	masked := maskToken(token)
	p := strings.TrimSpace(provider)
	if p == "" {
		p = providerOpenAI
	}
	fmt.Fprintf(os.Stderr,
		"\nNext steps:\n1. Save this token to keys.yaml (or your secret manager) as an upstream key value.\n2. Use that key in provider %q OAuth channel (e.g. oauth_refresh_token $channel.key;).\n3. Restart ONR and test the upstream API.\nToken (masked): %s\n",
		p, masked,
	)
}

func maskToken(token string) string {
	t := strings.TrimSpace(token)
	if len(t) <= 10 {
		return t
	}
	return t[:6] + "..." + t[len(t)-4:]
}
