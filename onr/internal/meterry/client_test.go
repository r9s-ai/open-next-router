package meterry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
