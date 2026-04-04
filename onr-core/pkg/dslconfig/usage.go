package dslconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

type UsageExtractConfig struct {
	Mode string

	builtinPreset string

	InputTokensPath      string
	OutputTokensPath     string
	CacheReadTokensPath  string
	CacheWriteTokensPath string

	InputTokensExpr      *UsageExpr
	OutputTokensExpr     *UsageExpr
	CacheReadTokensExpr  *UsageExpr
	CacheWriteTokensExpr *UsageExpr
	TotalTokensExpr      *UsageExpr

	facts            []usageFactConfig
	factGroups       map[usageFactKey][]usageFactConfig
	explicitFactKeys map[usageFactKey]struct{}
}

const (
	usageModeOpenAI    = "openai"
	usageModeAnthropic = "anthropic"
	usageModeGemini    = "gemini"
	usageModeCustom    = "custom"
)

type ProviderUsage struct {
	Defaults UsageExtractConfig
	Matches  []MatchUsage
}

type MatchUsage struct {
	API    string
	Stream *bool

	Extract UsageExtractConfig
}

func (p ProviderUsage) Select(meta *dslmeta.Meta) (UsageExtractConfig, bool) {
	if meta == nil {
		return UsageExtractConfig{}, false
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return UsageExtractConfig{}, false
	}
	cfg := p.Defaults
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		cfg = mergeUsageConfig(cfg, m.Extract)
	}
	if strings.TrimSpace(cfg.Mode) == "" {
		return UsageExtractConfig{}, false
	}
	return cfg, true
}

func (p ProviderUsage) selectMatch(api string, stream bool) (MatchUsage, bool) {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return MatchUsage{}, false
}

func ExtractUsage(meta *dslmeta.Meta, cfg UsageExtractConfig, respBody []byte) (*Usage, int, error) {
	cfg = prepareUsageExtractConfig(cfg)
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil, 0, nil
	}
	respRoot, err := responseRootFromBody(meta, cfg, respBody)
	if err != nil {
		return nil, 0, err
	}
	return extractUsageFromResponseRoot(meta, cfg, respRoot, respBody)
}

func extractUsageFromResponseRoot(meta *dslmeta.Meta, cfg UsageExtractConfig, respRoot map[string]any, respBody []byte) (*Usage, int, error) {
	cfg = prepareUsageExtractConfig(cfg)
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil, 0, nil
	}
	reqRoot := requestRootFromMeta(meta)
	derivedRoot := derivedRootFromMeta(meta)
	return extractUsageFromRoots(meta, cfg, reqRoot, respRoot, derivedRoot, respBody)
}

func extractUsageFromRoots(meta *dslmeta.Meta, cfg UsageExtractConfig, reqRoot, respRoot, derivedRoot map[string]any, respBody []byte) (*Usage, int, error) {
	cfg = compileUsageExtractConfig(meta, cfg)
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case usageModeCustom:
		usage, cachedTokens, err := extractCustomUsage(reqRoot, respRoot, derivedRoot, cfg)
		if err != nil {
			return nil, 0, err
		}
		return usage, cachedTokens, nil
	default:
		return nil, 0, fmt.Errorf("unsupported usage_extract mode %q", cfg.Mode)
	}
}

func responseRootFromBody(meta *dslmeta.Meta, cfg UsageExtractConfig, respBody []byte) (map[string]any, error) {
	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		if usageConfigAllowsNonJSONResponse(meta, cfg) {
			return nil, nil
		}
		return nil, fmt.Errorf("parse response json: %w", err)
	}
	root, ok := data.(map[string]any)
	if !ok {
		return nil, nil
	}
	return root, nil
}

func usageConfigAllowsNonJSONResponse(meta *dslmeta.Meta, cfg UsageExtractConfig) bool {
	for _, fact := range cfg.facts {
		switch strings.ToLower(strings.TrimSpace(fact.Source)) {
		case "request", "derived":
			return true
		}
	}
	if meta != nil &&
		usageBuiltinPreset(cfg) == usageModeOpenAI &&
		strings.EqualFold(strings.TrimSpace(meta.API), "audio.speech") &&
		len(meta.DerivedUsage) > 0 {
		return true
	}
	return false
}

func requestRootFromMeta(meta *dslmeta.Meta) map[string]any {
	if meta == nil {
		return nil
	}
	return meta.RequestRoot()
}

func prepareUsageExtractConfig(cfg UsageExtractConfig) UsageExtractConfig {
	if len(cfg.facts) == 0 {
		cfg.factGroups = nil
		cfg.explicitFactKeys = nil
		return cfg
	}
	if cfg.factGroups == nil {
		cfg.factGroups = groupUsageFactConfigs(cfg.facts)
	}
	if cfg.explicitFactKeys == nil {
		cfg.explicitFactKeys = usageFactExplicitKeys(cfg.facts)
	}
	return cfg
}

