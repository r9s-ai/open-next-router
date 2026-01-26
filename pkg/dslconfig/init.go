package dslconfig

import (
	"log"
	"sync"
)

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

func DefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry()
	})
	return defaultRegistry
}

// ReloadDefault loads provider configs from dir into the default registry.
// It is safe to call multiple times; updates are applied atomically.
func ReloadDefault(providersDir string) error {
	reg := DefaultRegistry()
	res, err := reg.ReloadFromDir(providersDir)
	if err != nil {
		return err
	}
	log.Printf("dsl providers loaded: dir=%s providers=%v", providersDir, res.LoadedProviders)
	if len(res.SkippedFiles) > 0 {
		log.Printf("dsl providers skipped: dir=%s files=%v", providersDir, res.SkippedFiles)
	}
	return nil
}
