package cli

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	cfgpkg "github.com/r9s-ai/open-next-router/pkg/config"
)

func resolveProviderSourcePath(cfg *cfgpkg.Config, override string) string {
	if v := strings.TrimSpace(override); v != "" {
		return v
	}
	path, _ := cfgpkg.ResolveProviderDSLSource(cfg)
	return path
}

func loadRegistryFromProviderSource(path string) (*dslconfig.Registry, dslconfig.LoadResult, error) {
	reg := dslconfig.NewRegistry()
	res, err := reg.ReloadFromPath(strings.TrimSpace(path))
	if err != nil {
		return nil, dslconfig.LoadResult{}, err
	}
	return reg, res, nil
}
