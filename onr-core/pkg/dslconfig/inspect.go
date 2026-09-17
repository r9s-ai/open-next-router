package dslconfig

import "os"

// InspectProvidersPath returns validated provider definitions for offline tools.
// File handler references are retained without reading JS files or preparing a
// runtime. Use Registry reload methods before passing handlers to NewSession.
func InspectProvidersPath(path string) ([]ProviderFile, error) {
	if _, err := ValidateProvidersPath(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	content, err := BundleProvidersPath(path)
	if err != nil {
		return nil, err
	}
	state := newModeRegistryState()
	if !info.IsDir() {
		state, _, err = loadGlobalModeRegistryState(globalConfigPathForMergedProvidersFile(path))
		if err != nil {
			return nil, err
		}
	}
	providers, names, err := parseProvidersFromMergedFile(path, content, state)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderFile, 0, len(names))
	for _, name := range names {
		out = append(out, providers[name])
	}
	return out, nil
}
