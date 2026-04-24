package dslconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateProviderFile_ModelsBlock_OpenAI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test fixture.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

models_mode "openai" {}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    auth {
      auth_bearer;
    }
    models {
      models_mode openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if got := pf.Models.Defaults.Mode; got != modelsModeOpenAI {
		t.Fatalf("models mode=%q", got)
	}
	cfg, ok := pf.Models.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if cfg.Path != "/v1/models" {
		t.Fatalf("models path=%q", cfg.Path)
	}
	if !reflect.DeepEqual(cfg.IDPaths, []string{"$.data[*].id"}) {
		t.Fatalf("id paths=%v", cfg.IDPaths)
	}
}

func TestValidateProviderFile_ModelsBlock_CustomMissingIDPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	// #nosec G306 -- test fixture.
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      models_mode custom;
      method GET;
      path "/v1/my-models";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected models custom validate error")
	}
}

func TestValidateProviderFile_ModelsBlock_ImplicitCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      path "/v1/my-models";
      id_path "$.items[*].name";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Models.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if got, want := cfg.Mode, modelsModeCustom; got != want {
		t.Fatalf("models mode=%q want=%q", got, want)
	}
	if got, want := cfg.Method, "GET"; got != want {
		t.Fatalf("method=%q want=%q", got, want)
	}
}

func TestExtractModelIDs_GeminiRewriteAndAllow(t *testing.T) {
	cfg := ModelsQueryConfig{
		Mode:         modelsModeGemini,
		IDAllowRegex: "^(gemini-2\\.5|gemini-1\\.5)",
	}
	body := []byte(`{
  "models": [
    {"name": "models/gemini-2.5-flash"},
    {"name": "models/gemini-1.5-pro"},
    {"name": "models/text-embedding-004"}
  ]
}`)
	ids, err := ExtractModelIDs(&cfg, body)
	if err != nil {
		t.Fatalf("ExtractModelIDs: %v", err)
	}
	want := []string{"gemini-2.5-flash", "gemini-1.5-pro"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v want=%v", ids, want)
	}
}

func TestValidateProviderFile_LoadsSiblingOnrConfigModelsMode(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

models_mode "shared_openai_models" {
  models_mode openai;
  path "/v2/models";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	path := filepath.Join(providersDir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      models_mode shared_openai_models;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Models.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if got, want := cfg.Mode, modelsModeOpenAI; got != want {
		t.Fatalf("models mode=%q want=%q", got, want)
	}
	if got, want := cfg.Path, "/v2/models"; got != want {
		t.Fatalf("models path=%q want=%q", got, want)
	}
	if !reflect.DeepEqual(cfg.IDPaths, []string{"$.data[*].id"}) {
		t.Fatalf("id paths=%v", cfg.IDPaths)
	}
}

func TestValidateProviderFile_LoadsSiblingOnrConfigModelsModeImplicitCustom(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

models_mode "shared_custom_models" {
  path "/v2/models";
  id_path "$.items[*].name";
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile onr.conf: %v", err)
	}
	path := filepath.Join(providersDir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      models_mode shared_custom_models;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile provider: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Models.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if got, want := cfg.Mode, modelsModeCustom; got != want {
		t.Fatalf("models mode=%q want=%q", got, want)
	}
	if got, want := cfg.Path, "/v2/models"; got != want {
		t.Fatalf("models path=%q want=%q", got, want)
	}
	if !reflect.DeepEqual(cfg.IDPaths, []string{"$.items[*].name"}) {
		t.Fatalf("id paths=%v", cfg.IDPaths)
	}
}

func TestValidateProviderFile_UserModelsModeOverridesBuiltinName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

models_mode "openai" {
  models_mode custom;
  path "/v9/models";
  id_path "$.items[*].name";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      models_mode openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Models.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if got, want := cfg.Mode, modelsModeOpenAI; got != want {
		t.Fatalf("models mode=%q want=%q", got, want)
	}
	if got, want := cfg.Path, "/v9/models"; got != want {
		t.Fatalf("models path=%q want=%q", got, want)
	}
	if !reflect.DeepEqual(cfg.IDPaths, []string{"$.items[*].name"}) {
		t.Fatalf("id paths=%v", cfg.IDPaths)
	}
}

func TestValidateProviderFile_ImplicitBuiltinModelsModeByName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

models_mode "openai" {
  path "/v2/models";
}

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      models_mode openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	cfg, ok := pf.Models.Select(nil)
	if !ok {
		t.Fatalf("expected models config selected")
	}
	if got, want := cfg.Mode, modelsModeOpenAI; got != want {
		t.Fatalf("models mode=%q want=%q", got, want)
	}
	if got, want := cfg.Path, "/v2/models"; got != want {
		t.Fatalf("models path=%q want=%q", got, want)
	}
	if !reflect.DeepEqual(cfg.IDPaths, []string{"$.data[*].id"}) {
		t.Fatalf("id paths=%v", cfg.IDPaths)
	}
}

func TestValidateProviderFile_ModelsModeRequiresGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "demo" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    models {
      models_mode openai;
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := ValidateProviderFile(path); err == nil {
		t.Fatalf("expected missing global models_mode to fail validation")
	}
}
