package dslconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const globalDSLConfigFilename = "onr.conf"

func globalConfigPathForProvidersDir(providersDir string) string {
	dir := strings.TrimSpace(providersDir)
	if dir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Clean(dir)), globalDSLConfigFilename)
}

func globalConfigPathForProviderFile(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	return globalConfigPathForProvidersDir(filepath.Dir(p))
}

func globalConfigPathForMergedProvidersFile(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if filepath.Base(filepath.Clean(p)) == globalDSLConfigFilename {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Clean(p)), globalDSLConfigFilename)
}

func loadGlobalUsageModesFromFile(path string) (usageModeRegistry, map[string]string, string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil, "", nil
	}
	contentBytes, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", nil
		}
		return nil, nil, "", fmt.Errorf("read global config %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(contentBytes))
	if err != nil {
		return nil, nil, "", err
	}
	rawModes, err := parseGlobalUsageModes(p, content)
	if err != nil {
		return nil, nil, "", err
	}
	paths := make(map[string]string, len(rawModes))
	for name := range rawModes {
		paths[name] = p
	}
	return rawModes, paths, content, nil
}

func loadGlobalFinishReasonModesFromFile(path string) (finishReasonModeRegistry, map[string]string, string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil, "", nil
	}
	contentBytes, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", nil
		}
		return nil, nil, "", fmt.Errorf("read global config %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(contentBytes))
	if err != nil {
		return nil, nil, "", err
	}
	rawModes, err := parseGlobalFinishReasonModes(p, content)
	if err != nil {
		return nil, nil, "", err
	}
	paths := make(map[string]string, len(rawModes))
	for name := range rawModes {
		paths[name] = p
	}
	return rawModes, paths, content, nil
}

func loadGlobalModelsModesFromFile(path string) (modelsModeRegistry, map[string]string, string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil, "", nil
	}
	contentBytes, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", nil
		}
		return nil, nil, "", fmt.Errorf("read global config %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(contentBytes))
	if err != nil {
		return nil, nil, "", err
	}
	rawModes, err := parseGlobalModelsModes(p, content)
	if err != nil {
		return nil, nil, "", err
	}
	paths := make(map[string]string, len(rawModes))
	for name := range rawModes {
		paths[name] = p
	}
	return rawModes, paths, content, nil
}

func loadGlobalBalanceModesFromFile(path string) (balanceModeRegistry, map[string]string, string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil, "", nil
	}
	contentBytes, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", nil
		}
		return nil, nil, "", fmt.Errorf("read global config %q: %w", p, err)
	}
	content, err := preprocessIncludes(p, string(contentBytes))
	if err != nil {
		return nil, nil, "", err
	}
	rawModes, err := parseGlobalBalanceModes(p, content)
	if err != nil {
		return nil, nil, "", err
	}
	paths := make(map[string]string, len(rawModes))
	for name := range rawModes {
		paths[name] = p
	}
	return rawModes, paths, content, nil
}
