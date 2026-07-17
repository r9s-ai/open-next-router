package proxy

import (
	"encoding/json"
	"io"
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

// populateNonStreamDerivedUsage requires a non-nil meta and response. It
// prepares the derived usage keys the selected usage mode references:
// TTS 响应音频的时长/估算 tokens,以及 whisper 类计费需要的请求音频时长与
// 输入/输出文本 token 预估(与 relay 侧派生键对齐)。
func populateNonStreamDerivedUsage(gc *gin.Context, meta *dslmeta.Meta, pf dslconfig.ProviderFile, model string, resp *http.Response, respBody []byte) {
	if resp.StatusCode != http.StatusOK {
		return
	}
	usageCfg, ok := pf.Usage.Select(meta)
	if !ok {
		return
	}
	derived := map[string]any{}

	if len(respBody) > 0 &&
		(dslconfig.UsesDerivedUsagePath(meta, usageCfg, "$.audio_duration_seconds") ||
			dslconfig.UsesDerivedUsagePath(meta, usageCfg, "$.audio_estimated_tokens")) {
		for k, v := range onraudio.BuildSpeechDerivedUsage(respBody, 0) {
			derived[k] = v
		}
	}
	if dslconfig.UsesDerivedUsagePath(meta, usageCfg, "$.input_text_tokens") {
		if input, _ := meta.RequestRoot()["input"].(string); input != "" {
			if n, err := usageestimate.EstimateToken(model, meta.API, input, usageestimate.EstimateInput); err == nil && n > 0 {
				derived["input_text_tokens"] = float64(n)
			}
		}
	}
	if dslconfig.UsesDerivedUsagePath(meta, usageCfg, "$.output_text_tokens") {
		if text := responseTextField(respBody); text != "" {
			if n, err := usageestimate.EstimateToken(model, meta.API, text, usageestimate.EstimateOutput); err == nil && n > 0 {
				derived["output_text_tokens"] = float64(n)
			}
		}
	}
	if dslconfig.UsesDerivedUsagePath(meta, usageCfg, "$.request_audio_duration_seconds") {
		if payloads := requestAudioPayloads(gc); len(payloads) > 0 {
			derived["request_audio_duration_seconds"] = onraudio.SumDurationsFromPayloadsOrDefault(payloads, 1.0)
		}
	}

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

// responseTextField parses the upstream JSON body and returns its top-level
// "text" field, or "" for non-JSON/binary bodies.
func responseTextField(respBody []byte) string {
	if len(respBody) == 0 {
		return ""
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ""
	}
	return parsed.Text
}

// requestAudioPayloads reads the uploaded audio file bytes from the multipart
// request ("file" field, matching the OpenAI audio endpoints). Returns nil
// when the request carries no readable audio file.
func requestAudioPayloads(gc *gin.Context) [][]byte {
	file, _, err := gc.Request.FormFile("file")
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil || len(data) == 0 {
		return nil
	}
	return [][]byte{data}
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
