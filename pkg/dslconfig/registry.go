package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type ProviderFile struct {
	Name     string
	Path     string
	Content  string
	Routing  ProviderRouting
	Headers  ProviderHeaders
	Request  ProviderRequestTransform
	Response ProviderResponse
	Error    ProviderError
	Usage    ProviderUsage
	Finish   ProviderFinishReason
	Balance  ProviderBalance
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]ProviderFile
}

func NewRegistry() *Registry {
	return &Registry{
		providers: map[string]ProviderFile{},
	}
}

func (r *Registry) ListProviderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) GetProvider(name string) (ProviderFile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[normalizeProviderName(name)]
	return p, ok
}

type LoadResult struct {
	LoadedProviders []string
	SkippedFiles    []string
}

const providerConfExt = ".conf"

func normalizeProviderName(s string) string {
	return NormalizeProviderName(s)
}

var providerNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

func validateProviderName(name string) error {
	if !providerNamePattern.MatchString(name) {
		return fmt.Errorf("invalid provider name %q, expected pattern %s", name, providerNamePattern.String())
	}
	return nil
}

func (r *Registry) ReloadFromDir(providersDir string) (LoadResult, error) {
	dir := strings.TrimSpace(providersDir)
	if dir == "" {
		return LoadResult{}, fmt.Errorf("providers dir is empty")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return LoadResult{}, nil
		}
		return LoadResult{}, fmt.Errorf("read providers dir %q: %w", dir, err)
	}

	next := map[string]ProviderFile{}
	loaded := make([]string, 0)
	skipped := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != providerConfExt {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// #nosec G304 -- provider files are loaded from a configured directory; filenames come from ReadDir of that directory.
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		content, err := preprocessIncludes(path, string(contentBytes))
		if err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		providerName, err := findProviderName(path, content)
		if err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		providerName = normalizeProviderName(providerName)
		expected := normalizeProviderName(strings.TrimSuffix(entry.Name(), providerConfExt))
		if err := validateProviderName(providerName); err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		if err := validateProviderName(expected); err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		if providerName != expected {
			skipped = append(skipped, entry.Name())
			continue
		}

		if _, exists := next[providerName]; exists {
			skipped = append(skipped, entry.Name())
			continue
		}

		routing, headers, req, response, perr, usage, finish, balance, err := parseProviderConfig(path, content)
		if err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		if err := validateProviderBaseURL(path, providerName, routing); err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		if err := validateProviderHeaders(path, providerName, headers); err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		if err := validateProviderBalance(path, providerName, balance); err != nil {
			skipped = append(skipped, entry.Name())
			continue
		}
		next[providerName] = ProviderFile{
			Name:     providerName,
			Path:     path,
			Content:  content,
			Routing:  routing,
			Headers:  headers,
			Request:  req,
			Response: response,
			Error:    perr,
			Usage:    usage,
			Finish:   finish,
			Balance:  balance,
		}
		loaded = append(loaded, providerName)
	}

	sort.Strings(loaded)
	sort.Strings(skipped)

	r.mu.Lock()
	r.providers = next
	r.mu.Unlock()

	return LoadResult{LoadedProviders: loaded, SkippedFiles: skipped}, nil
}
