package dslconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

type FinishReasonExtractConfig struct {
	Mode             string
	FinishReasonPath string
	paths            []finishReasonPathConfig
}

type finishReasonPathConfig struct {
	Path          string
	Fallback      bool
	Event         string
	EventOptional bool
}

type ProviderFinishReason struct {
	Defaults FinishReasonExtractConfig
	Matches  []MatchFinishReason
}

type MatchFinishReason struct {
	API    string
	Stream *bool

	Extract FinishReasonExtractConfig
}

func (p ProviderFinishReason) Select(meta *dslmeta.Meta) (FinishReasonExtractConfig, bool) {
	if meta == nil {
		return FinishReasonExtractConfig{}, false
	}
	// match overrides
	for _, m := range p.Matches {
		if strings.TrimSpace(m.API) != "" && strings.TrimSpace(m.API) != strings.TrimSpace(meta.API) {
			continue
		}
		if m.Stream != nil && *m.Stream != meta.IsStream {
			continue
		}
		cfg := mergeFinishReasonConfig(p.Defaults, m.Extract)
		if strings.TrimSpace(cfg.Mode) == "" && !cfg.hasFinishReasonPath() {
			return FinishReasonExtractConfig{}, false
		}
		return cfg, true
	}
	// defaults
	if strings.TrimSpace(p.Defaults.Mode) == "" && !p.Defaults.hasFinishReasonPath() {
		return FinishReasonExtractConfig{}, false
	}
	return p.Defaults, true
}

func mergeFinishReasonConfig(base, override FinishReasonExtractConfig) FinishReasonExtractConfig {
	out := base
	if len(base.paths) > 0 {
		out.paths = append([]finishReasonPathConfig(nil), base.paths...)
	}
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if len(override.paths) > 0 {
		out.paths = append([]finishReasonPathConfig(nil), override.paths...)
		out.FinishReasonPath = override.paths[len(override.paths)-1].Path
	} else if strings.TrimSpace(override.FinishReasonPath) != "" {
		out.paths = nil
		out.FinishReasonPath = override.FinishReasonPath
	}
	return out
}

func (cfg *FinishReasonExtractConfig) addFinishReasonPath(path string, fallback bool) {
	cfg.addFinishReasonPathRule(path, fallback, "", false)
}

func (cfg *FinishReasonExtractConfig) addFinishReasonPathRule(path string, fallback bool, event string, eventOptional bool) {
	if cfg == nil {
		return
	}
	cfg.FinishReasonPath = path
	cfg.paths = append(cfg.paths, finishReasonPathConfig{
		Path:          path,
		Fallback:      fallback,
		Event:         strings.TrimSpace(event),
		EventOptional: eventOptional,
	})
}

func (cfg FinishReasonExtractConfig) hasFinishReasonPath() bool {
	return len(cfg.finishReasonPathConfigs()) > 0
}

func (cfg FinishReasonExtractConfig) finishReasonPathConfigs() []finishReasonPathConfig {
	if len(cfg.paths) > 0 {
		return append([]finishReasonPathConfig(nil), cfg.paths...)
	}
	path := strings.TrimSpace(cfg.FinishReasonPath)
	if path == "" {
		return nil
	}
	return []finishReasonPathConfig{{Path: path}}
}

// ExtractFinishReason extracts finish_reason from a JSON response (best-effort).
// Returns empty string when it cannot be extracted.
func ExtractFinishReason(meta *dslmeta.Meta, cfg FinishReasonExtractConfig, respBody []byte) (string, error) {
	var obj any
	if err := json.Unmarshal(respBody, &obj); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return "", nil
	}
	return extractFinishReasonFromRoot(meta, cfg, root)
}

func extractFinishReasonFromRoot(meta *dslmeta.Meta, cfg FinishReasonExtractConfig, root map[string]any) (string, error) {
	return extractFinishReasonFromRootWithEvent(meta, cfg, "", root)
}

func extractFinishReasonFromRootWithEvent(meta *dslmeta.Meta, cfg FinishReasonExtractConfig, event string, root map[string]any) (string, error) {
	_ = meta
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	hasPaths := cfg.hasFinishReasonPath()

	if mode == "" && !hasPaths {
		return "", nil
	}

	if mode == "custom" {
		if !hasPaths {
			return "", nil
		}
		return extractFinishReasonByConfiguredPaths(root, cfg, event), nil
	}
	return "", fmt.Errorf("unsupported finish_reason_extract mode %q", cfg.Mode)
}

func extractFinishReasonByConfiguredPaths(root map[string]any, cfg FinishReasonExtractConfig, event string) string {
	paths := cfg.finishReasonPathConfigs()
	if len(paths) == 0 {
		return ""
	}
	var fallback []finishReasonPathConfig
	for _, rule := range paths {
		if !finishReasonRuleMatchesEvent(rule, event) {
			continue
		}
		if rule.Fallback {
			fallback = append(fallback, rule)
			continue
		}
		if v := strings.TrimSpace(jsonutil.GetStringByPath(root, rule.Path)); v != "" {
			return v
		}
	}
	for _, rule := range fallback {
		if v := strings.TrimSpace(jsonutil.GetStringByPath(root, rule.Path)); v != "" {
			return v
		}
	}
	return ""
}

func finishReasonRuleMatchesEvent(rule finishReasonPathConfig, event string) bool {
	expectedEvent := strings.TrimSpace(rule.Event)
	if expectedEvent == "" {
		return true
	}
	currentEvent := strings.TrimSpace(event)
	switch {
	case currentEvent == "" && rule.EventOptional:
		return true
	case !strings.EqualFold(expectedEvent, currentEvent):
		return false
	default:
		return true
	}
}
