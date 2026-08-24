package onrserver

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr/internal/meterry"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func meterryWebhookHandler(cfg *config.Config, billing *meterry.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || billing == nil || !billing.Enabled() || !cfg.Meterry.BalanceEnforcement.Enabled {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if err := meterry.VerifyWebhookSignature(
			cfg.Meterry.BalanceEnforcement.WebhookSecret,
			c.GetHeader("X-Billing-Webhook-Timestamp"),
			c.GetHeader("X-Billing-Webhook-Signature"),
			body,
			time.Now(),
			time.Duration(cfg.Meterry.BalanceEnforcement.TimestampToleranceS)*time.Second,
		); err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		event, err := meterry.ParseWebhookEvent(body)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if event.ID == "" {
			event.ID = c.GetHeader("X-Billing-Webhook-ID")
		}
		if event.ID == "" {
			event.ID = c.GetHeader("X-Billing-Webhook-Delivery-ID")
		}
		if strings.TrimSpace(event.ID) == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		subjectType, subjectID := event.Subject()
		if err := billing.ApplyWebhook(event.ID, strings.ToLower(strings.TrimSpace(event.Type)), subjectType, subjectID); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.AbortWithStatus(http.StatusNoContent)
	}
}
