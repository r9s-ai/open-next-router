package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitransform"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/ssecollect"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
)

func (c *Client) handleNonStreamResponse(
	gc *gin.Context,
	provider string,
	key ProviderKey,
	api string,
	stream bool,
	start time.Time,
	pf dslconfig.ProviderFile,
	m *dslmeta.Meta,
	model string,
	reqBody []byte,
	respDir *dslconfig.ResponseDirective,
	resp *http.Response,
) (*Result, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// traffic dump: upstream response (original bytes)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		limited, truncated := trafficdump.LimitBytes(respBody, rec.MaxBytes())
		binary := trafficdump.IsBinaryPayload(ct, limited)
		trafficdump.AppendUpstreamResponse(gc, resp.Status, resp.Header, limited, binary, truncated)
	}

	respOutBody, respOutObj, outCT, didTransform, err := mapNonStreamResponse(gc.Request.Context(), respBody, resp, respDir)
	if err != nil {
		return nil, err
	}
	if respOutBody == nil && respOutObj != nil {
		respOutBody, err = json.Marshal(respOutObj)
		if err != nil {
			return nil, err
		}
	}

	// metrics are extracted from the response after response mapping (resp_map),
	// but before response json ops (json_del/json_set/json_rename) so operators can strip fields
	// from downstream without losing upstream usage/finish_reason signals.
	metricsBody := respOutBody
	populateNonStreamDerivedUsage(gc, m, pf, model, resp, metricsBody)
	estimateEnabled := shouldEstimateUsage(resp.StatusCode)

	usage := map[string]any(nil)
	usageStage := ""
	var upstreamUsage *dslconfig.Usage
	if estimateEnabled {
		usage, usageStage, upstreamUsage = estimateNonStreamUsage(c.UsageEst, pf, m, api, model, reqBody, metricsBody)
	}
	finishReason := ""
	if estimateEnabled {
		finishReason = extractNonStreamFinishReason(pf, m, respOutObj, metricsBody)
	}
	cost := map[string]any(nil)
	if estimateEnabled {
		cost = c.computeCost(m, provider, key.Name, usage)
	}
	c.logUsageFactsDebug(gc, provider, api, stream, model, usageStage, upstreamUsage)

	var responseJSONOps []dslconfig.JSONOp
	if respDir != nil {
		responseJSONOps = respDir.JSONOps
	}
	respOutBody, outCT, didTransform, err = applyNonStreamResponseJSONOps(respOutObj, respOutBody, outCT, resp, m, responseJSONOps, didTransform)
	if err != nil {
		return nil, err
	}

	copyHeadersToClient(gc, resp.Header, didTransform)
	if strings.TrimSpace(outCT) != "" {
		gc.Writer.Header().Set("Content-Type", outCT)
	}

	gc.Status(resp.StatusCode)
	if _, err := gc.Writer.Write(respOutBody); err != nil {
		return nil, err
	}

	// traffic dump: proxy response (final downstream bytes)
	if rec := trafficdump.FromContext(gc); rec != nil && rec.MaxBytes() > 0 {
		ct := strings.ToLower(outCT)
		limited, truncated := trafficdump.LimitBytes(respOutBody, rec.MaxBytes())
		binary := trafficdump.IsBinaryPayload(ct, limited)
		trafficdump.AppendProxyResponse(gc, limited, binary, truncated, resp.StatusCode)
	}

	return &Result{
		Provider:       provider,
		ProviderKey:    key.Name,
		ProviderSource: "dsl",
		API:            api,
		Stream:         stream,
		Model:          model,
		Status:         resp.StatusCode,
		LatencyMs:      time.Since(start).Milliseconds(),
		Usage:          usage,
		UsageStage:     usageStage,
		FinishReason:   finishReason,
		Cost:           cost,
	}, nil
}

