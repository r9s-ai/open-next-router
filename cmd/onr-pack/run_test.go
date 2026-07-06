package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestRun_WritesBundledProvidersFile(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll providers: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.openai.com";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile openai.conf: %v", err)
	}
	outPath := filepath.Join(root, "dist", "providers.conf")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	contentBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile bundle: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, `provider "openai"`) {
		t.Fatalf("bundled output missing provider block: %q", content)
	}
	if !strings.Contains(stdout.String(), "bundled providers:") {
		t.Fatalf("stdout=%q want bundled summary", stdout.String())
	}
	if _, err := dslconfig.ValidateProvidersFile(outPath); err != nil {
		t.Fatalf("ValidateProvidersFile: %v", err)
	}
}

func TestRun_InvalidProvidersDoesNotWriteOutput(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "bad.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "bad" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com"
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile bad.conf: %v", err)
	}
	outPath := filepath.Join(root, "providers.conf")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--out", outPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run code=%d want=1 stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("output file should not exist, stat err=%v", err)
	}
	if !strings.Contains(stderr.String(), "validate providers failed") {
		t.Fatalf("stderr=%q want validation failure", stderr.String())
	}
}

func TestRun_CheckOnlyPrintsValidationResultWithoutWritingOutput(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll providers: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";
provider "openai" {
  defaults {
    upstream_config {
      base_url = "https://api.openai.com";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile openai.conf: %v", err)
	}
	outPath := filepath.Join(root, "dist", "providers.conf")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--check-only", "--out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("output file should not exist in check-only mode, stat err=%v", err)
	}
	if !strings.Contains(stdout.String(), "validate providers:") {
		t.Fatalf("stdout=%q want validation summary", stdout.String())
	}
	if strings.Contains(stdout.String(), "bundled providers:") {
		t.Fatalf("stdout=%q should not contain bundle summary", stdout.String())
	}
}

func TestRun_CheckRequiredUsageFailsForChatCompletionsWithoutUsageExtract(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "openai-compatible" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }
  match api = "chat.completions" {
    upstream {
      set_path "/v1/chat/completions";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile providers.conf: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--check-only", "--check", "required-usage"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run code=%d want=1 stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "extra checks failed") {
		t.Fatalf("stderr=%q want extra check failure", stderr.String())
	}
	if !strings.Contains(stderr.String(), "fix: add metrics") {
		t.Fatalf("stderr=%q want fix hint", stderr.String())
	}
}

func TestRun_CheckRequiredUsagePassesWithUsageExtract(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "openai-compatible" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }
  match api = "chat.completions" {
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token path="$.usage.completion_tokens";
    }
    upstream {
      set_path "/v1/chat/completions";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile providers.conf: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--check-only", "--check", "required-usage"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "validate providers:") {
		t.Fatalf("stdout=%q want validation summary", stdout.String())
	}
}

func TestRun_CheckRequiredUsageIsOptIn(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "openai-compatible" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }
  match api = "chat.completions" {
    upstream {
      set_path "/v1/chat/completions";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile providers.conf: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--check-only"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRun_CheckAcceptsCommaSeparatedAll(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";
provider "openai-compatible" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }
  match api = "chat.completions" {
    metrics {
      usage_extract custom;
      usage_fact input token path="$.usage.prompt_tokens";
      usage_fact output token path="$.usage.completion_tokens";
    }
    upstream {
      set_path "/v1/chat/completions";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile providers.conf: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--providers", sourcePath, "--check-only", "--check", "all,required-usage"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRun_VersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatalf("stdout should contain version")
	}
}

func TestRun_VersionFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatalf("stdout should contain version")
	}
}

func TestRun_ConfigFlagIsNotSupported(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--config", "./onr.yaml"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run code=%d want=2 stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined: -config") {
		t.Fatalf("stderr=%q want unsupported config flag error", stderr.String())
	}
}

func TestRun_HelpIncludesExamples(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"onr-pack --providers ./config/onr.conf --out ./dist/providers.conf",
		"onr-pack --providers ./config/onr.conf --check-only --check required-usage",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout=%q missing %q", stdout.String(), want)
		}
	}
}
