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

	facts []usageFactConfig
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
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil, 0, nil
	}
	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, 0, fmt.Errorf("parse response json: %w", err)
	}
	root, ok := data.(map[string]any)
	if !ok {
		return nil, 0, nil
	}

	switch mode {
	case usageModeCustom:
		usage, cachedTokens, err := extractCustomUsage(root, cfg)
		if err != nil {
			return nil, 0, err
		}
		return usage, cachedTokens, nil
	case usageModeOpenAI, usageModeAnthropic, usageModeGemini:
		return extractBuiltinUsage(root, cfg)
	default:
		return nil, 0, fmt.Errorf("unsupported usage_extract mode %q", cfg.Mode)
	}
}

func evalUsageField(root map[string]any, expr *UsageExpr, fallbackPath string) int {
	if expr != nil {
		return expr.Eval(root)
	}
	return jsonutil.GetIntByPath(root, strings.TrimSpace(fallbackPath))
}

func mergeUsageConfig(base UsageExtractConfig, override UsageExtractConfig) UsageExtractConfig {
	out := base
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
		out.facts = append(out.facts, override.facts...)
	}
	return out
}
