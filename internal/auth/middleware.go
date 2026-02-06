package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AccessKeyMatcher func(accessKey string) (name string, ok bool)

func Middleware(masterKey string, matchAccessKey AccessKeyMatcher) gin.HandlerFunc {
	expected := strings.TrimSpace(masterKey)
	return func(c *gin.Context) {
		if expected == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "server misconfigured: missing api_key",
					"type":    "server_error",
					"code":    "server_misconfigured",
				},
			})
			return
		}

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
		if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1 {
			c.Next()
			return
		}

		// Token key: onr:v1?... (no-sig, editable)
		if IsTokenKey(got) {
			claims, accessKey, err := ParseTokenKeyV1(got)
			if err == nil && claims != nil && strings.TrimSpace(accessKey) != "" {
				ok := false
				if subtle.ConstantTimeCompare([]byte(accessKey), []byte(expected)) == 1 {
					ok = true
				}
				if !ok && matchAccessKey != nil {
					_, ok = matchAccessKey(accessKey)
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

func TokenProvider(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(c.GetString(ctxTokenProvider)))
}

func TokenModelOverride(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString(ctxTokenModelOverride))
}

func TokenUpstreamKey(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString(ctxTokenUpstreamKey))
}

func TokenModeFromContext(c *gin.Context) TokenMode {
	if c == nil {
		return TokenModeONR
	}
	v := strings.ToLower(strings.TrimSpace(c.GetString(ctxTokenMode)))
	switch v {
	case string(TokenModeBYOK):
		return TokenModeBYOK
	default:
		return TokenModeONR
	}
}
