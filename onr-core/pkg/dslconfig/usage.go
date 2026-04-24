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
	usageModeCustom = "custom"
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

// Select requires a non-nil meta and a valid ProviderUsage receiver.
// It returns a request-scoped copy assembled from defaults and the matched override.
// Callers must treat the shared provider config as read-only across requests.
func (p *ProviderUsage) Select(meta *dslmeta.Meta) (*UsageExtractConfig, bool) {
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return nil, false
	}
	cfg := p.Defaults
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		cfg = mergeUsageConfig(cfg, m.Extract)
	}
	if cfg.Mode == "" {
		return nil, false
	}
	return &cfg, true
}

func (p *ProviderUsage) selectMatch(api string, stream bool) (MatchUsage, bool) {
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

// ExtractUsage requires a selected UsageExtractConfig and non-nil meta; cfg is never nil in the current call paths.
// It returns the extracted usage, the configured sample rate, and any extraction error.
func ExtractUsage(meta *dslmeta.Meta, cfg *UsageExtractConfig, respBody []byte) (*Usage, int, error) {
	compiledCfg := prepareUsageExtractConfig(*cfg)
	mode := normalizeUsageMode(compiledCfg.Mode)
	if mode == "" {
		return nil, 0, nil
	}
	respRoot, err := responseRootFromBody(meta, compiledCfg, respBody)
	if err != nil {
		return nil, 0, err
	}
	return extractUsageFromResponseRoot(meta, compiledCfg, respRoot, respBody)
}

func extractUsageFromResponseRoot(meta *dslmeta.Meta, cfg UsageExtractConfig, respRoot map[string]any, respBody []byte) (*Usage, int, error) {
	cfg = prepareUsageExtractConfig(cfg)
	mode := normalizeUsageMode(cfg.Mode)
	if mode == "" {
		return nil, 0, nil
	}
	reqRoot := requestRootFromMeta(meta)
	derivedRoot := derivedRootFromMeta(meta)
	return extractUsageFromRootsWithEvent(meta, "", cfg, reqRoot, respRoot, derivedRoot, respBody)
}

func extractUsageFromRoots(meta *dslmeta.Meta, cfg UsageExtractConfig, reqRoot, respRoot, derivedRoot map[string]any, respBody []byte) (*Usage, int, error) {
	return extractUsageFromRootsWithEvent(meta, "", cfg, reqRoot, respRoot, derivedRoot, respBody)
}

func extractUsageFromRootsWithEvent(meta *dslmeta.Meta, event string, cfg UsageExtractConfig, reqRoot, respRoot, derivedRoot map[string]any, respBody []byte) (*Usage, int, error) {
	cfg = compileUsageExtractConfig(meta, cfg)
	mode := normalizeUsageMode(cfg.Mode)
	switch mode {
	case usageModeCustom:
		usage, cachedTokens, err := extractCustomUsageWithEvent(event, reqRoot, respRoot, derivedRoot, cfg)
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
		switch strings.ToLower(fact.Source) {
		case "request", "derived":
			return true
		}
	}
	_ = meta
	return false
}

// requestRootFromMeta requires a non-nil meta.
func requestRootFromMeta(meta *dslmeta.Meta) map[string]any {
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
	_ = meta
	cfg = prepareUsageExtractConfig(cfg)
	mode := normalizeUsageMode(cfg.Mode)
	switch {
	case mode == "":
		return cfg
	case mode == usageModeCustom:
		compiled := UsageExtractConfig{
			Mode:            usageModeCustom,
			TotalTokensExpr: cfg.TotalTokensExpr,
		}
		compiled.facts = append(compiled.facts, legacyUsageFactConfigs(cfg, cfg.explicitFactKeys)...)
		compiled.facts = append(compiled.facts, cloneUsageFactConfigs(cfg.facts)...)
		return prepareUsageExtractConfig(compiled)
	default:
		return cfg
	}
}

// derivedRootFromMeta requires a non-nil meta.
func derivedRootFromMeta(meta *dslmeta.Meta) map[string]any {
	if len(meta.DerivedUsage) == 0 {
		return nil
	}
	return meta.DerivedUsage
}

func evalUsageField(root map[string]any, expr *UsageExpr, fallbackPath string) int {
	if expr != nil {
		return expr.Eval(root)
	}
	return jsonutil.GetIntByPath(root, fallbackPath)
}

func mergeUsageConfig(base UsageExtractConfig, override UsageExtractConfig) UsageExtractConfig {
	out := base
	out.factGroups = nil
	out.explicitFactKeys = nil
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
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
// UsesDerivedUsagePath requires a selected UsageExtractConfig.
func UsesDerivedUsagePath(meta *dslmeta.Meta, cfg *UsageExtractConfig, path string) bool {
	compiledCfg := compileUsageExtractConfig(meta, *cfg)
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	for _, fact := range compiledCfg.facts {
		if !strings.EqualFold(fact.Source, "derived") {
			continue
		}
		if fact.Path == path {
			return true
		}
	}
	return false
}
