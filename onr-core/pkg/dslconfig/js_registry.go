package dslconfig

import (
	"fmt"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
)

// ConfigureJS must be called before the registry is exposed to concurrent users.
// It enables strict, atomic reloads: a skipped provider must not remove a policy.
func (r *Registry) ConfigureJS(cfg jsext.RuntimeConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	r.jsConfig = cfg.Clone()
	r.strictJS = true
	return nil
}
func (r *Registry) prepareJS(next map[string]ProviderFile, skipped []string) error {
	if r.strictJS && len(skipped) > 0 {
		return fmt.Errorf("atomic provider reload rejected skipped files: %v", skipped)
	}
	compiler := jsext.NewCompiler(r.jsConfig)
	for name, pf := range next {
		js, err := compiler.Prepare(pf.JS, name)
		if err != nil {
			return fmt.Errorf("provider %s: %w", name, err)
		}
		pf.JS = js
		next[name] = pf
	}
	return nil
}

// RegistrySnapshot provides read-only access to one published version.
type RegistrySnapshot struct{ providers map[string]ProviderFile }

func (s RegistrySnapshot) GetProvider(name string) (ProviderFile, bool) {
	p, ok := s.providers[normalizeProviderName(name)]
	return p, ok
}

// Snapshot captures the immutable provider map without copying it. Requests
// retain this handle across attempts and never observe a partial reload.
func (r *Registry) Snapshot() RegistrySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return RegistrySnapshot{providers: r.providers}
}

// ReloadFromPathWithJS prepares DSL, modes, JS sources and HTTP grants in a
// private registry. Publication changes all of them under the same lock.
func (r *Registry) ReloadFromPathWithJS(path string, cfg jsext.RuntimeConfig) (LoadResult, error) {
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()
	candidate := NewRegistry()
	if err := candidate.ConfigureJS(cfg); err != nil {
		return LoadResult{}, err
	}
	result, err := candidate.ReloadFromPath(path)
	if err != nil {
		return LoadResult{}, err
	}
	r.mu.Lock()
	r.providers = candidate.providers
	r.jsConfig = candidate.jsConfig
	r.strictJS = true
	r.mu.Unlock()
	return result, nil
}
