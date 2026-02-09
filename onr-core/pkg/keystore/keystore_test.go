package keystore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_AccessKeys_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := os.WriteFile(path, []byte(`
access_keys:
  - name: "client-a"
    value: ""
providers: {}
`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("ONR_ACCESS_KEY_CLIENT_A", "ak-xxx")
	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load err=%v", err)
	}
	if _, ok := st.MatchAccessKey("ak-xxx"); !ok {
		t.Fatalf("expected match")
	}
}

func TestLoad_Empty_All(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.yaml")
	if err := os.WriteFile(path, []byte(`providers: {}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected error")
	}
}
