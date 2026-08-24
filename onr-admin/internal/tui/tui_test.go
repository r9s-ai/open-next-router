package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/controlplane"
)

func TestParseMetadata(t *testing.T) {
	metadata, err := parseMetadata("team=platform\nowner=onr\n")
	if err != nil || metadata["team"] != "platform" || metadata["owner"] != "onr" {
		t.Fatalf("metadata=(%v,%v)", metadata, err)
	}
	if _, err := parseMetadata("invalid"); err == nil {
		t.Fatal("expected invalid metadata error")
	}
}

func TestRedactIdentifier(t *testing.T) {
	if got := redactIdentifier("proj_1234567890"); got != "proj_1...7890" {
		t.Fatalf("redacted identifier=%q", got)
	}
	if got := redactIdentifier("short"); got != "short" {
		t.Fatalf("short identifier=%q", got)
	}
}

func TestAccessKeysModelIncludesAdministrativeStatuses(t *testing.T) {
	m := newAccessKeysModel(nil)
	expired := time.Now().Add(-time.Minute)
	m.records = []controlplane.AccessKeyRecord{
		{Name: "active", Status: "active", SubjectType: "api_key", SubjectID: "a"},
		{Name: "revoked", Status: "revoked", SubjectType: "api_key", SubjectID: "b"},
		{Name: "expired", Status: "active", SubjectType: "api_key", SubjectID: "c", ExpiresAt: &expired},
	}
	m.setRows()
	rows := m.table.Rows()
	if len(rows) != 3 || rows[1][1] != "revoked" || rows[2][1] != "expired" {
		t.Fatalf("rows=%v", rows)
	}
}

func TestRootModuleNavigation(t *testing.T) {
	m := newRootModel(nil, "./dumps")
	if m.active != overviewModule {
		t.Fatal("root should start on overview")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(rootModel).active != accessKeysModule {
		t.Fatal("tab should move to access keys")
	}
	if strings.Contains(updated.(rootModel).View(), "secret_hash") {
		t.Fatal("root view must not expose secret hashes")
	}
}
