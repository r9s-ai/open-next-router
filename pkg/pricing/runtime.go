package pricing

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	rateInput      = "input"
	rateOutput     = "output"
	rateCacheRead  = "cache_read"
	rateCacheWrite = "cache_write"
)

type OverridesFile struct {
	Version   string                   `yaml:"version"`
	Providers map[string]ScopeOverride `yaml:"providers"`
	Channels  map[string]ScopeOverride `yaml:"channels"`
}

type ScopeOverride struct {
	Multiplier *float64                 `yaml:"multiplier"`
	Models     map[string]ModelOverride `yaml:"models"`
}

type ModelOverride struct {
	Cost map[string]float64 `yaml:"cost"`
}

type Resolver struct {
	unit string

	base map[string]map[string]map[string]float64

	providerOverrides map[string]ScopeOverride
	channelOverrides  map[string]map[string]ScopeOverride
}

type CostResult struct {
	Provider string
	Key      string
	Channel  string
	Model    string

	Unit     string
	RateUnit string

	Multiplier float64

	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheWriteTokens    int
	BillableInputTokens int

	InputRate      float64
	OutputRate     float64
	CacheReadRate  float64
	CacheWriteRate float64

	InputCost      float64
	OutputCost     float64
	CacheReadCost  float64
	CacheWriteCost float64
	TotalCost      float64
}

func LoadResolver(pricePath, overridesPath string) (*Resolver, error) {
	pricePath = strings.TrimSpace(pricePath)
	if pricePath == "" {
		return nil, nil
	}
	priceDoc, err := loadPriceFile(pricePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(priceDoc.Entries) == 0 {
		return nil, nil
	}
	unit := strings.TrimSpace(priceDoc.Unit)
	if unit == "" {
		unit = defaultPriceUnit
	}

	base := map[string]map[string]map[string]float64{}
	for _, entry := range priceDoc.Entries {
		provider := strings.ToLower(strings.TrimSpace(entry.Provider))
		model := strings.TrimSpace(entry.Model)
		if provider == "" || model == "" || len(entry.Cost) == 0 {
			continue
		}
		if _, ok := base[provider]; !ok {
			base[provider] = map[string]map[string]float64{}
		}
		base[provider][model] = cloneFloatMap(entry.Cost)
	}
	if len(base) == 0 {
		return nil, nil
	}

	overrides := OverridesFile{}
	if strings.TrimSpace(overridesPath) != "" {
		ov, oerr := loadOverridesFile(overridesPath)
		if oerr != nil && !errors.Is(oerr, os.ErrNotExist) {
			return nil, oerr
		}
		if ov != nil {
			overrides = *ov
		}
	}

	out := &Resolver{
		unit:              unit,
		base:              base,
		providerOverrides: normalizeProviderOverrides(overrides.Providers),
		channelOverrides:  normalizeChannelOverrides(overrides.Channels),
	}
	return out, nil
}

func (r *Resolver) Compute(provider, key, model string, usage map[string]any) (CostResult, bool) {
	if r == nil || usage == nil {
		return CostResult{}, false
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	key = strings.TrimSpace(key)
	if provider == "" || model == "" {
		return CostResult{}, false
	}
	models, ok := r.base[provider]
	if !ok {
		return CostResult{}, false
	}
	baseRates, ok := models[model]
	if !ok {
		return CostResult{}, false
	}
	effectiveRates := cloneFloatMap(baseRates)
	multiplier := 1.0

	if ov, ok := r.providerOverrides[provider]; ok {
		applyModelOverride(effectiveRates, model, ov)
		if ov.Multiplier != nil {
			multiplier *= *ov.Multiplier
		}
	}
	if channels, ok := r.channelOverrides[provider]; ok && key != "" {
		if ov, ok := channels[key]; ok {
			applyModelOverride(effectiveRates, model, ov)
			if ov.Multiplier != nil {
				multiplier *= *ov.Multiplier
			}
		}
	}
	for k, v := range effectiveRates {
		effectiveRates[k] = v * multiplier
	}

	inputRate := effectiveRates[rateInput]
	outputRate := effectiveRates[rateOutput]
	cacheReadRate := effectiveRates[rateCacheRead]
	cacheWriteRate := effectiveRates[rateCacheWrite]
	if cacheReadRate == 0 {
		cacheReadRate = inputRate
	}
	if cacheWriteRate == 0 {
		cacheWriteRate = inputRate
	}
	if inputRate == 0 && outputRate == 0 && cacheReadRate == 0 && cacheWriteRate == 0 {
		return CostResult{}, false
	}

	inputTokens := intFromAny(usage["input_tokens"])
	outputTokens := intFromAny(usage["output_tokens"])
	cacheReadTokens := intFromAny(usage["cache_read_tokens"])
	cacheWriteTokens := intFromAny(usage["cache_write_tokens"])
	billableInput := inputTokens - cacheReadTokens - cacheWriteTokens
	if billableInput < 0 {
		billableInput = 0
	}

	inputCost := usdByRatePerMillion(billableInput, inputRate)
	outputCost := usdByRatePerMillion(outputTokens, outputRate)
	cacheReadCost := usdByRatePerMillion(cacheReadTokens, cacheReadRate)
	cacheWriteCost := usdByRatePerMillion(cacheWriteTokens, cacheWriteRate)
	total := inputCost + outputCost + cacheReadCost + cacheWriteCost

	channel := provider
	if key != "" {
		channel = provider + "/" + key
	}

	return CostResult{
		Provider: provider,
		Key:      key,
		Channel:  channel,
		Model:    model,
		Unit:     "usd",
		RateUnit: r.unit,

		Multiplier: multiplier,

		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		CacheReadTokens:     cacheReadTokens,
		CacheWriteTokens:    cacheWriteTokens,
		BillableInputTokens: billableInput,

		InputRate:      inputRate,
		OutputRate:     outputRate,
		CacheReadRate:  cacheReadRate,
		CacheWriteRate: cacheWriteRate,

		InputCost:      inputCost,
		OutputCost:     outputCost,
		CacheReadCost:  cacheReadCost,
		CacheWriteCost: cacheWriteCost,
		TotalCost:      total,
	}, true
}

func loadPriceFile(path string) (*PriceFile, error) {
	// #nosec G304 -- path is provided by trusted config/env.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out PriceFile
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse price file %q failed: %w", path, err)
	}
	return &out, nil
}

func loadOverridesFile(path string) (*OverridesFile, error) {
	// #nosec G304 -- path is provided by trusted config/env.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out OverridesFile
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse price overrides file %q failed: %w", path, err)
	}
	return &out, nil
}

