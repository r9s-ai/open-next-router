package proxy

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
)

// SetPricingResolver requires a non-nil Client receiver.
func (c *Client) SetPricingResolver(r *pricing.Resolver) {
	c.pricingMu.Lock()
	c.pricing = r
	c.pricingMu.Unlock()
}

// SetPricingEnabled requires a non-nil Client receiver.
func (c *Client) SetPricingEnabled(enabled bool) {
	c.pricingMu.Lock()
	c.pricingEnabled = enabled
	c.pricingMu.Unlock()
}

// getPricingResolver requires a non-nil Client receiver and may return nil when unset.
func (c *Client) getPricingResolver() *pricing.Resolver {
	c.pricingMu.RLock()
	defer c.pricingMu.RUnlock()
	return c.pricing
}

// isPricingEnabled requires a non-nil Client receiver.
func (c *Client) isPricingEnabled() bool {
	c.pricingMu.RLock()
	defer c.pricingMu.RUnlock()
	return c.pricingEnabled
}

func (c *Client) computeCost(
	meta *dslmeta.Meta,
	provider string,
	keyName string,
	usage map[string]any,
) map[string]any {
	if usage == nil {
		return nil
	}
	if !c.isPricingEnabled() {
		return nil
	}
	resolver := c.getPricingResolver()
	if resolver == nil {
		return nil
	}
	model := strings.TrimSpace(meta.DSLModelMapped)
	if model == "" {
		model = strings.TrimSpace(meta.OriginModelName)
	}
	out, ok := resolver.Compute(provider, keyName, model, usage)
	if !ok || out == nil {
		return nil
	}
	return map[string]any{
		"cost_total":            out.TotalCost,
		"cost_input":            out.InputCost,
		"cost_output":           out.OutputCost,
		"cost_cache_read":       out.CacheReadCost,
		"cost_cache_write":      out.CacheWriteCost,
		"billable_input_tokens": out.BillableInputTokens,
		"cost_multiplier":       out.Multiplier,
		"cost_model":            out.Model,
		"cost_channel":          out.Channel,
		"cost_unit":             out.Unit,
		"cost_rate_unit":        out.RateUnit,
		"price_input":           out.InputRate,
		"price_output":          out.OutputRate,
		"price_cache_read":      out.CacheReadRate,
		"price_cache_write":     out.CacheWriteRate,
	}
}
