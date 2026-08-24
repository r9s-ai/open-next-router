package onrserver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr/internal/meterry"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func newBalanceClient(t *testing.T, response string, status int) *meterry.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	c, err := meterry.New(meterry.Config{
		Enabled:          true,
		BaseURL:          srv.URL,
		ProjectID:        "proj",
		APIKey:           "secret",
		ExtractorRuleSet: "ers",
		OutboxDir:        t.TempDir(),
		BalanceEnabled:   true,
		BalanceCurrency:  "USD",
		BalanceTimeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestEnforceBillingBalanceInsufficient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newBalanceClient(t, `{"balance":"0"}`, http.StatusOK)
	cfg := &config.Config{}
	cfg.Meterry.Enabled = true
	cfg.Meterry.SubjectType = "api_key"
	cfg.Meterry.BalanceEnforcement.Enabled = true
	cfg.Meterry.BalanceEnforcement.FailureMode = "closed"
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("onr.auth_subject_id", "client-a")
	ctx.Set("X-Onr-Request-Id", "rid-1")
	if enforceBillingBalance(cfg, c, ctx, "X-Onr-Request-Id") {
		t.Fatal("expected request to be rejected")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status=%d want=%d", rec.Code, http.StatusPaymentRequired)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"insufficient_balance"`)) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestEnforceBillingBalanceFailureMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		mode   string
		allow  bool
		status int
	}{
		{name: "closed", mode: "closed", allow: false, status: http.StatusServiceUnavailable},
		{name: "open", mode: "open", allow: true, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newBalanceClient(t, `unavailable`, http.StatusBadGateway)
			cfg := &config.Config{}
			cfg.Meterry.Enabled = true
			cfg.Meterry.SubjectType = "api_key"
			cfg.Meterry.BalanceEnforcement.Enabled = true
			cfg.Meterry.BalanceEnforcement.FailureMode = tc.mode
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			ctx.Set("onr.auth_subject_id", "client-a")
			if got := enforceBillingBalance(cfg, c, ctx, "X-Onr-Request-Id"); got != tc.allow {
				t.Fatalf("allow=%v want=%v", got, tc.allow)
			}
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d", rec.Code, tc.status)
			}
		})
	}
}

func TestMeterryWebhookHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newBalanceClient(t, `{"balance":"1"}`, http.StatusOK)
	secret := "webhook-secret"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"id":"evt_1","type":"wallet.insufficient_balance","data":{"subject":{"subject_type":"api_key","subject_id":"client-a"}}}`)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(body)
	cfg := &config.Config{}
	cfg.Meterry.Enabled = true
	cfg.Meterry.BalanceEnforcement.Enabled = true
	cfg.Meterry.BalanceEnforcement.WebhookSecret = secret
	cfg.Meterry.BalanceEnforcement.TimestampToleranceS = 300
	handler := meterryWebhookHandler(cfg, c)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/internal/meterry/webhook", bytes.NewReader(body))
	ctx.Request.Header.Set("X-Billing-Webhook-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Billing-Webhook-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
	handler(ctx)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", rec.Code, http.StatusNoContent)
	}
	allowed, err := c.CheckBalance(t.Context(), "api_key", "client-a")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("subject should be blocked by webhook")
	}
}
