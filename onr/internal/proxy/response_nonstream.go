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
	"github.com/r9s-ai/open-next-router/onr-core/pkg/respinline"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/ssecollect"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/trafficdump"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
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

	// Inlining runs after the metrics snapshot and before the body is serialized
	// for the client. Metrics count entries, not bytes, so they gain nothing
	// from the fetched content — while a snapshot taken afterwards would carry
	// every inlined asset through usage extraction and the traffic dump.
	respOutObj, respOutBody, didTransform, err = c.applyResponseInlineURL(gc, respOutObj, respOutBody, outCT, resp, respDir, didTransform)
	if err != nil {
		return nil, err
	}
	if respOutBody == nil && respOutObj != nil {
		respOutBody, err = json.Marshal(respOutObj)
		if err != nil {
			return nil, err
		}
	}

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

// applyResponseInlineURL runs the resp_inline_url rule, if the matched response
// directive has one, and returns the response object and body to carry forward.
//
// It parses a passthrough body on demand: a provider can already answer in the
// downstream shape and still need its links inlined, so the rule must not
// depend on resp_map having run.
//
// Fetch failures are logged and left in place: the rule degrades to the URL the
// upstream returned rather than failing a response that already succeeded.
//
// The gate reads the client's original request, not the mapped upstream one: a
// provider that always returns links has no field to carry the caller's choice
// of response format.
func (c *Client) applyResponseInlineURL(
	gc *gin.Context,
	root map[string]any,
	body []byte,
	outCT string,
	resp *http.Response,
	respDir *dslconfig.ResponseDirective,
	didTransform bool,
) (map[string]any, []byte, bool, error) {
	if respDir == nil || respDir.InlineURL == nil || c.HTTP == nil {
		return root, body, didTransform, nil
	}

	parsedHere := false
	if root == nil {
		if !apitransform.ResponseBodyLooksLikeJSON(outCT, body) {
			return root, body, didTransform, nil
		}
		decoded, _, err := apitransform.DecodeResponseBody(body, resp.Header.Get("Content-Encoding"))
		if err != nil {
			return nil, nil, didTransform, err
		}
		root = jsonObjectOrNil(decoded)
		if root == nil {
			// A body that is not a JSON object has nothing to inline. That is
			// not a failure of the response, only of this rule's applicability.
			return nil, body, didTransform, nil
		}
		parsedHere = true
	}

	var requestRoot map[string]any
	if cached, ok := gc.Get("onr.request_root"); ok {
		requestRoot, _ = cached.(map[string]any)
	}
	res := respinline.Apply(gc.Request.Context(), root, requestRoot, respDir.InlineURL, c.HTTP)
	if res.Failed > 0 && c.SystemLogger != nil {
		c.SystemLogger.Warn(logx.SystemCategoryServer, "resp_inline_url left URLs in place", map[string]any{
			"attempted": res.Attempted,
			"inlined":   res.Inlined,
			"failed":    res.Failed,
			"error":     fmt.Sprint(res.FirstError),
		})
	}
	if res.Inlined == 0 {
		if parsedHere {
			// Nothing changed, so keep the original bytes rather than
			// re-serializing a body the client did not ask us to rewrite.
			return nil, body, didTransform, nil
		}
		return root, body, didTransform, nil
	}
	// Drop the body so the caller re-serializes from the mutated object.
	return root, nil, true, nil
}

// jsonObjectOrNil decodes body as a JSON object, or returns nil when it is not
// one. Callers use the nil to mean "not applicable" rather than "failed".
func jsonObjectOrNil(body []byte) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil
	}
	return out
}