func compileUsageExtractConfig(meta *dslmeta.Meta, cfg UsageExtractConfig) UsageExtractConfig {
	cfg = prepareUsageExtractConfig(cfg)
	mode := normalizeUsageMode(cfg.Mode)
	builtinMode := usageBuiltinPreset(cfg)
	switch {
	case mode == "":
		return cfg
	case mode == usageModeCustom && builtinMode == "":
		compiled := UsageExtractConfig{
			Mode:            usageModeCustom,
			TotalTokensExpr: cfg.TotalTokensExpr,
		}
		compiled.facts = append(compiled.facts, legacyUsageFactConfigs(cfg, cfg.explicitFactKeys)...)
		compiled.facts = append(compiled.facts, cloneUsageFactConfigs(cfg.facts)...)
		return prepareUsageExtractConfig(compiled)
	case builtinMode != "":
		overrideKeys := mergeUsageFactKeySets(cfg.explicitFactKeys, legacyUsageFactKeys(cfg))
		compiled := UsageExtractConfig{Mode: usageModeCustom}
		baseSet := builtinUsageFactSet(builtinMode)
		for key, group := range baseSet.factGroups {
			if _, ok := overrideKeys[key]; ok {
				continue
			}
			compiled.facts = append(compiled.facts, cloneUsageFactConfigs(group)...)
		}
		compiled.facts = append(compiled.facts, legacyUsageFactConfigs(cfg, cfg.explicitFactKeys)...)
		compiled.facts = append(compiled.facts, cloneUsageFactConfigs(cfg.facts)...)
		compiled.facts = append(compiled.facts, builtinSupplementalUsageFactConfigs(meta, builtinMode, overrideKeys)...)
		if totalExpr := builtinTotalTokensExpr(builtinMode); totalExpr != nil {
			compiled.TotalTokensExpr = totalExpr
		}
		if cfg.TotalTokensExpr != nil {
			compiled.TotalTokensExpr = cfg.TotalTokensExpr
		}
		return prepareUsageExtractConfig(compiled)
	default:
		return cfg
	}
}

func derivedRootFromMeta(meta *dslmeta.Meta) map[string]any {
	if meta == nil || len(meta.DerivedUsage) == 0 {
		return nil
	}
	return meta.DerivedUsage
}

func evalUsageField(root map[string]any, expr *UsageExpr, fallbackPath string) int {
	if expr != nil {
		return expr.Eval(root)
	}
	return jsonutil.GetIntByPath(root, strings.TrimSpace(fallbackPath))
}

func mergeUsageConfig(base UsageExtractConfig, override UsageExtractConfig) UsageExtractConfig {
	out := base
	out.factGroups = nil
	out.explicitFactKeys = nil
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
		out.builtinPreset = override.builtinPreset
	}
	if strings.TrimSpace(override.InputTokensPath) != "" {
		out.InputTokensPath = override.InputTokensPath
	}
	if strings.TrimSpace(override.OutputTokensPath) != "" {
		out.OutputTokensPath = override.OutputTokensPath
	}
	if strings.TrimSpace(override.CacheReadTokensPath) != "" {
		out.CacheReadTokensPath = override.CacheReadTokensPath
	}
	if strings.TrimSpace(override.CacheWriteTokensPath) != "" {
		out.CacheWriteTokensPath = override.CacheWriteTokensPath
	}
	if override.InputTokensExpr != nil {
		out.InputTokensExpr = override.InputTokensExpr
	}
	if override.OutputTokensExpr != nil {
		out.OutputTokensExpr = override.OutputTokensExpr
	}
	if override.CacheReadTokensExpr != nil {
		out.CacheReadTokensExpr = override.CacheReadTokensExpr
	}
	if override.CacheWriteTokensExpr != nil {
		out.CacheWriteTokensExpr = override.CacheWriteTokensExpr
	}
	if override.TotalTokensExpr != nil {
		out.TotalTokensExpr = override.TotalTokensExpr
	}
	if len(override.facts) > 0 {
		combined := make([]usageFactConfig, 0, len(base.facts)+len(override.facts))
		combined = append(combined, base.facts...)
		combined = append(combined, override.facts...)
		out.facts = combined
	}
	return prepareUsageExtractConfig(out)
}

// UsesDerivedUsagePath reports whether the config explicitly references a
// derived-source usage_fact at the given JSON path.
func UsesDerivedUsagePath(meta *dslmeta.Meta, cfg UsageExtractConfig, path string) bool {
	cfg = compileUsageExtractConfig(meta, cfg)
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	for _, fact := range cfg.facts {
		if !strings.EqualFold(strings.TrimSpace(fact.Source), "derived") {
			continue
		}
		if strings.TrimSpace(fact.Path) == path {
			return true
		}
	}
	return false
}

func builtinUsagePresetName(mode string) string {
	switch normalizeUsageMode(mode) {
	case usageModeOpenAI:
		return usageModeOpenAI
	case usageModeAnthropic:
		return usageModeAnthropic
	case usageModeGemini:
		return usageModeGemini
	default:
		return ""
	}
}

func usageBuiltinPreset(cfg UsageExtractConfig) string {
	if preset := builtinUsagePresetName(cfg.builtinPreset); preset != "" {
		return preset
	}
	return builtinUsagePresetName(cfg.Mode)
}
