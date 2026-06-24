package proxy

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	onraudio "github.com/r9s-ai/open-next-router/onr-core/pkg/providerusage/audio"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

func estimateNonStreamUsage(
	estCfg *usageestimate.Config,
	pf dslconfig.ProviderFile,
	meta *dslmeta.Meta,
	api string,
	model string,
	reqBody []byte,
	metricsBody []byte,
) (usage map[string]any, usageStage string, upstreamUsage *dslconfig.Usage) {
	if cfg, ok := pf.Usage.Select(meta); ok {
		u, _, err := dslconfig.ExtractUsage(meta, cfg, metricsBody)
		if err == nil && u != nil {
			out := usageestimate.Estimate(estCfg, usageestimate.Input{
				API:           api,
				Model:         model,
				UpstreamUsage: u,
				RequestBody:   reqBody,
				RequestRoot:   meta.RequestRoot(),
				ResponseBody:  metricsBody,
			})
			return usageMap(out.Usage), out.Stage, u
		}
	}
	out := usageestimate.Estimate(estCfg, usageestimate.Input{
		API:          api,
		Model:        model,
		RequestBody:  reqBody,
		RequestRoot:  meta.RequestRoot(),
		ResponseBody: metricsBody,
	})
	return usageMap(out.Usage), out.Stage, nil
}

// populateNonStreamDerivedUsage requires a non-nil meta and response.
func populateNonStreamDerivedUsage(meta *dslmeta.Meta, pf dslconfig.ProviderFile, resp *http.Response, respBody []byte) {
	if len(respBody) == 0 {
		return
	}
	if resp.StatusCode != http.StatusOK {
		return
	}
	usageCfg, ok := pf.Usage.Select(meta)
	if !ok || !dslconfig.UsesDerivedUsagePath(meta, usageCfg, "$.audio_duration_seconds") {
		return
	}
	derived := onraudio.BuildSpeechDerivedUsage(respBody, 0)
	if len(derived) == 0 {
		return
	}
	if meta.DerivedUsage == nil {
		meta.DerivedUsage = map[string]any{}
	}
	for k, v := range derived {
		meta.DerivedUsage[k] = v
	}
}

func extractNonStreamFinishReason(
	pf dslconfig.ProviderFile,
	meta *dslmeta.Meta,
	metricsObj map[string]any,
	metricsBody []byte,
) string {
	finishReason := ""
	if frCfg, ok := pf.Finish.Select(meta); ok {
		var (
			v   string
			err error
		)
		if metricsObj != nil {
			v, err = dslconfig.ExtractFinishReasonObject(meta, frCfg, metricsObj)
		} else {
			v, err = dslconfig.ExtractFinishReason(meta, frCfg, metricsBody)
		}
		if err == nil {
			finishReason = strings.TrimSpace(v)
		}
	}
	return finishReason
}

func numberAsFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func shouldEstimateUsage(statusCode int) bool {
	return statusCode == http.StatusOK
}

// logUsageFactsDebug requires a non-nil Client receiver.
func (c *Client) logUsageFactsDebug(
	gc *gin.Context,
	provider string,
	api string,
	stream bool,
	model string,
	usageStage string,
	usage *dslconfig.Usage,
) {
	if usage == nil || len(usage.DebugFacts) == 0 {
		return
	}
	payload, err := json.Marshal(usage.DebugFacts)
	if err != nil {
		return
	}
	fields := map[string]any{
		"provider":    provider,
		"api":         strings.TrimSpace(api),
		"stream":      stream,
		"model":       strings.TrimSpace(model),
		"usage_stage": strings.TrimSpace(usageStage),
		"usage_facts": string(payload),
	}
	if rid := strings.TrimSpace(gc.GetString("X-Onr-Request-Id")); rid != "" {
		fields["request_id"] = rid
	} else if rid := strings.TrimSpace(gc.GetString("X-Request-Id")); rid != "" {
		fields["request_id"] = rid
	}
	c.SystemLogger.Debug(logx.SystemCategoryServer, "usage facts extracted", fields)
}

func usageMap(u *dslconfig.Usage) map[string]any {
	if u == nil {
		return nil
	}
	// normalize totals for callers
	out := usageestimate.Estimate(nil, usageestimate.Input{UpstreamUsage: u})
	u = out.Usage
	if u == nil {
		return nil
	}
	m := map[string]any{
		"input_tokens":  u.InputTokens,
		"output_tokens": u.OutputTokens,
		"total_tokens":  u.TotalTokens,
	}
	if u.InputTokenDetails != nil {
		m["cache_read_tokens"] = u.InputTokenDetails.CachedTokens
		m["cache_write_tokens"] = u.InputTokenDetails.CacheWriteTokens
	}
	for k, v := range u.FlatFields {
		if strings.TrimSpace(k) == "" {
			continue
		}
		if _, exists := m[k]; exists {
			continue
		}
		m[k] = v
	}
	return m
}
