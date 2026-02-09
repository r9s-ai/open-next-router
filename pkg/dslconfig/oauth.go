package dslconfig

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
)

const (
	oauthModeOpenAI      = "openai"
	oauthModeGemini      = "gemini"
	oauthModeQwen        = "qwen"
	oauthModeClaude      = "claude"
	oauthModeIFLow       = "iflow"
	oauthModeAntigravity = "antigravity"
	oauthModeKimi        = "kimi"
	oauthModeCustom      = "custom"

	oauthContentTypeForm = "form"
	oauthContentTypeJSON = "json"
)

type OAuthFormField struct {
	Key       string
	ValueExpr string
}

type OAuthConfig struct {
	Mode string

	TokenURLExpr       string
	ClientIDExpr       string
	ClientSecretExpr   string
	RefreshTokenExpr   string
	ScopeExpr          string
	AudienceExpr       string
	Method             string
	ContentType        string
	TokenPath          string
	ExpiresInPath      string
	TokenTypePath      string
	TimeoutMs          *int
	RefreshSkewSec     *int
	FallbackTTLSeconds *int

	Form []OAuthFormField
}

type ResolvedOAuthConfig struct {
	Mode        string
	TokenURL    string
	Method      string
	ContentType string

	Form map[string]string
	// Optional for providers that require client auth in HTTP Basic.
	BasicAuthUsername string
	BasicAuthPassword string

	TokenPath      string
	ExpiresInPath  string
	TokenTypePath  string
	TimeoutMs      int
	RefreshSkewSec int
	FallbackTTLSec int
}

func (c OAuthConfig) IsEmpty() bool {
	return strings.TrimSpace(c.Mode) == "" &&
		strings.TrimSpace(c.TokenURLExpr) == "" &&
		strings.TrimSpace(c.ClientIDExpr) == "" &&
		strings.TrimSpace(c.ClientSecretExpr) == "" &&
		strings.TrimSpace(c.RefreshTokenExpr) == "" &&
		strings.TrimSpace(c.ScopeExpr) == "" &&
		strings.TrimSpace(c.AudienceExpr) == "" &&
		strings.TrimSpace(c.Method) == "" &&
		strings.TrimSpace(c.ContentType) == "" &&
		strings.TrimSpace(c.TokenPath) == "" &&
		strings.TrimSpace(c.ExpiresInPath) == "" &&
		strings.TrimSpace(c.TokenTypePath) == "" &&
		c.TimeoutMs == nil &&
		c.RefreshSkewSec == nil &&
		c.FallbackTTLSeconds == nil &&
		len(c.Form) == 0
}

func (c OAuthConfig) Merge(override OAuthConfig) OAuthConfig {
	out := c
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.TokenURLExpr) != "" {
		out.TokenURLExpr = override.TokenURLExpr
	}
	if strings.TrimSpace(override.ClientIDExpr) != "" {
		out.ClientIDExpr = override.ClientIDExpr
	}
	if strings.TrimSpace(override.ClientSecretExpr) != "" {
		out.ClientSecretExpr = override.ClientSecretExpr
	}
	if strings.TrimSpace(override.RefreshTokenExpr) != "" {
		out.RefreshTokenExpr = override.RefreshTokenExpr
	}
	if strings.TrimSpace(override.ScopeExpr) != "" {
		out.ScopeExpr = override.ScopeExpr
	}
	if strings.TrimSpace(override.AudienceExpr) != "" {
		out.AudienceExpr = override.AudienceExpr
	}
	if strings.TrimSpace(override.Method) != "" {
		out.Method = override.Method
	}
	if strings.TrimSpace(override.ContentType) != "" {
		out.ContentType = override.ContentType
	}
	if strings.TrimSpace(override.TokenPath) != "" {
		out.TokenPath = override.TokenPath
	}
	if strings.TrimSpace(override.ExpiresInPath) != "" {
		out.ExpiresInPath = override.ExpiresInPath
	}
	if strings.TrimSpace(override.TokenTypePath) != "" {
		out.TokenTypePath = override.TokenTypePath
	}
	if override.TimeoutMs != nil {
		v := *override.TimeoutMs
		out.TimeoutMs = &v
	}
	if override.RefreshSkewSec != nil {
		v := *override.RefreshSkewSec
		out.RefreshSkewSec = &v
	}
	if override.FallbackTTLSeconds != nil {
		v := *override.FallbackTTLSeconds
		out.FallbackTTLSeconds = &v
	}
	if len(override.Form) > 0 {
		out.Form = append(append([]OAuthFormField(nil), out.Form...), override.Form...)
	}
	return out
}

