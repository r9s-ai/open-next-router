package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateModelsGetFlags(t *testing.T) {
	if err := validateModelsGetFlags(modelsGetOptions{provider: "openai"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateModelsGetFlags(modelsGetOptions{}); err == nil {
		t.Fatalf("expected missing provider error")
	}
	if err := validateModelsGetFlags(modelsGetOptions{provider: "openai", allProviders: true}); err == nil {
		t.Fatalf("expected mutually exclusive flags error")
	}
}

func TestRunModelsGetWithOptions_SingleProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/models" {
			t.Fatalf("path=%q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	providersDir := filepath.Join(dir, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("mkdir providers: %v", err)
	}
	conf := `
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config {
      base_url = "` + srv.URL + `";
    }
    auth {
      auth_bearer;
    }
    models {
      models_mode openai;
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(conf), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}
	onrConf := `
syntax "next-router/0.1";

models_mode "openai" {}
`
	if err := os.WriteFile(filepath.Join(dir, "onr.conf"), []byte(onrConf), 0o600); err != nil {
		t.Fatalf("write onr.conf: %v", err)
	}
	keysPath := filepath.Join(dir, "keys.yaml")
	keysYAML := `
providers:
  openai:
    keys:
      - name: default
        value: test-key
`
	if err := os.WriteFile(keysPath, []byte(keysYAML), 0o600); err != nil {
		t.Fatalf("write keys: %v", err)
	}
	cfgPath := filepath.Join(dir, "onr.yaml")
	cfgYAML := "auth:\n  api_key: \"x\"\nkeys:\n  file: \"" + keysPath + "\"\nproviders:\n  dir: \"" + providersDir + "\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := runModelsGetWithOptions(modelsGetOptions{
		cfgPath:  cfgPath,
		provider: "openai",
	})
	_ = w.Close()
	os.Stdout = old
	if runErr != nil {
		t.Fatalf("runModelsGetWithOptions: %v", runErr)
	}

	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	out := strings.TrimSpace(string(body))
	if out != "gpt-4.1\ngpt-4o-mini" {
		t.Fatalf("stdout=%q", out)
	}
}
