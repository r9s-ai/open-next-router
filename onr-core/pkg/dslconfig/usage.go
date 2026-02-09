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

	var inputTokens, outputTokens, cachedTokens, cacheWriteTokens int
	var totalTokens *int
	switch mode {
	case usageModeOpenAI:
		inputTokens = jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usage.prompt_tokens"),
			jsonutil.GetIntByPath(root, "$.usage.input_tokens"),
		)
		outputTokens = jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usage.completion_tokens"),
			jsonutil.GetIntByPath(root, "$.usage.output_tokens"),
		)
		cachedTokens = jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usage.prompt_tokens_details.cached_tokens"),
			jsonutil.GetIntByPath(root, "$.usage.input_tokens_details.cached_tokens"),
			jsonutil.GetIntByPath(root, "$.usage.cached_tokens"),
		)
	case usageModeAnthropic:
		inputTokens = jsonutil.GetIntByPath(root, "$.usage.input_tokens")
		outputTokens = jsonutil.GetIntByPath(root, "$.usage.output_tokens")
		cachedTokens = jsonutil.GetIntByPath(root, "$.usage.cache_read_input_tokens")
		cacheWriteTokens = jsonutil.GetIntByPath(root, "$.usage.cache_creation_input_tokens")
	case usageModeGemini:
		// Gemini native usage fields (new-api alignment):
		// - usageMetadata.promptTokenCount
		// - usageMetadata.candidatesTokenCount
		// - usageMetadata.thoughtsTokenCount (reasoning)
		// - usageMetadata.totalTokenCount
		//
		// CompletionTokens = candidatesTokenCount + thoughtsTokenCount
		// TotalTokens uses totalTokenCount if present.
		inputTokens = jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usageMetadata.promptTokenCount"),
			jsonutil.GetIntByPath(root, "$.usage_metadata.prompt_token_count"),
		)
		candidatesTokens := jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usageMetadata.candidatesTokenCount"),
			jsonutil.GetIntByPath(root, "$.usage_metadata.candidates_token_count"),
		)
		thoughtsTokens := jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usageMetadata.thoughtsTokenCount"),
			jsonutil.GetIntByPath(root, "$.usage_metadata.thoughts_token_count"),
		)
		outputTokens = candidatesTokens + thoughtsTokens
		if v := jsonutil.FirstInt(
			jsonutil.GetIntByPath(root, "$.usageMetadata.totalTokenCount"),
			jsonutil.GetIntByPath(root, "$.usage_metadata.total_token_count"),
		); v != 0 {
			totalTokens = &v
		}
	case usageModeCustom:
		inputTokens = evalUsageField(root, cfg.InputTokensExpr, cfg.InputTokensPath)
		outputTokens = evalUsageField(root, cfg.OutputTokensExpr, cfg.OutputTokensPath)
		cachedTokens = evalUsageField(root, cfg.CacheReadTokensExpr, cfg.CacheReadTokensPath)
		cacheWriteTokens = evalUsageField(root, cfg.CacheWriteTokensExpr, cfg.CacheWriteTokensPath)
		if cfg.TotalTokensExpr != nil {
			v := cfg.TotalTokensExpr.Eval(root)
			totalTokens = &v
		}
	default:
		return nil, 0, fmt.Errorf("unsupported usage_extract mode %q", cfg.Mode)
	}

	tt := inputTokens + outputTokens
	if totalTokens != nil {
		tt = *totalTokens
	}
	usage := &Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      tt,
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
	}
	if cachedTokens > 0 || cacheWriteTokens > 0 {
		usage.InputTokenDetails = &ResponseTokenDetails{
			CachedTokens:     cachedTokens,
			CacheWriteTokens: cacheWriteTokens,
		}
	}
	return usage, cachedTokens, nil
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
	return out
}
