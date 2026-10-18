package dslruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/oauthclient"
)

type AuthConfig struct {
	Type     string
	Header   string
	Mode     string
	Scope    string
	TokenURL string
}

type AuthHeaderInput struct {
	Auth AuthConfig
	Key  string

	HTTPClient *http.Client

	CacheKeyPrefix string

	IncludeGoogleUserProjectHeader bool
}

type RoutePathContext struct {
	Model               string
	ModelMapped         string
	CredentialProjectID string
	ChannelLocation     string
	BaseURL             string
	APIKey              string
	OAuthAccessToken    string
	TaskID              string
	TaskUpstreamID      string
}

func BuildAuthHeaders(ctx context.Context, in AuthHeaderInput) (http.Header, error) {
	hdr := make(http.Header)
	authType := strings.ToLower(strings.TrimSpace(in.Auth.Type))
	header := strings.TrimSpace(in.Auth.Header)
	switch authType {
	case "":
		return hdr, nil
	case "bearer":
		if header == "" {
			header = "Authorization"
		}
		hdr.Set(header, "Bearer "+strings.TrimSpace(in.Key))
	case "header_key":
		if header == "" {
			return nil, errors.New("dsl auth header is empty")
		}
		hdr.Set(header, strings.TrimSpace(in.Key))
	case "oauth_bearer":
		if header == "" {
			header = "Authorization"
		}
		token, err := acquireOAuthBearerToken(ctx, in)
		if err != nil {
			return nil, err
		}
		hdr.Set(header, "Bearer "+token)
		if in.IncludeGoogleUserProjectHeader && isGoogleServiceAccountMode(in.Auth.Mode) {
			if info, err := oauthclient.ParseServiceAccountCredentialInfo(in.Key); err == nil && strings.TrimSpace(info.ProjectID) != "" {
				hdr.Set("x-goog-user-project", strings.TrimSpace(info.ProjectID))
			}
		}
	default:
		return nil, fmt.Errorf("unsupported dsl auth type %q", authType)
	}
	return hdr, nil
}

func RenderRoutePath(defaultPath, overridePath string, ctx RoutePathContext) string {
	path := strings.TrimSpace(overridePath)
	if path == "" {
		return defaultPath
	}
	meta := routePathMeta(ctx)
	if looksLikeStringExpr(path) {
		path = dslconfig.EvalStringExpr(path, meta)
	}
	path = replaceRoutePlaceholders(path, ctx)
	if !strings.Contains(path, "?") && strings.Contains(defaultPath, "?") {
		if idx := strings.Index(defaultPath, "?"); idx >= 0 {
			path += defaultPath[idx:]
		}
	}
	return path
}

func GoogleServiceAccountProjectID(jsonContent string) (string, error) {
	info, err := oauthclient.ParseServiceAccountCredentialInfo(jsonContent)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(info.ProjectID), nil
}

func acquireOAuthBearerToken(ctx context.Context, in AuthHeaderInput) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(in.Auth.Mode))
	switch mode {
	case "google_service_account_file":
		scope := strings.TrimSpace(in.Auth.Scope)
		if scope == "" {
			return "", errors.New("dsl oauth_scope is required for google_service_account_file mode")
		}
		cachePrefix := strings.TrimSpace(in.CacheKeyPrefix)
		if cachePrefix == "" {
			cachePrefix = "dsl-runtime"
		}
		hash := sha256.Sum256([]byte(strings.TrimSpace(in.Key) + "|" + scope + "|" + strings.TrimSpace(in.Auth.TokenURL)))
		client := oauthclient.New(in.HTTPClient, false, "")
		tok, err := client.GetToken(ctx, oauthclient.AcquireInput{
			CacheKey:                     cachePrefix + "|" + hex.EncodeToString(hash[:]),
			Mode:                         mode,
			TokenURL:                     strings.TrimSpace(in.Auth.TokenURL),
			Method:                       http.MethodPost,
			ContentType:                  "form",
			ServiceAccountCredentialJSON: strings.TrimSpace(in.Key),
			ServiceAccountScope:          scope,
			Timeout:                      10 * time.Second,
			RefreshSkew:                  5 * time.Minute,
			FallbackTTL:                  time.Hour,
		})
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(tok.AccessToken), nil
	default:
		return "", fmt.Errorf("unsupported dsl oauth mode %q", mode)
	}
}

func isGoogleServiceAccountMode(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "google_service_account_file")
}

func looksLikeStringExpr(path string) bool {
	raw := strings.TrimSpace(path)
	return strings.HasPrefix(raw, "template(") ||
		strings.HasPrefix(raw, "concat(") ||
		strings.HasPrefix(raw, "$") ||
		(len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')))
}

func routePathMeta(ctx RoutePathContext) *dslmeta.Meta {
	mapped := strings.TrimSpace(ctx.ModelMapped)
	if mapped == "" {
		mapped = strings.TrimSpace(ctx.Model)
	}
	return &dslmeta.Meta{
		BaseURL:             strings.TrimSpace(ctx.BaseURL),
		APIKey:              strings.TrimSpace(ctx.APIKey),
		ChannelLocation:     strings.TrimSpace(ctx.ChannelLocation),
		CredentialProjectID: strings.TrimSpace(ctx.CredentialProjectID),
		OAuthAccessToken:    strings.TrimSpace(ctx.OAuthAccessToken),
		OriginModelName:     strings.TrimSpace(ctx.Model),
		DSLModelMapped:      mapped,
		RequestURLPath:      "",
	}
}

func replaceRoutePlaceholders(path string, ctx RoutePathContext) string {
	model := strings.TrimSpace(ctx.ModelMapped)
	if model == "" {
		model = strings.TrimSpace(ctx.Model)
	}
	replacements := map[string]string{
		"{model}":                 model,
		"{request.model}":         strings.TrimSpace(ctx.Model),
		"{request.model_mapped}":  model,
		"{credential.project_id}": strings.TrimSpace(ctx.CredentialProjectID),
		"{channel.location}":      strings.TrimSpace(ctx.ChannelLocation),
		"{task.id}":               strings.TrimSpace(ctx.TaskID),
		"{task.upstream_id}":      strings.TrimSpace(ctx.TaskUpstreamID),
	}
	for k, v := range replacements {
		path = strings.ReplaceAll(path, k, v)
	}
	return path
}
