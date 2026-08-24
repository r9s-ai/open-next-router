package auth

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AccessKeyMatcher func(accessKey string) (name string, ok bool)
type AccessKeyResolver func(ctx context.Context, accessKey string) (name string, ok bool, err error)

type TokenKeyOptions struct {
	AllowBYOKWithoutK bool
}

func Middleware(masterKey string, matchAccessKey AccessKeyMatcher, tokenOpts ...TokenKeyOptions) gin.HandlerFunc {
	var resolver AccessKeyResolver
	if matchAccessKey != nil {
		resolver = func(_ context.Context, accessKey string) (string, bool, error) {
			name, ok := matchAccessKey(accessKey)
			return name, ok, nil
		}
	}
	return MiddlewareWithResolver(masterKey, resolver, tokenOpts...)
}

func MiddlewareWithResolver(masterKey string, resolveAccessKey AccessKeyResolver, tokenOpts ...TokenKeyOptions) gin.HandlerFunc {
	expected := strings.TrimSpace(masterKey)
	allowBYOKWithoutK := false
	if len(tokenOpts) > 0 {
		allowBYOKWithoutK = tokenOpts[0].AllowBYOKWithoutK
	}
	return func(c *gin.Context) {
		got := ""
		if v := strings.TrimSpace(c.GetHeader("Authorization")); strings.HasPrefix(v, "Bearer ") {
			got = strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
		}
		if got == "" {
			got = strings.TrimSpace(c.GetHeader("x-api-key"))
		}
		if got == "" {
			got = strings.TrimSpace(c.GetHeader("x-goog-api-key"))
		}

		// Legacy: exact match master key.
		if expected != "" && subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1 {
			c.Next()
			return
		}
		if resolveAccessKey != nil {
			if name, ok, err := resolveAccessKey(c.Request.Context(), got); err != nil {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
					"message": "authentication service is unavailable",
					"type":    "auth_error",
					"code":    "authentication_unavailable",
				}})
				return
			} else if ok {
				if strings.TrimSpace(name) != "" {
					c.Set("onr.auth_subject_id", strings.TrimSpace(name))
				}
				c.Next()
				return
			}
		}

		if ok, err := authenticateToken(c, got, expected, resolveAccessKey, allowBYOKWithoutK); err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
				"message": "authentication service is unavailable",
				"type":    "auth_error",
				"code":    "authentication_unavailable",
			}})
			return
		} else if ok {
			c.Next()
			return
		}

		{
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "unauthorized",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			})
			return
		}
	}
}

func authenticateToken(c *gin.Context, got, expected string, resolver AccessKeyResolver, allowBYOKWithoutK bool) (bool, error) {
	if !IsTokenKey(got) {
		return false, nil
	}
	claims, accessKey := parseToken(got, allowBYOKWithoutK)
	if claims == nil {
		return false, nil
	}
	var err error
	ok := false
	if strings.TrimSpace(accessKey) != "" {
		ok = expected != "" && subtle.ConstantTimeCompare([]byte(accessKey), []byte(expected)) == 1
		if !ok && resolver != nil {
			_, ok, err = resolver(c.Request.Context(), accessKey)
			if err != nil {
				return false, err
			}
		}
	} else if allowBYOKWithoutK && claims.Mode == TokenModeBYOK && strings.TrimSpace(claims.UpstreamKey) != "" {
		ok = true
	}
	if !ok {
		return false, nil
	}
	if strings.TrimSpace(accessKey) != "" && resolver != nil {
		name, matched, err := resolver(c.Request.Context(), accessKey)
		if err != nil {
			return false, err
		}
		if matched && strings.TrimSpace(name) != "" {
			c.Set("onr.auth_subject_id", strings.TrimSpace(name))
		}
	}
	if claims.Provider != "" {
		c.Set(ctxTokenProvider, claims.Provider)
	}
	if claims.ModelOverride != "" {
		c.Set(ctxTokenModelOverride, claims.ModelOverride)
	}
	if claims.UpstreamKey != "" {
		c.Set(ctxTokenUpstreamKey, claims.UpstreamKey)
	}
	c.Set(ctxTokenMode, string(claims.Mode))
	return true, nil
}

func parseToken(got string, allowBYOKWithoutK bool) (*TokenClaims, string) {
	claims, accessKey, err := ParseTokenKeyV1WithOptions(got, TokenParseOptions{AllowBYOKWithoutK: allowBYOKWithoutK})
	if err != nil {
		return nil, ""
	}
	return claims, accessKey
}

// TokenProvider requires a non-nil Gin context from the auth middleware path.
func TokenProvider(c *gin.Context) string {
	return strings.ToLower(strings.TrimSpace(c.GetString(ctxTokenProvider)))
}

// TokenModelOverride requires a non-nil Gin context from the auth middleware path.
func TokenModelOverride(c *gin.Context) string {
	return strings.TrimSpace(c.GetString(ctxTokenModelOverride))
}

// TokenUpstreamKey requires a non-nil Gin context from the auth middleware path.
func TokenUpstreamKey(c *gin.Context) string {
	return strings.TrimSpace(c.GetString(ctxTokenUpstreamKey))
}

// TokenModeFromContext requires a non-nil Gin context from the auth middleware path.
func TokenModeFromContext(c *gin.Context) TokenMode {
	v := strings.ToLower(strings.TrimSpace(c.GetString(ctxTokenMode)))
	switch v {
	case string(TokenModeBYOK):
		return TokenModeBYOK
	default:
		return TokenModeONR
	}
}
