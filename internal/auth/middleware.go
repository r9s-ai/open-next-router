package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func Middleware(apiKey string) gin.HandlerFunc {
	expected := strings.TrimSpace(apiKey)
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

		if got != expected {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "unauthorized",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			})
			return
		}
		c.Next()
	}
}
