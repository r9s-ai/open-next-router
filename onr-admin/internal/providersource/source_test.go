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

func TestResolve_MissingDirectorySource(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "providers")
	info, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if info.SourcePath != dir || info.EditablePath != dir || info.SourceIsFile || info.EditableIsFile {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestResolve_MixedInlineAndIncludedDir_DoesNotFail(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "anthropic.conf"), []byte(`
syntax "next-router/0.1";
provider "anthropic" {
  defaults { upstream_config { base_url = "https://api.anthropic.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	info, err := Resolve(sourcePath)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if info.EditablePath != "" {
		t.Fatalf("expected no global editable path, got %+v", info)
	}
	if target, err := ResolveProviderTarget(info, "openai"); err != nil || target.Path != sourcePath {
		t.Fatalf("ResolveProviderTarget(openai) got (%+v,%v), want source file", target, err)
	}
	if target, err := ResolveProviderTarget(info, "anthropic"); err != nil || target.Path != filepath.Join(providersDir, "anthropic.conf") {
		t.Fatalf("ResolveProviderTarget(anthropic) got (%+v,%v), want included provider file", target, err)
	}
	if target, err := ResolveProviderTarget(info, "gemini"); err != nil || target.Path != filepath.Join(providersDir, "gemini.conf") {
		t.Fatalf("ResolveProviderTarget(gemini) got (%+v,%v), want new file under providers dir", target, err)
	}
}

func TestResolve_MultipleProviderDirs_DoesNotFail(t *testing.T) {
	root := t.TempDir()
	aDir := filepath.Join(root, "providers-a")
	bDir := filepath.Join(root, "providers-b")
	if err := os.MkdirAll(aDir, 0o750); err != nil {
		t.Fatalf("MkdirAll aDir: %v", err)
	}
	if err := os.MkdirAll(bDir, 0o750); err != nil {
		t.Fatalf("MkdirAll bDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "openai.conf"), []byte(`
syntax "next-router/0.1";
provider "openai" { defaults { upstream_config { base_url = "https://api.openai.com"; } } }
`), 0o600); err != nil {
		t.Fatalf("WriteFile openai: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "anthropic.conf"), []byte(`
syntax "next-router/0.1";
provider "anthropic" { defaults { upstream_config { base_url = "https://api.anthropic.com"; } } }
`), 0o600); err != nil {
		t.Fatalf("WriteFile anthropic: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
include providers-a/*.conf;
include providers-b/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	info, err := Resolve(sourcePath)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if info.EditablePath != "" {
		t.Fatalf("expected no global editable path, got %+v", info)
	}
	if target, err := ResolveProviderTarget(info, "openai"); err != nil || target.Path != filepath.Join(aDir, "openai.conf") {
		t.Fatalf("ResolveProviderTarget(openai) got (%+v,%v), want providers-a/openai.conf", target, err)
	}
	if _, err := ResolveProviderTarget(info, "gemini"); err == nil {
		t.Fatalf("expected ambiguous target error for new provider across multiple dirs")
	}
}