func (c OAuthConfig) Resolve(meta *dslmeta.Meta) (ResolvedOAuthConfig, bool) {
	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	if mode == "" {
		return ResolvedOAuthConfig{}, false
	}

	tpl, ok := oauthBuiltinTemplates[mode]
	if !ok {
		tpl = oauthBuiltinTemplates[oauthModeCustom]
	}

	res := ResolvedOAuthConfig{
		Mode:           mode,
		TokenURL:       strings.TrimSpace(evalStringExpr(firstNonEmpty(c.TokenURLExpr, quoteIfNeeded(tpl.TokenURL)), meta)),
		Method:         strings.ToUpper(strings.TrimSpace(firstNonEmpty(c.Method, tpl.Method))),
		ContentType:    strings.ToLower(strings.TrimSpace(firstNonEmpty(c.ContentType, tpl.ContentType))),
		TokenPath:      strings.TrimSpace(firstNonEmpty(c.TokenPath, tpl.TokenPath)),
		ExpiresInPath:  strings.TrimSpace(firstNonEmpty(c.ExpiresInPath, tpl.ExpiresInPath)),
		TokenTypePath:  strings.TrimSpace(firstNonEmpty(c.TokenTypePath, tpl.TokenTypePath)),
		TimeoutMs:      intOrDefault(c.TimeoutMs, tpl.TimeoutMs),
		RefreshSkewSec: intOrDefault(c.RefreshSkewSec, tpl.RefreshSkewSec),
		FallbackTTLSec: intOrDefault(c.FallbackTTLSeconds, tpl.FallbackTTLSec),
		Form:           map[string]string{},
	}

	clientIDExpr := firstNonEmpty(c.ClientIDExpr, quoteIfNeeded(tpl.ClientID))
	clientSecretExpr := firstNonEmpty(c.ClientSecretExpr, quoteIfNeeded(tpl.ClientSecret))
	refreshExpr := firstNonEmpty(c.RefreshTokenExpr, exprChannelKey)
	scopeExpr := firstNonEmpty(c.ScopeExpr, quoteIfNeeded(tpl.Scope))
	audienceExpr := firstNonEmpty(c.AudienceExpr, quoteIfNeeded(tpl.Audience))

	clientID := strings.TrimSpace(evalStringExpr(clientIDExpr, meta))
	clientSecret := strings.TrimSpace(evalStringExpr(clientSecretExpr, meta))
	refreshToken := strings.TrimSpace(evalStringExpr(refreshExpr, meta))
	scope := strings.TrimSpace(evalStringExpr(scopeExpr, meta))
	audience := strings.TrimSpace(evalStringExpr(audienceExpr, meta))

	switch mode {
	case oauthModeCustom:
		// custom mode: only explicit oauth_form fields are used.
	default:
		res.Form["grant_type"] = "refresh_token"
		if refreshToken != "" {
			res.Form["refresh_token"] = refreshToken
		}
		if clientID != "" {
			res.Form["client_id"] = clientID
		}
		if tpl.ClientSecretInForm && clientSecret != "" {
			res.Form["client_secret"] = clientSecret
		}
		if scope != "" {
			res.Form["scope"] = scope
		}
		if audience != "" {
			res.Form["audience"] = audience
		}
		if tpl.BasicAuthFromClientCreds {
			res.BasicAuthUsername = clientID
			res.BasicAuthPassword = clientSecret
		}
	}

	for _, f := range c.Form {
		k := strings.TrimSpace(f.Key)
		if k == "" {
			continue
		}
		res.Form[k] = strings.TrimSpace(evalStringExpr(f.ValueExpr, meta))
	}

	return res, true
}

func (r ResolvedOAuthConfig) CacheIdentity() string {
	keys := make([]string, 0, len(r.Form))
	for k := range r.Form {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(strings.ToLower(strings.TrimSpace(r.Mode)))
	b.WriteByte('|')
	b.WriteString(strings.ToUpper(strings.TrimSpace(r.Method)))
	b.WriteByte('|')
	b.WriteString(strings.ToLower(strings.TrimSpace(r.ContentType)))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(r.TokenURL))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(r.BasicAuthUsername))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(r.BasicAuthPassword))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(r.TokenPath))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(r.ExpiresInPath))
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(r.TokenTypePath))
	b.WriteByte('|')
	b.WriteString(strconv.Itoa(r.TimeoutMs))
	b.WriteByte('|')
	b.WriteString(strconv.Itoa(r.RefreshSkewSec))
	b.WriteByte('|')
	b.WriteString(strconv.Itoa(r.FallbackTTLSec))
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(r.Form[k])
	}
	return b.String()
}

