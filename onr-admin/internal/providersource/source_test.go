package providersource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_DirectorySource(t *testing.T) {
	dir := t.TempDir()
	info, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if info.SourcePath != dir || info.EditablePath != dir || info.SourceIsFile || info.EditableIsFile {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestResolve_FileSourceWithIncludedProvidersDir(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	info, err := Resolve(sourcePath)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !info.SourceIsFile || info.EditableIsFile {
		t.Fatalf("unexpected source flags: %+v", info)
	}
	if got, want := info.EditablePath, providersDir; got != want {
		t.Fatalf("EditablePath=%q want=%q", got, want)
	}
}

func TestResolve_MergedProviderFileSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	info, err := Resolve(sourcePath)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !info.SourceIsFile || !info.EditableIsFile {
		t.Fatalf("unexpected source flags: %+v", info)
	}
	if got, want := info.EditablePath, sourcePath; got != want {
		t.Fatalf("EditablePath=%q want=%q", got, want)
	}
}
