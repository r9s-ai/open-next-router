package dslconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func mustReadDSLConfigTestData(t *testing.T, rel string) []byte {
	t.Helper()

	path := filepath.Join("testdata", rel)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read testdata %q: %v", path, err)
	}
	return b
}

func mustReadSharedTestData(t *testing.T, rel string) []byte {
	t.Helper()

	path := filepath.Join("..", "..", "..", "testdata", "real_upstream", rel)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared testdata %q: %v", path, err)
	}
	return b
}

func mustLoadProviderMatchConfigsTB(tb testing.TB, providerFile string, api string, stream bool) (UsageExtractConfig, FinishReasonExtractConfig) {
	tb.Helper()

	path := filepath.Join("..", "..", "..", "config", "providers", providerFile)
	pf, err := ValidateProviderFile(path)
	if err != nil {
		tb.Fatalf("ValidateProviderFile(%q): %v", path, err)
	}

	meta := &dslmeta.Meta{API: api, IsStream: stream}

	usageCfg, ok := pf.Usage.Select(meta)
	if !ok {
		tb.Fatalf("expected usage config for provider=%q api=%q stream=%t", providerFile, api, stream)
	}

	finishCfg, _ := pf.Finish.Select(meta)
	return usageCfg, finishCfg
}

func mustLoadProviderMatchConfigs(t *testing.T, providerFile string, api string, stream bool) (UsageExtractConfig, FinishReasonExtractConfig) {
	t.Helper()
	return mustLoadProviderMatchConfigsTB(t, providerFile, api, stream)
}
