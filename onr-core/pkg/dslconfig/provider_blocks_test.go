package dslconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListProviderBlocksAndUpsert(t *testing.T) {
	content := `syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_fact input token path="$.usage.input_tokens";
}

provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}

provider "anthropic" {
  defaults { upstream_config { base_url = "https://api.anthropic.com"; } }
}
`

	blocks, err := ListProviderBlocks("providers.conf", content)
	if err != nil {
		t.Fatalf("ListProviderBlocks: %v", err)
	}
	if got, want := len(blocks), 2; got != want {
		t.Fatalf("blocks len=%d want=%d", got, want)
	}
	if got, want := blocks[0].Name, "openai"; got != want {
		t.Fatalf("blocks[0].Name=%q want=%q", got, want)
	}
	if got, want := blocks[1].Name, "anthropic"; got != want {
		t.Fatalf("blocks[1].Name=%q want=%q", got, want)
	}

	block, ok, err := ExtractProviderBlockOptional("providers.conf", content, "anthropic")
	if err != nil {
		t.Fatalf("ExtractProviderBlockOptional: %v", err)
	}
	if !ok {
		t.Fatalf("expected anthropic block")
	}
	if got, ok, err := FindProviderNameOptional("providers.conf", block); err != nil || !ok || got != "anthropic" {
		t.Fatalf("FindProviderNameOptional got=(%q,%v,%v) want=(anthropic,true,nil)", got, ok, err)
	}

	replaced, err := UpsertProviderBlock("providers.conf", content, "anthropic", `
provider "anthropic" {
  defaults { upstream_config { base_url = "https://example.com"; } }
}
`)
	if err != nil {
		t.Fatalf("UpsertProviderBlock replace: %v", err)
	}
	if got, ok, err := ExtractProviderBlockOptional("providers.conf", replaced, "anthropic"); err != nil || !ok {
		t.Fatalf("ExtractProviderBlockOptional replaced err=%v ok=%v", err, ok)
	} else if want := `provider "anthropic" {
  defaults { upstream_config { base_url = "https://example.com"; } }
}`; got != want+"\n" && got != want {
		t.Fatalf("replaced block=%q", got)
	}

	appended, err := UpsertProviderBlock("providers.conf", content, "gemini", `
provider "gemini" {
  defaults { upstream_config { base_url = "https://generativelanguage.googleapis.com"; } }
}
`)
	if err != nil {
		t.Fatalf("UpsertProviderBlock append: %v", err)
	}
	if _, ok, err := ExtractProviderBlockOptional("providers.conf", appended, "gemini"); err != nil || !ok {
		t.Fatalf("ExtractProviderBlockOptional appended err=%v ok=%v", err, ok)
	}
}

func TestListIncludedFiles(t *testing.T) {
	root := t.TempDir()
	modesDir := filepath.Join(root, "modes")
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(modesDir, 0o750); err != nil {
		t.Fatalf("MkdirAll modes: %v", err)
	}
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll providers: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modesDir, "usage_modes.conf"), []byte(`syntax "next-router/0.1";`), 0o600); err != nil {
		t.Fatalf("WriteFile usage_modes.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults { upstream_config { base_url = "https://api.openai.com"; } }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile openai.conf: %v", err)
	}
	onrPath := filepath.Join(root, "onr.conf")
	onrContent := `
syntax "next-router/0.1";
include modes/*.conf;
include providers/*.conf;
`
	if err := os.WriteFile(onrPath, []byte(onrContent), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}

	files, err := ListIncludedFiles(onrPath, onrContent)
	if err != nil {
		t.Fatalf("ListIncludedFiles: %v", err)
	}
	if got, want := len(files), 2; got != want {
		t.Fatalf("included files len=%d want=%d: %v", got, want, files)
	}
}
