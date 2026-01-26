package dslconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
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
	case "openai":
		inputTokens = firstInt(
			getIntByPath(root, "$.usage.prompt_tokens"),
			getIntByPath(root, "$.usage.input_tokens"),
		)
		outputTokens = firstInt(
			getIntByPath(root, "$.usage.completion_tokens"),
			getIntByPath(root, "$.usage.output_tokens"),
		)
		cachedTokens = firstInt(
			getIntByPath(root, "$.usage.prompt_tokens_details.cached_tokens"),
			getIntByPath(root, "$.usage.input_tokens_details.cached_tokens"),
			getIntByPath(root, "$.usage.cached_tokens"),
		)
	case "anthropic":
		inputTokens = getIntByPath(root, "$.usage.input_tokens")
		outputTokens = getIntByPath(root, "$.usage.output_tokens")
		cachedTokens = getIntByPath(root, "$.usage.cache_read_input_tokens")
		cacheWriteTokens = getIntByPath(root, "$.usage.cache_creation_input_tokens")
	case "custom":
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
	return getIntByPath(root, strings.TrimSpace(fallbackPath))
}

func firstInt(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

// getIntByPath implements a restricted JSONPath subset:
// - $.a.b.c
// - $.items[0].x
// - $.items[*].x (sum of numeric values)
func getIntByPath(root map[string]any, path string) int {
	p := strings.TrimSpace(path)
	if p == "" {
		return 0
	}
	if !strings.HasPrefix(p, "$.") {
		return 0
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	return getIntByParts(root, parts)
}

func getIntByParts(cur any, parts []string) int {
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return 0
		}
		name, idx, hasIdx, isStar := splitIndex(part)
		if name != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return 0
			}
			cur, ok = m[name]
			if !ok {
				return 0
			}
		}
		if hasIdx {
			arr, ok := cur.([]any)
			if !ok {
				return 0
			}
			if isStar {
				sum := 0
				rest := parts[i+1:]
				if len(rest) == 0 {
					for _, item := range arr {
						sum += coerceInt(item)
					}
					return sum
				}
				for _, item := range arr {
					sum += getIntByParts(item, rest)
				}
				return sum
			}
			if idx < 0 || idx >= len(arr) {
				return 0
			}
			cur = arr[idx]
		}
	}
	return coerceInt(cur)
}

func splitIndex(s string) (name string, idx int, hasIdx bool, isStar bool) {
	open := strings.IndexByte(s, '[')
	if open < 0 {
		return s, 0, false, false
	}
	close := strings.IndexByte(s, ']')
	if close < 0 || close < open {
		return s, 0, false, false
	}
	name = s[:open]
	inner := strings.TrimSpace(s[open+1 : close])
	if inner == "*" {
		return name, 0, true, true
	}
	n, err := strconv.Atoi(inner)
	if err != nil {
		return name, 0, false, false
	}
	return name, n, true, false
}

func coerceInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i
		}
	case []any:
		sum := 0
		for _, item := range t {
			sum += coerceInt(item)
		}
		return sum
	}
	return 0
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