// mapNonStreamResponse requires a non-nil upstream response from the non-stream proxy path.
func mapNonStreamResponse(ctx context.Context, respBody []byte, resp *http.Response, respDir *dslconfig.ResponseDirective) ([]byte, map[string]any, string, bool, error) {
	respOutBody := respBody
	outCT := resp.Header.Get("Content-Type")
	var root map[string]any
	if shouldCollectSSE(respDir, resp) {
		if resp.StatusCode >= http.StatusBadRequest {
			return respOutBody, nil, outCT, false, nil
		}
		decoded, _, err := apitransform.DecodeResponseBody(respBody, resp.Header.Get("Content-Encoding"))
		if err != nil {
			return nil, nil, outCT, false, err
		}
		collected, err := ssecollect.CollectByMode(ctx, respDir.SSECollectMode, bytes.NewReader(decoded), ssecollect.Options{})
		if err != nil {
			return nil, nil, outCT, false, err
		}
		root = collected
		respOutBody = nil
		outCT = contentTypeJSON
	}
	if respDir == nil || respDir.Op != "resp_map" {
		if root != nil {
			return nil, root, outCT, true, nil
		}
		return respOutBody, nil, outCT, false, nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return respOutBody, nil, outCT, false, nil
	}
	if !apitransform.SupportsResponseMapMode(respDir.Mode) {
		return respOutBody, nil, outCT, false, nil
	}
	if root == nil {
		decoded, _, err := apitransform.DecodeResponseBody(respBody, resp.Header.Get("Content-Encoding"))
		if err != nil {
			return nil, nil, outCT, false, err
		}
		if err := json.Unmarshal(decoded, &root); err != nil {
			return nil, nil, outCT, false, err
		}
	}
	outObj, outCT, changed, err := apitransform.TransformNonStreamResponseBody(
		resp.StatusCode,
		respDir.Mode,
		root,
		outCT,
	)
	if err != nil {
		return nil, nil, outCT, changed, err
	}
	if !changed {
		if respOutBody == nil && root != nil {
			return nil, root, outCT, true, nil
		}
		return respOutBody, outObj, outCT, false, nil
	}
	return nil, outObj, outCT, true, nil
}

func shouldCollectSSE(respDir *dslconfig.ResponseDirective, resp *http.Response) bool {
	if respDir == nil || strings.TrimSpace(respDir.SSECollectMode) == "" || resp == nil {
		return false
	}
	upstreamCT := strings.ToLower(resp.Header.Get("Content-Type"))
	return strings.Contains(upstreamCT, "text/event-stream")
}

func applyNonStreamResponseJSONOps(
	respOutObj map[string]any,
	respOutBody []byte,
	outCT string,
	resp *http.Response,
	meta *dslmeta.Meta,
	ops []dslconfig.JSONOp,
	didTransform bool,
) ([]byte, string, bool, error) {
	if len(ops) == 0 {
		return respOutBody, outCT, didTransform, nil
	}

	if respOutObj == nil && !apitransform.ResponseBodyLooksLikeJSON(outCT, respOutBody) {
		return respOutBody, outCT, didTransform, nil
	}

	if respOutObj == nil {
		decoded, changed, err := apitransform.DecodeResponseBody(respOutBody, resp.Header.Get("Content-Encoding"))
		if err != nil {
			return nil, "", false, err
		}
		if changed {
			respOutBody = decoded
		}
		if err := json.Unmarshal(respOutBody, &respOutObj); err != nil {
			return nil, "", false, err
		}
		if respOutObj == nil {
			return nil, "", false, fmt.Errorf("response json ops require json object body")
		}
	}

	mappedObj, err := dslconfig.ApplyJSONOps(meta, respOutObj, ops)
	if err != nil {
		return nil, "", false, err
	}
	outBytes, err := json.Marshal(mappedObj)
	if err != nil {
		return nil, "", false, err
	}
	respOutBody = outBytes
	if !strings.Contains(outCT, "json") {
		outCT = "application/json"
	}
	return respOutBody, outCT, true, nil
}

func copyHeadersToClient(gc *gin.Context, hdr http.Header, didTransform bool) {
	for k, vs := range hdr {
		if len(vs) == 0 {
			continue
		}
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		if strings.EqualFold(k, "Content-Encoding") && didTransform {
			continue
		}
		for _, item := range vs {
			gc.Writer.Header().Add(k, item)
		}
	}
}