func normalizeProviderOverrides(in map[string]ScopeOverride) map[string]ScopeOverride {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]ScopeOverride, len(in))
	for provider, cfg := range in {
		p := strings.ToLower(strings.TrimSpace(provider))
		if p == "" {
			continue
		}
		out[p] = normalizeScopeOverride(cfg)
	}
	return out
}

func normalizeChannelOverrides(in map[string]ScopeOverride) map[string]map[string]ScopeOverride {
	if len(in) == 0 {
		return nil
	}
	out := map[string]map[string]ScopeOverride{}
	for channel, cfg := range in {
		raw := strings.TrimSpace(channel)
		parts := strings.SplitN(raw, "/", 2)
		if len(parts) != 2 {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(parts[0]))
		key := strings.TrimSpace(parts[1])
		if provider == "" || key == "" {
			continue
		}
		if _, ok := out[provider]; !ok {
			out[provider] = map[string]ScopeOverride{}
		}
		out[provider][key] = normalizeScopeOverride(cfg)
	}
	return out
}

func normalizeScopeOverride(in ScopeOverride) ScopeOverride {
	models := map[string]ModelOverride{}
	for model, ov := range in.Models {
		m := strings.TrimSpace(model)
		if m == "" {
			continue
		}
		models[m] = ModelOverride{Cost: cloneFloatMap(ov.Cost)}
	}
	return ScopeOverride{
		Multiplier: in.Multiplier,
		Models:     models,
	}
}

func applyModelOverride(rates map[string]float64, model string, scope ScopeOverride) {
	if rates == nil || len(scope.Models) == 0 {
		return
	}
	ov, ok := scope.Models[strings.TrimSpace(model)]
	if !ok {
		return
	}
	for k, v := range ov.Cost {
		rates[strings.TrimSpace(k)] = v
	}
}

func usdByRatePerMillion(tokens int, rate float64) float64 {
	if tokens <= 0 || rate == 0 {
		return 0
	}
	return (float64(tokens) * rate) / 1_000_000
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case int32:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	default:
		return 0
	}
}
