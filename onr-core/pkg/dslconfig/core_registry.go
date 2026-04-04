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
	Models   ProviderModels
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
	SkippedReasons  map[string]string
	Warnings        []ValidationWarning
}

type registryDirCandidate struct {
	entryName string
	path      string
	content   string
}

type modeFileIndex struct {
	usage        map[string][]string
	finishReason map[string][]string
	models       map[string][]string
	balance      map[string][]string
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
	skippedReasons := make(map[string]string)
	modeState, _, err := loadGlobalModeRegistryState(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	modeState, candidates, modeFiles, skipped, skippedReasons := collectRegistryDirCandidates(dir, entries, modeState, skipped, skippedReasons)
	resolvedState, candidates, skipped, skippedReasons, err := resolveRegistryDirModeState(modeState, candidates, modeFiles, skipped, skippedReasons)
	if err != nil {
		return LoadResult{}, err
	}
	loadProvidersFromRegistryDirCandidates(next, &loaded, &skipped, skippedReasons, candidates, resolvedState)

	sort.Strings(loaded)
	sort.Strings(skipped)

	r.mu.Lock()
	r.providers = next
	r.mu.Unlock()

	return LoadResult{LoadedProviders: loaded, SkippedFiles: skipped, SkippedReasons: skippedReasons}, nil
}

func newModeFileIndex() modeFileIndex {
	return modeFileIndex{
		usage:        map[string][]string{},
		finishReason: map[string][]string{},
		models:       map[string][]string{},
		balance:      map[string][]string{},
	}
}

func (idx *modeFileIndex) record(path string, local modeRegistryState) {
	for name := range local.usage {
		idx.usage[path] = append(idx.usage[path], name)
	}
	for name := range local.finishReason {
		idx.finishReason[path] = append(idx.finishReason[path], name)
	}
	for name := range local.models {
		idx.models[path] = append(idx.models[path], name)
	}
	for name := range local.balance {
		idx.balance[path] = append(idx.balance[path], name)
	}
}

func collectRegistryDirCandidates(dir string, entries []os.DirEntry, modeState modeRegistryState, skipped []string, skippedReasons map[string]string) (modeRegistryState, []registryDirCandidate, modeFileIndex, []string, map[string]string) {
	candidates := make([]registryDirCandidate, 0)
	modeFiles := newModeFileIndex()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != providerConfExt {
			continue
		}
		candidate, localModes, err := loadRegistryDirCandidate(dir, entry)
		if err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		if err := modeState.merge(candidate.path, localModes); err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		modeFiles.record(candidate.path, localModes)
		candidates = append(candidates, candidate)
	}
	return modeState, candidates, modeFiles, skipped, skippedReasons
}

func loadRegistryDirCandidate(dir string, entry os.DirEntry) (registryDirCandidate, modeRegistryState, error) {
	path := filepath.Join(dir, entry.Name())
	// #nosec G304 -- provider files are loaded from a configured directory; filenames come from ReadDir of that directory.
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return registryDirCandidate{}, modeRegistryState{}, err
	}
	content, err := preprocessIncludes(path, string(contentBytes))
	if err != nil {
		return registryDirCandidate{}, modeRegistryState{}, err
	}
	localModes, err := parseLocalModeRegistryState(path, content)
	if err != nil {
		return registryDirCandidate{}, modeRegistryState{}, err
	}
	return registryDirCandidate{entryName: entry.Name(), path: path, content: content}, localModes, nil
}

