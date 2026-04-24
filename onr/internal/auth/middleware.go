package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AccessKeyMatcher func(accessKey string) (name string, ok bool)

type TokenKeyOptions struct {
	AllowBYOKWithoutK bool
}

func Middleware(masterKey string, matchAccessKey AccessKeyMatcher, tokenOpts ...TokenKeyOptions) gin.HandlerFunc {
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
		if matchAccessKey != nil {
			if _, ok := matchAccessKey(got); ok {
				c.Next()
				return
			}
		}

		// Token key: onr:v1?... (no-sig, editable)
		if IsTokenKey(got) {
			claims, accessKey, err := ParseTokenKeyV1WithOptions(got, TokenParseOptions{
				AllowBYOKWithoutK: allowBYOKWithoutK,
			})
			if err == nil && claims != nil {
				ok := false
				if strings.TrimSpace(accessKey) != "" {
					if expected != "" && subtle.ConstantTimeCompare([]byte(accessKey), []byte(expected)) == 1 {
						ok = true
					}
					if !ok && matchAccessKey != nil {
						_, ok = matchAccessKey(accessKey)
					}
				} else if allowBYOKWithoutK && claims.Mode == TokenModeBYOK && strings.TrimSpace(claims.UpstreamKey) != "" {
					ok = true
				}
				if ok {
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
					c.Next()
					return
				}
			}
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
