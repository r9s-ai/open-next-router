package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProviderFile_RejectsUnsupportedMatchAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "xxx" {
    upstream {
      set_path "/v1/xxx";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ValidateProviderFile(path)
	if err == nil {
		t.Fatalf("expected unsupported match api validation error")
	}
	if !strings.Contains(err.Error(), `match[0].api "xxx" is unsupported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProvidersDir_RejectsUnsupportedMatchAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test data file.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "xxx" {
    upstream {
      set_path "/v1/xxx";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := ValidateProvidersDir(dir)
	if err == nil {
		t.Fatalf("expected unsupported match api validation error")
	}
	if !strings.Contains(err.Error(), `match[0].api "xxx" is unsupported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