func resolveRegistryDirModeState(modeState modeRegistryState, candidates []registryDirCandidate, modeFiles modeFileIndex, skipped []string, skippedReasons map[string]string) (modeRegistryState, []registryDirCandidate, []string, map[string]string, error) {
	state := modeState.clone()
	var err error
	state.usage, candidates, skipped, skippedReasons, err = resolveModeRegistryWithSkips[UsageExtractConfig, usageModeRegistry](candidates, skipped, skippedReasons, state.usagePaths, state.usage, modeFiles.usage, resolveUsageModeRegistry)
	if err != nil {
		return modeRegistryState{}, nil, skipped, skippedReasons, err
	}
	state.finishReason, candidates, skipped, skippedReasons, err = resolveModeRegistryWithSkips[FinishReasonExtractConfig, finishReasonModeRegistry](candidates, skipped, skippedReasons, state.finishReasonPaths, state.finishReason, modeFiles.finishReason, resolveFinishReasonModeRegistry)
	if err != nil {
		return modeRegistryState{}, nil, skipped, skippedReasons, err
	}
	state.models, candidates, skipped, skippedReasons, err = resolveModeRegistryWithSkips[ModelsQueryConfig, modelsModeRegistry](candidates, skipped, skippedReasons, state.modelsPaths, state.models, modeFiles.models, resolveModelsModeRegistry)
	if err != nil {
		return modeRegistryState{}, nil, skipped, skippedReasons, err
	}
	state.balance, candidates, skipped, skippedReasons, err = resolveModeRegistryWithSkips[BalanceQueryConfig, balanceModeRegistry](candidates, skipped, skippedReasons, state.balancePaths, state.balance, modeFiles.balance, resolveBalanceModeRegistry)
	if err != nil {
		return modeRegistryState{}, nil, skipped, skippedReasons, err
	}
	return state, candidates, skipped, skippedReasons, nil
}

func resolveModeRegistryWithSkips[T any, M ~map[string]T](candidates []registryDirCandidate, skipped []string, skippedReasons map[string]string, paths map[string]string, registry M, files map[string][]string, resolve func(map[string]string, M) (M, error)) (M, []registryDirCandidate, []string, map[string]string, error) {
	for {
		resolved, err := resolve(paths, registry)
		if err == nil {
			return resolved, candidates, skipped, skippedReasons, nil
		}
		skippedFile := usageModeFileFromError(err.Error(), files)
		if skippedFile == "" {
			var zero M
			return zero, candidates, skipped, skippedReasons, err
		}
		entryName := filepath.Base(skippedFile)
		if _, exists := skippedReasons[entryName]; exists {
			var zero M
			return zero, candidates, skipped, skippedReasons, err
		}
		skipped = append(skipped, entryName)
		skippedReasons[entryName] = err.Error()
		for _, modeName := range files[skippedFile] {
			delete(registry, modeName)
			delete(paths, modeName)
		}
		delete(files, skippedFile)
		candidates = filterRegistryDirCandidates(candidates, skippedFile)
	}
}

func filterRegistryDirCandidates(candidates []registryDirCandidate, skippedFile string) []registryDirCandidate {
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if candidate.path == skippedFile {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func loadProvidersFromRegistryDirCandidates(next map[string]ProviderFile, loaded, skipped *[]string, skippedReasons map[string]string, candidates []registryDirCandidate, resolvedState modeRegistryState) {
	for _, candidate := range candidates {
		pf, hasProvider, err := validateAndBuildProviderFile(candidate.path, candidate.content, resolvedState.usage, resolvedState.finishReason, resolvedState.models, resolvedState.balance)
		if err != nil {
			*skipped = append(*skipped, candidate.entryName)
			skippedReasons[candidate.entryName] = err.Error()
			continue
		}
		if !hasProvider {
			continue
		}
		if _, exists := next[pf.Name]; exists {
			*skipped = append(*skipped, candidate.entryName)
			skippedReasons[candidate.entryName] = fmt.Sprintf("duplicate provider name %q", pf.Name)
			continue
		}
		next[pf.Name] = pf
		*loaded = append(*loaded, pf.Name)
	}
}

func (r *Registry) ReloadFromPath(path string) (LoadResult, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return LoadResult{}, fmt.Errorf("providers path is empty")
	}
	info, err := os.Stat(p)
	if err != nil {
		return LoadResult{}, err
	}
	if info.IsDir() {
		return r.ReloadFromDir(p)
	}
	return r.ReloadFromFile(p)
}
