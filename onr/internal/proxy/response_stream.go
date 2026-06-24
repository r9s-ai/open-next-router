package proxy

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
)

func (c *Client) handleStreamResponse(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	start time.Time,
	pf dslconfig.ProviderFile,
	m *dslmeta.Meta,
	model string,
	reqBody []byte,
	respDir *dslconfig.ResponseDirective,
	resp *http.Response,
) (*Result, error) {
	// copy headers
	copyHeadersToClient(gc, resp.Header, false)

	// Always keep a tail buffer for best-effort usage extraction from SSE.
	tailLimit := 256 << 10 // 256KB
	if c.UsageEst != nil && c.UsageEst.MaxStreamCollectBytes > 0 {
		tailLimit = c.UsageEst.MaxStreamCollectBytes
	}
	usageTail := &tailBuffer{limit: tailLimit}
	usageCfg, _ := pf.Usage.Select(m)
	finishCfg, _ := pf.Finish.Select(m)
	streamAgg := dslconfig.NewStreamMetricsAggregator(m, usageCfg, finishCfg)
	tapRawSSEForMetrics := shouldTapRawSSEForMetrics(usageCfg, finishCfg)

	var metricsTap *sseMetricsTap
	upstreamCT := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(upstreamCT, "text/event-stream") {
		metricsTap = newSSEMetricsTap(streamAgg)
	}

	dump := newStreamDumpState(gc)
	defer dump.Append(gc, resp)

	n, firstWriteAt, err := streamToDownstream(gc, m, respDir, resp, usageTail, metricsTap, tapRawSSEForMetrics, dump)
	ignoredDisconnect := isClientDisconnectErr(err)
	dump.SetStreamResult(n, err, ignoredDisconnect)
	if err != nil && !ignoredDisconnect {
		return nil, err
	}
	if f, ok := gc.Writer.(http.Flusher); ok {
		f.Flush()
	}

	// best-effort: extract metrics from SSE stream tail via pkg/dslconfig aggregator
	estimateEnabled := shouldEstimateUsage(resp.StatusCode)
	var upstreamUsage *dslconfig.Usage
	finishReason := ""
	if estimateEnabled && usageTail.Len() > 0 {
		u, _, fr, ok := streamAgg.Result()
		if (!ok || u == nil) && strings.Contains(upstreamCT, "text/event-stream") {
			streamAgg.OnSSETail(usageTail.Bytes())
			u, _, fr, ok = streamAgg.Result()
		}
		if ok && u != nil {
			upstreamUsage = u
		}
		finishReason = strings.TrimSpace(fr)
	}

	usage := map[string]any(nil)
	usageStage := ""
	cost := map[string]any(nil)
	if estimateEnabled {
		out := usageestimate.Estimate(c.UsageEst, usageestimate.Input{
			API:           api,
			Model:         model,
			UpstreamUsage: upstreamUsage,
			RequestBody:   reqBody,
			RequestRoot:   m.RequestRoot(),
			StreamTail:    usageTail.Bytes(),
		})
		usage = usageMap(out.Usage)
		usageStage = out.Stage
		cost = c.computeCost(m, provider, key.Name, usage)
	}
	c.logUsageFactsDebug(gc, provider, api, true, model, usageStage, upstreamUsage)
	ttftMs, tps := streamPerfMetrics(start, firstWriteAt, usage)
	return &Result{
		Provider:       provider,
		ProviderKey:    key.Name,
		ProviderSource: "dsl",
		API:            api,
		Stream:         true,
		Model:          model,
		Status:         resp.StatusCode,
		LatencyMs:      time.Since(start).Milliseconds(),
		Usage:          usage,
		UsageStage:     usageStage,
		FinishReason:   finishReason,
		Cost:           cost,
		TTFTMs:         ttftMs,
		TPS:            tps,
	}, nil
}

func shouldTapRawSSEForMetrics(usageCfg *dslconfig.UsageExtractConfig, finishCfg *dslconfig.FinishReasonExtractConfig) bool {
	// Stream metrics should be tapped from upstream raw SSE before any response conversion.
	// This is not anthropic-specific: any configured stream metrics extraction should use raw events.
	return usageCfg != nil || finishCfg != nil
}

func streamPerfMetrics(start time.Time, firstWriteAt time.Time, usage map[string]any) (int64, float64) {
	if start.IsZero() || firstWriteAt.IsZero() || firstWriteAt.Before(start) {
		return 0, 0
	}
	ttftMs := firstWriteAt.Sub(start).Milliseconds()
	if ttftMs < 0 {
		ttftMs = 0
	}
	outputTokens, ok := numberAsFloat64(usage["output_tokens"])
	if !ok || outputTokens <= 0 {
		return ttftMs, 0
	}
	elapsed := time.Since(firstWriteAt).Seconds()
	if elapsed <= 0 {
		return ttftMs, 0
	}
	return ttftMs, outputTokens / elapsed
}
