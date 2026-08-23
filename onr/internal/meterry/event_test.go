package meterry

import (
	"encoding/json"
	"testing"
)

func TestNewEventContainsOnlyBillingMetadata(t *testing.T) {
	e := NewEvent("rid_1", "openai", "chat.completions", "gpt-4.1-mini", true, 200, "upstream", map[string]any{"input_tokens": 12}, "api_key", "key_1", "client", map[string]any{"input_tokens": map[string]any{"unit_price": "1.0"}})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" || string(b) == "{}" {
		t.Fatalf("unexpected event JSON: %s", b)
	}
	if e.RawJSON["request_body"] != nil {
		t.Fatalf("event must not contain request body")
	}
	if e.IdempotencyKey != "onr:rid_1" {
		t.Fatalf("idempotency key = %q", e.IdempotencyKey)
	}
	if e.SubjectType != "api_key" || e.SubjectID != "key_1" {
		t.Fatalf("subject = %s/%s", e.SubjectType, e.SubjectID)
	}
	if got := e.RawJSON["usage"].(map[string]any)["prompt_tokens"]; got != 12 {
		t.Fatalf("prompt_tokens alias = %#v", got)
	}
}