type oauthTemplate struct {
	TokenURL string
	Method   string

	ContentType   string
	TokenPath     string
	ExpiresInPath string
	TokenTypePath string

	ClientID     string
	ClientSecret string
	Scope        string
	Audience     string

	ClientSecretInForm       bool
	BasicAuthFromClientCreds bool

	TimeoutMs      int
	RefreshSkewSec int
	FallbackTTLSec int
}

var oauthBuiltinTemplates = map[string]oauthTemplate{
	oauthModeOpenAI: {
		TokenURL:       "https://auth.openai.com/oauth/token",
		Method:         http.MethodPost,
		ContentType:    oauthContentTypeForm,
		TokenPath:      "$.access_token",
		ExpiresInPath:  "$.expires_in",
		TokenTypePath:  "$.token_type",
		ClientID:       "app_EMoamEEZ73f0CkXaXp7hrann",
		TimeoutMs:      5000,
		RefreshSkewSec: 300,
		FallbackTTLSec: 1800,
	},
	oauthModeGemini: {
		TokenURL:           "https://oauth2.googleapis.com/token",
		Method:             http.MethodPost,
		ContentType:        oauthContentTypeForm,
		TokenPath:          "$.access_token",
		ExpiresInPath:      "$.expires_in",
		TokenTypePath:      "$.token_type",
		ClientID:           "",
		ClientSecret:       "",
		ClientSecretInForm: true,
		TimeoutMs:          5000,
		RefreshSkewSec:     300,
		FallbackTTLSec:     1800,
	},
	oauthModeQwen: {
		TokenURL:       "https://chat.qwen.ai/api/v1/oauth2/token",
		Method:         http.MethodPost,
		ContentType:    oauthContentTypeForm,
		TokenPath:      "$.access_token",
		ExpiresInPath:  "$.expires_in",
		TokenTypePath:  "$.token_type",
		ClientID:       "f0304373b74a44d2b584a3fb70ca9e56",
		TimeoutMs:      5000,
		RefreshSkewSec: 300,
		FallbackTTLSec: 1800,
	},
	oauthModeClaude: {
		TokenURL:       "https://console.anthropic.com/v1/oauth/token",
		Method:         http.MethodPost,
		ContentType:    oauthContentTypeJSON,
		TokenPath:      "$.access_token",
		ExpiresInPath:  "$.expires_in",
		TokenTypePath:  "$.token_type",
		ClientID:       "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		TimeoutMs:      5000,
		RefreshSkewSec: 300,
		FallbackTTLSec: 1800,
	},
	oauthModeIFLow: {
		TokenURL:                 "https://iflow.cn/oauth/token",
		Method:                   http.MethodPost,
		ContentType:              oauthContentTypeForm,
		TokenPath:                "$.access_token",
		ExpiresInPath:            "$.expires_in",
		TokenTypePath:            "$.token_type",
		ClientID:                 "10009311001",
		ClientSecret:             "",
		ClientSecretInForm:       true,
		BasicAuthFromClientCreds: true,
		TimeoutMs:                5000,
		RefreshSkewSec:           300,
		FallbackTTLSec:           1800,
	},
	oauthModeAntigravity: {
		TokenURL:           "https://oauth2.googleapis.com/token",
		Method:             http.MethodPost,
		ContentType:        oauthContentTypeForm,
		TokenPath:          "$.access_token",
		ExpiresInPath:      "$.expires_in",
		TokenTypePath:      "$.token_type",
		ClientID:           "",
		ClientSecret:       "",
		ClientSecretInForm: true,
		TimeoutMs:          5000,
		RefreshSkewSec:     300,
		FallbackTTLSec:     1800,
	},
	oauthModeKimi: {
		TokenURL:       "https://auth.kimi.com/api/oauth/token",
		Method:         http.MethodPost,
		ContentType:    oauthContentTypeForm,
		TokenPath:      "$.access_token",
		ExpiresInPath:  "$.expires_in",
		TokenTypePath:  "$.token_type",
		ClientID:       "17e5f671-d194-4dfb-9706-5516cb48c098",
		TimeoutMs:      5000,
		RefreshSkewSec: 300,
		FallbackTTLSec: 1800,
	},
	oauthModeCustom: {
		Method:         http.MethodPost,
		ContentType:    oauthContentTypeForm,
		TokenPath:      "$.access_token",
		ExpiresInPath:  "$.expires_in",
		TokenTypePath:  "$.token_type",
		TimeoutMs:      5000,
		RefreshSkewSec: 300,
		FallbackTTLSec: 1800,
	},
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func intOrDefault(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func quoteIfNeeded(v string) string {
	t := strings.TrimSpace(v)
	if t == "" {
		return ""
	}
	if strings.HasPrefix(t, "\"") && strings.HasSuffix(t, "\"") {
		return t
	}
	return strconv.Quote(t)
}
