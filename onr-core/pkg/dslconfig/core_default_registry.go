package dslconfig

import (
	"log"
	"sort"
	"sync"
)

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

// DefaultRegistry returns a non-nil shared registry instance.
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
		skipped := append([]string(nil), res.SkippedFiles...)
		sort.Strings(skipped)
		details := make([]string, 0, len(skipped))
		for _, file := range skipped {
			reason := res.SkippedReasons[file]
			if reason == "" {
				reason = "unknown"
			}
			details = append(details, file+": "+reason)
		}
		log.Printf("dsl providers skipped: dir=%s files=%v details=%v", providersDir, skipped, details)
	}
	return nil
}
