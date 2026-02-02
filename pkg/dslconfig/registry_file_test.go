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
