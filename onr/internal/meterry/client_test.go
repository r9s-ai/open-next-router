package meterry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientSendsQueuedEvent(t *testing.T) {
	received := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e Event
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			t.Errorf("decode event: %v", err)
		}
		received <- e
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	c, err := New(Config{
		Enabled:          true,
		BaseURL:          srv.URL,
		ProjectID:        "proj",
		APIKey:           "secret",
		ExtractorRuleSet: "ers",
		OutboxDir:        t.TempDir(),
		RetryInterval:    5 * time.Millisecond,
		RequestTimeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	e := NewEvent("rid_1", "openai", "chat.completions", "m", false, 200, "upstream", nil, "api_key", "k", "", nil)
	if err := c.Enqueue(e); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got.IdempotencyKey != e.IdempotencyKey {
			t.Fatalf("idempotency key = %q", got.IdempotencyKey)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Meterry ingest")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientCheckBalance(t *testing.T) {
	requests := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"account_id":"acct_1","currency":"USD","balance":"0.01"}`))
	}))
	defer srv.Close()
	c, err := New(Config{
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
	defer func() { _ = c.Close() }()
	allowed, err := c.CheckBalance(t.Context(), "api_key", "client-a")
	if err != nil || !allowed {
		t.Fatalf("CheckBalance=(%v,%v), want true,nil", allowed, err)
	}
	query := <-requests
	if !strings.Contains(query, "subject_type=api_key") || !strings.Contains(query, "subject_id=client-a") || !strings.Contains(query, "currency=USD") {
		t.Fatalf("unexpected query: %s", query)
	}
}

func TestClientCheckBalanceZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance":"0"}`))
	}))
	defer srv.Close()
	c, err := New(Config{Enabled: true, BaseURL: srv.URL, ProjectID: "proj", APIKey: "secret", ExtractorRuleSet: "ers", OutboxDir: t.TempDir(), BalanceEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	allowed, err := c.CheckBalance(t.Context(), "api_key", "client-a")
	if err != nil || allowed {
		t.Fatalf("CheckBalance=(%v,%v), want false,nil", allowed, err)
	}
}
