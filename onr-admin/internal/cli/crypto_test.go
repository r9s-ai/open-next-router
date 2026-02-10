package cli

import (
	"bytes"
	"testing"
)

func TestResolveEncryptPlaintext_FromFlag(t *testing.T) {
	t.Parallel()

	got, err := resolveEncryptPlaintext("  hello  ", bytes.NewBufferString("ignored"), true)
	if err != nil {
		t.Fatalf("resolveEncryptPlaintext err=%v", err)
	}
	if got != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}

func TestResolveEncryptPlaintext_FromTerminalLine(t *testing.T) {
	t.Parallel()

	got, err := resolveEncryptPlaintext("", bytes.NewBufferString("secret\nsecond\n"), true)
	if err != nil {
		t.Fatalf("resolveEncryptPlaintext err=%v", err)
	}
	if got != "secret" {
		t.Fatalf("got=%q want=%q", got, "secret")
	}
}

func TestResolveEncryptPlaintext_FromPipe(t *testing.T) {
	t.Parallel()

	got, err := resolveEncryptPlaintext("", bytes.NewBufferString("secret\nsecond\n"), false)
	if err != nil {
		t.Fatalf("resolveEncryptPlaintext err=%v", err)
	}
	if got != "secret\nsecond" {
		t.Fatalf("got=%q want=%q", got, "secret\nsecond")
	}
}
