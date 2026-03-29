package dslconfig

import (
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
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
	respRoot, err := responseRootFromBody(meta, cfg, respBody)
	if err != nil {
		return nil, 0, err
	}
	reqRoot := requestRootFromMeta(meta)
	derivedRoot := derivedRootFromMeta(meta)

	switch mode {
	case usageModeCustom:
		usage, cachedTokens, err := extractCustomUsage(reqRoot, respRoot, derivedRoot, cfg)
		if err != nil {
			return nil, 0, err
		}
		return usage, cachedTokens, nil
	case usageModeOpenAI, usageModeAnthropic, usageModeGemini:
		return extractBuiltinUsage(meta, reqRoot, respRoot, derivedRoot, respBody, cfg)
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
		strings.EqualFold(strings.TrimSpace(cfg.Mode), usageModeOpenAI) &&
		strings.EqualFold(strings.TrimSpace(meta.API), "audio.speech") &&
		len(meta.DerivedUsage) > 0 {
		return true
	}
	return false
}

func requestRootFromMeta(meta *dslmeta.Meta) map[string]any {
	if meta == nil || len(meta.RequestBody) == 0 {
		return nil
	}
	contentType := strings.TrimSpace(meta.RequestContentType)
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		return multipartRequestRootFromMeta(meta.RequestBody, contentType)
	}
	var data any
	if err := json.Unmarshal(meta.RequestBody, &data); err != nil {
		return nil
	}
	root, _ := data.(map[string]any)
	return root
}

func derivedRootFromMeta(meta *dslmeta.Meta) map[string]any {
	if meta == nil || len(meta.DerivedUsage) == 0 {
		return nil
	}
	return meta.DerivedUsage
}

func multipartRequestRootFromMeta(body []byte, contentType string) map[string]any {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil
	}
	reader := multipart.NewReader(strings.NewReader(string(body)), boundary)
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		return nil
	}
	defer form.RemoveAll()

	root := make(map[string]any)
	for k, vals := range form.Value {
		switch len(vals) {
		case 0:
			continue
		case 1:
			root[k] = vals[0]
		default:
			items := make([]any, 0, len(vals))
			for _, v := range vals {
				items = append(items, v)
			}
			root[k] = items
		}
	}
	if len(root) == 0 {
		return nil
	}
	return root
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
