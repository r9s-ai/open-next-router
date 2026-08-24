package controlplane

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	mr := miniredis.RunT(t)
	c, err := New(Config{
		Addr:                 "redis://" + mr.Addr(),
		KeyPrefix:            "test",
		OperationTimeout:     time.Second,
		AccessKeyHashSecret:  "test-hash-secret",
		BillingStream:        "billing",
		BillingConsumerGroup: "group",
		BillingConsumerName:  "consumer-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestAccessKeyRoundTripAndRevoke(t *testing.T) {
	c := newTestClient(t)
	secret, err := NewAccessKeySecret()
	if err != nil {
		t.Fatal(err)
	}
	record := AccessKeyRecord{
		Name:        "client-a",
		SecretHash:  c.HashAccessKey(secret),
		SubjectType: "api_key",
		SubjectID:   "client-a",
	}
	if err := c.CreateAccessKey(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateAccessKey(context.Background(), record); err == nil {
		t.Fatal("expected duplicate access key error")
	}
	got, err := c.LookupAccessKey(context.Background(), secret)
	if err != nil || got == nil || got.Name != "client-a" {
		t.Fatalf("LookupAccessKey=(%+v,%v)", got, err)
	}
	if err := c.RevokeAccessKey(context.Background(), "client-a"); err != nil {
		t.Fatal(err)
	}
	got, err = c.LookupAccessKey(context.Background(), secret)
	if err != nil || got != nil {
		t.Fatalf("revoked LookupAccessKey=(%+v,%v), want nil,nil", got, err)
	}
	if raw := c.HashAccessKey(secret); raw == secret {
		t.Fatal("access key hash must not equal plaintext secret")
	}
}

func TestListAccessKeyRecordsIncludesRevokedAndExpired(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	secretA, _ := NewAccessKeySecret()
	secretB, _ := NewAccessKeySecret()
	expired := time.Now().Add(-time.Minute)
	if err := c.CreateAccessKey(ctx, AccessKeyRecord{Name: "active", SecretHash: c.HashAccessKey(secretA), Status: "active", SubjectType: "api_key", SubjectID: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateAccessKey(ctx, AccessKeyRecord{Name: "expired", SecretHash: c.HashAccessKey(secretB), Status: "active", SubjectType: "api_key", SubjectID: "expired", ExpiresAt: &expired}); err != nil {
		t.Fatal(err)
	}
	if err := c.RevokeAccessKey(ctx, "active"); err != nil {
		t.Fatal(err)
	}
	records, err := c.ListAccessKeyRecords(ctx)
	if err != nil || len(records) != 2 {
		t.Fatalf("records=(%+v,%v), want two administrative records", records, err)
	}
	if record, err := c.GetAccessKey(ctx, "active"); err != nil || record != nil {
		t.Fatalf("active lookup after revoke=(%+v,%v), want nil,nil", record, err)
	}
}

func TestSubjectAndBalanceState(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	if err := c.SetSubjectState(ctx, "api_key", "client-a", SubjectState{Blocked: true, BlockedReason: "insufficient_balance"}); err != nil {
		t.Fatal(err)
	}
	state, err := c.GetSubjectState(ctx, "api_key", "client-a")
	if err != nil || !state.Blocked || state.BlockedReason != "insufficient_balance" {
		t.Fatalf("subject state=(%+v,%v)", state, err)
	}
	value := BalanceCacheValue{Allowed: true, Balance: "1", ExpiresAt: time.Now().Add(time.Minute)}
	if err := c.SetBalanceCache(ctx, "api_key", "client-a", "USD", value, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.GetBalanceCache(ctx, "api_key", "client-a", "USD")
	if err != nil || !ok || !got.Allowed || got.Balance != "1" {
		t.Fatalf("balance cache=(%+v,%v,%v)", got, ok, err)
	}
	if err := c.InvalidateBalanceCache(ctx, "api_key", "client-a", "USD"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := c.GetBalanceCache(ctx, "api_key", "client-a", "USD"); err != nil || ok {
		t.Fatalf("invalidated balance cache=(%v,%v)", ok, err)
	}
}

func TestBillingStreamRoundTrip(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	if err := c.EnsureBillingGroup(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EnqueueBillingEvent(ctx, []byte(`{"idempotency_key":"onr:req-1"}`)); err != nil {
		t.Fatal(err)
	}
	messages, err := c.ReadBilling(ctx, 1, time.Second)
	if err != nil || len(messages) != 1 || messages[0].Values["payload"] == "" {
		t.Fatalf("billing messages=(%+v,%v)", messages, err)
	}
	if err := c.AckBilling(ctx, messages[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestBalanceRefreshLockIsOwned(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	got, err := c.AcquireBalanceRefreshLock(ctx, "api_key", "client-a", "USD", "token-a", time.Minute)
	if err != nil || !got {
		t.Fatalf("first lock=(%v,%v)", got, err)
	}
	got, err = c.AcquireBalanceRefreshLock(ctx, "api_key", "client-a", "USD", "token-b", time.Minute)
	if err != nil || got {
		t.Fatalf("second lock=(%v,%v), want false,nil", got, err)
	}
	if err := c.ReleaseBalanceRefreshLock(ctx, "api_key", "client-a", "USD", "token-b"); err != nil {
		t.Fatal(err)
	}
	got, err = c.AcquireBalanceRefreshLock(ctx, "api_key", "client-a", "USD", "token-b", time.Minute)
	if err != nil || got {
		t.Fatalf("lock after wrong release=(%v,%v), want false,nil", got, err)
	}
	if err := c.ReleaseBalanceRefreshLock(ctx, "api_key", "client-a", "USD", "token-a"); err != nil {
		t.Fatal(err)
	}
	got, err = c.AcquireBalanceRefreshLock(ctx, "api_key", "client-a", "USD", "token-b", time.Minute)
	if err != nil || !got {
		t.Fatalf("lock after owner release=(%v,%v), want true,nil", got, err)
	}
}
