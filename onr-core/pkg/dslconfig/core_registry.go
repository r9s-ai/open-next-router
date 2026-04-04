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

	type candidate struct {
		entryName string
		path      string
		content   string
	}
	next := map[string]ProviderFile{}
	loaded := make([]string, 0)
	skipped := make([]string, 0)
	skippedReasons := make(map[string]string)
	candidates := make([]candidate, 0)
	rawModes, modePaths, _, err := loadGlobalUsageModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	rawFinishReasonModes, finishReasonModePaths, _, err := loadGlobalFinishReasonModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	rawModelsModes, modelsModePaths, _, err := loadGlobalModelsModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	rawBalanceModes, balanceModePaths, _, err := loadGlobalBalanceModesFromFile(globalConfigPathForProvidersDir(dir))
	if err != nil {
		return LoadResult{}, err
	}
	if rawModes == nil {
		rawModes = usageModeRegistry{}
	}
	if modePaths == nil {
		modePaths = map[string]string{}
	}
	if rawFinishReasonModes == nil {
		rawFinishReasonModes = finishReasonModeRegistry{}
	}
	if finishReasonModePaths == nil {
		finishReasonModePaths = map[string]string{}
	}
	if rawModelsModes == nil {
		rawModelsModes = modelsModeRegistry{}
	}
	if modelsModePaths == nil {
		modelsModePaths = map[string]string{}
	}
	if rawBalanceModes == nil {
		rawBalanceModes = balanceModeRegistry{}
	}
	if balanceModePaths == nil {
		balanceModePaths = map[string]string{}
	}
	modeFiles := map[string][]string{}
	finishReasonModeFiles := map[string][]string{}
	modelModeFiles := map[string][]string{}
	balanceModeFiles := map[string][]string{}
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
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		content, err := preprocessIncludes(path, string(contentBytes))
		if err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		defs, err := parseGlobalUsageModes(path, content)
		if err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		finishDefs, err := parseGlobalFinishReasonModes(path, content)
		if err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		modelDefs, err := parseGlobalModelsModes(path, content)
		if err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		balanceDefs, err := parseGlobalBalanceModes(path, content)
		if err != nil {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = err.Error()
			continue
		}
		duplicateMode := ""
		duplicateModePath := ""
		for name := range defs {
			if prev, exists := modePaths[name]; exists {
				duplicateMode = name
				duplicateModePath = prev
				break
			}
		}
		if duplicateMode != "" {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = fmt.Sprintf("duplicate usage_mode %q (already in %q)", duplicateMode, duplicateModePath)
			continue
		}
		duplicateFinishReasonMode := ""
		duplicateFinishReasonModePath := ""
		for name := range finishDefs {
			if prev, exists := finishReasonModePaths[name]; exists {
				duplicateFinishReasonMode = name
				duplicateFinishReasonModePath = prev
				break
			}
		}
		if duplicateFinishReasonMode != "" {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = fmt.Sprintf("duplicate finish_reason_mode %q (already in %q)", duplicateFinishReasonMode, duplicateFinishReasonModePath)
			continue
		}
		duplicateModelMode := ""
		duplicateModelModePath := ""
		for name := range modelDefs {
			if prev, exists := modelsModePaths[name]; exists {
				duplicateModelMode = name
				duplicateModelModePath = prev
				break
			}
		}
		if duplicateModelMode != "" {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = fmt.Sprintf("duplicate models_mode %q (already in %q)", duplicateModelMode, duplicateModelModePath)
			continue
		}
		duplicateBalanceMode := ""
		duplicateBalanceModePath := ""
		for name := range balanceDefs {
			if prev, exists := balanceModePaths[name]; exists {
				duplicateBalanceMode = name
				duplicateBalanceModePath = prev
				break
			}
		}
		if duplicateBalanceMode != "" {
			skipped = append(skipped, entry.Name())
			skippedReasons[entry.Name()] = fmt.Sprintf("duplicate balance_mode %q (already in %q)", duplicateBalanceMode, duplicateBalanceModePath)
			continue
		}
		for name, cfg := range defs {
			modePaths[name] = path
			modeFiles[path] = append(modeFiles[path], name)
			rawModes[name] = cfg
		}
		for name, cfg := range finishDefs {
			finishReasonModePaths[name] = path
			finishReasonModeFiles[path] = append(finishReasonModeFiles[path], name)
			rawFinishReasonModes[name] = cfg
		}
		for name, cfg := range modelDefs {
			modelsModePaths[name] = path
			modelModeFiles[path] = append(modelModeFiles[path], name)
			rawModelsModes[name] = cfg
		}
		for name, cfg := range balanceDefs {
			balanceModePaths[name] = path
			balanceModeFiles[path] = append(balanceModeFiles[path], name)
			rawBalanceModes[name] = cfg
		}
		candidates = append(candidates, candidate{entryName: entry.Name(), path: path, content: content})
	}

	resolvedModes := usageModeRegistry{}
	remainingModes := rawModes
	remainingPaths := modePaths
	remainingModeFiles := modeFiles
	resolvedFinishReasonModes := finishReasonModeRegistry{}
	remainingFinishReasonModes := rawFinishReasonModes
	remainingFinishReasonModePaths := finishReasonModePaths
	remainingFinishReasonModeFiles := finishReasonModeFiles
	resolvedModelsModes := modelsModeRegistry{}
	remainingModelsModes := rawModelsModes
	remainingModelsModePaths := modelsModePaths
	remainingModelsModeFiles := modelModeFiles
	resolvedBalanceModes := balanceModeRegistry{}
	remainingBalanceModes := rawBalanceModes
	remainingBalanceModePaths := balanceModePaths
	remainingBalanceModeFiles := balanceModeFiles
	for {
		var err error
		resolvedModes, err = resolveUsageModeRegistry(remainingPaths, remainingModes)
		if err == nil {
			break
		}
		skippedFile := usageModeFileFromError(err.Error(), remainingModeFiles)
		if skippedFile == "" {
			return LoadResult{}, err
		}
		if _, exists := skippedReasons[filepath.Base(skippedFile)]; exists {
			return LoadResult{}, err
		}
		entryName := filepath.Base(skippedFile)
		skipped = append(skipped, entryName)
		skippedReasons[entryName] = err.Error()
		for _, modeName := range remainingModeFiles[skippedFile] {
			delete(remainingModes, modeName)
			delete(remainingPaths, modeName)
		}
		delete(remainingModeFiles, skippedFile)
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate.path == skippedFile {
				continue
			}
			filtered = append(filtered, candidate)
		}
		candidates = filtered
	}
	for {
		var err error
		resolvedFinishReasonModes, err = resolveFinishReasonModeRegistry(remainingFinishReasonModePaths, remainingFinishReasonModes)
		if err == nil {
			break
		}
		skippedFile := usageModeFileFromError(err.Error(), remainingFinishReasonModeFiles)
		if skippedFile == "" {
			return LoadResult{}, err
		}
		if _, exists := skippedReasons[filepath.Base(skippedFile)]; exists {
			return LoadResult{}, err
		}
		entryName := filepath.Base(skippedFile)
		skipped = append(skipped, entryName)
		skippedReasons[entryName] = err.Error()
		for _, modeName := range remainingFinishReasonModeFiles[skippedFile] {
			delete(remainingFinishReasonModes, modeName)
			delete(remainingFinishReasonModePaths, modeName)
		}
		delete(remainingFinishReasonModeFiles, skippedFile)
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate.path == skippedFile {
				continue
			}
			filtered = append(filtered, candidate)
		}
		candidates = filtered
	}
	for {
		var err error
		resolvedModelsModes, err = resolveModelsModeRegistry(remainingModelsModePaths, remainingModelsModes)
		if err == nil {
			break
		}
		skippedFile := usageModeFileFromError(err.Error(), remainingModelsModeFiles)
		if skippedFile == "" {
			return LoadResult{}, err
		}
		if _, exists := skippedReasons[filepath.Base(skippedFile)]; exists {
			return LoadResult{}, err
		}
		entryName := filepath.Base(skippedFile)
		skipped = append(skipped, entryName)
		skippedReasons[entryName] = err.Error()
		for _, modeName := range remainingModelsModeFiles[skippedFile] {
			delete(remainingModelsModes, modeName)
			delete(remainingModelsModePaths, modeName)
		}
		delete(remainingModelsModeFiles, skippedFile)
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate.path == skippedFile {
				continue
			}
			filtered = append(filtered, candidate)
		}
		candidates = filtered
	}
	for {
		var err error
		resolvedBalanceModes, err = resolveBalanceModeRegistry(remainingBalanceModePaths, remainingBalanceModes)
		if err == nil {
			break
		}
		skippedFile := usageModeFileFromError(err.Error(), remainingBalanceModeFiles)
		if skippedFile == "" {
			return LoadResult{}, err
		}
		if _, exists := skippedReasons[filepath.Base(skippedFile)]; exists {
			return LoadResult{}, err
		}
		entryName := filepath.Base(skippedFile)
		skipped = append(skipped, entryName)
		skippedReasons[entryName] = err.Error()
		for _, modeName := range remainingBalanceModeFiles[skippedFile] {
			delete(remainingBalanceModes, modeName)
			delete(remainingBalanceModePaths, modeName)
		}
		delete(remainingBalanceModeFiles, skippedFile)
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate.path == skippedFile {
				continue
			}
			filtered = append(filtered, candidate)
		}
		candidates = filtered
	}

	for _, candidate := range candidates {
		pf, hasProvider, err := validateAndBuildProviderFile(candidate.path, candidate.content, resolvedModes, resolvedFinishReasonModes, resolvedModelsModes, resolvedBalanceModes)
		if err != nil {
			skipped = append(skipped, candidate.entryName)
			skippedReasons[candidate.entryName] = err.Error()
			continue
		}
		if !hasProvider {
			continue
		}
		if _, exists := next[pf.Name]; exists {
			skipped = append(skipped, candidate.entryName)
			skippedReasons[candidate.entryName] = fmt.Sprintf("duplicate provider name %q", pf.Name)
			continue
		}
		next[pf.Name] = pf
		loaded = append(loaded, pf.Name)
	}

	sort.Strings(loaded)
	sort.Strings(skipped)

	r.mu.Lock()
	r.providers = next
	r.mu.Unlock()

	return LoadResult{LoadedProviders: loaded, SkippedFiles: skipped, SkippedReasons: skippedReasons}, nil
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
