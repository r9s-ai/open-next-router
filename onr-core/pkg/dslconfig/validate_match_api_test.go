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

func TestValidateProviderFile_AcceptsClaudeCountTokensAPI(t *testing.T) {
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

  match api = "claude.messages.count_tokens" stream = false {
    upstream {
      set_path "/v1/messages/count_tokens";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
}

func TestValidateProviderFile_AcceptsSeedanceVideoAPIs(t *testing.T) {
	t.Parallel()

	apis := []struct {
		name string
		api  string
		path string
	}{
		{"generations", "seedance.videos.generations", "/api/v3/contents/generations/tasks"},
		{"get", "seedance.videos.get", "/api/v3/contents/generations/tasks/{task_id}"},
		{"delete", "seedance.videos.delete", "/api/v3/contents/generations/tasks/{task_id}"},
	}
	for _, tc := range apis {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "demo.conf")
			content := `syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://ark.cn-beijing.volces.com";
    }
  }

  match api = "` + tc.api + `" {
    upstream {
      set_path "` + tc.path + `";
    }
  }
}
`
			// #nosec G306 -- test data file.
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			if _, err := ValidateProviderFile(path); err != nil {
				t.Fatalf("ValidateProviderFile(%q): %v", tc.api, err)
			}
		})
	}
}
