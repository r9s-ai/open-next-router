package meterry

import (
	"io"
	"testing"
)

func TestOutboxAppendFirstAck(t *testing.T) {
	dir := t.TempDir()
	o, err := openOutbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	one := NewEvent("rid_1", "openai", "chat.completions", "m", false, 200, "upstream", nil, "api_key", "k", "", nil)
	two := NewEvent("rid_2", "openai", "chat.completions", "m", false, 200, "upstream", nil, "api_key", "k", "", nil)
	if err := o.append(one); err != nil {
		t.Fatal(err)
	}
	if err := o.append(two); err != nil {
		t.Fatal(err)
	}
	got, token, err := o.first()
	if err != nil || got.IdempotencyKey != one.IdempotencyKey {
		t.Fatalf("first = %#v token=%q, err=%v", got, token, err)
	}
	if err := o.ack(token); err != nil {
		t.Fatal(err)
	}
	got, token, err = o.first()
	if err != nil || got.IdempotencyKey != two.IdempotencyKey {
		t.Fatalf("after ack first = %#v, err=%v", got, err)
	}
	if err := o.ack(token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := o.first(); err != io.EOF {
		t.Fatalf("empty outbox err=%v, want EOF", err)
	}
}
