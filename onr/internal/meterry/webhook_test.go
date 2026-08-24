package meterry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "webhook-secret"
	timestamp := time.Now().Unix()
	body := []byte(`{"id":"evt_1","type":"wallet.insufficient_balance","data":{"subject":{"subject_type":"api_key","subject_id":"client-a"}}}`)
	mac := hmac.New(sha256.New, []byte(secret))
	timestampValue := strconv.FormatInt(timestamp, 10)
	_, _ = mac.Write([]byte(timestampValue))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	signature := "v1=" + hex.EncodeToString(mac.Sum(nil))
	if err := VerifyWebhookSignature(secret, timestampValue, signature, body, time.Unix(timestamp, 0), time.Minute); err != nil {
		t.Fatalf("VerifyWebhookSignature error: %v", err)
	}
	if err := VerifyWebhookSignature(secret, strconv.FormatInt(timestamp-120, 10), signature, body, time.Unix(timestamp, 0), time.Minute); err == nil {
		t.Fatal("expected expired timestamp error")
	}
}

func TestParseWebhookEventAndApplyState(t *testing.T) {
	event, err := ParseWebhookEvent([]byte(`{"id":"evt_1","type":"wallet.insufficient_balance","data":{"subject":{"subject_type":"api_key","subject_id":"client-a"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != "evt_1" || event.Type != "wallet.insufficient_balance" {
		t.Fatalf("unexpected event: %+v", event)
	}
	typ, id := event.Subject()
	if typ != "api_key" || id != "client-a" {
		t.Fatalf("subject=%s/%s", typ, id)
	}
	state, err := openSubjectState(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.applyWebhook(event.ID, event.Type, typ, id); err != nil {
		t.Fatal(err)
	}
	if !state.isBlocked(typ, id) {
		t.Fatal("subject should be blocked")
	}
	if err := state.applyWebhook(event.ID, event.Type, typ, id); err != nil {
		t.Fatal(err)
	}
	if err := state.applyWebhook("evt_2", "wallet.balance_changed", typ, id); err != nil {
		t.Fatal(err)
	}
	if state.isBlocked(typ, id) {
		t.Fatal("subject should be unblocked after balance change")
	}
}
