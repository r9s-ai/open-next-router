package dslconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProvidersFile_MultipleProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "a" { defaults { upstream_config { base_url = "https://a.example.com"; } } }
provider "b" { defaults { upstream_config { base_url = "https://b.example.com"; } } }
`), 0o600))

	res, err := ValidateProvidersFile(path)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, res.LoadedProviders)
}

func TestReloadFromFile_DuplicateProviderRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "dup" { defaults { upstream_config { base_url = "https://a.example.com"; } } }
provider "dup" { defaults { upstream_config { base_url = "https://b.example.com"; } } }
`), 0o600))

	reg := NewRegistry()
	_, err := reg.ReloadFromFile(path)
	require.Error(t, err)
}

func TestValidateProvidersFile_GlobalUsageMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}

provider "a" {
  defaults {
    upstream_config { base_url = "https://a.example.com"; }
    metrics { usage_extract shared_tokens; }
  }
}
`), 0o600))

	res, err := ValidateProvidersFile(path)
	require.NoError(t, err)
	require.Equal(t, []string{"a"}, res.LoadedProviders)

	reg := NewRegistry()
	_, err = reg.ReloadFromFile(path)
	require.NoError(t, err)
	pf, ok := reg.GetProvider("a")
	require.True(t, ok)
	require.Equal(t, usageModeCustom, normalizeUsageMode(pf.Usage.Defaults.Mode))
	require.Len(t, pf.Usage.Defaults.CompiledFacts(nil), 2)
}

func TestValidateProvidersFile_IncludeProvidersDir(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	require.NoError(t, os.MkdirAll(providersDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

include "providers";

usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(providersDir, "openai.conf"), []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    metrics { usage_extract shared_tokens; }
  }
}
`), 0o600))

	res, err := ValidateProvidersFile(filepath.Join(root, "onr.conf"))
	require.NoError(t, err)
	require.Equal(t, []string{"openai"}, res.LoadedProviders)

	reg := NewRegistry()
	_, err = reg.ReloadFromFile(filepath.Join(root, "onr.conf"))
	require.NoError(t, err)
	_, ok := reg.GetProvider("openai")
	require.True(t, ok)
}
