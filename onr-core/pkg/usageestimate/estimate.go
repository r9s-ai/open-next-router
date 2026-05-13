package usageestimate

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

const (
	StageUpstream           = "upstream"
	StageEstimateBoth       = "estimate_both"
	StageEstimatePrompt     = "estimate_prompt"
	StageEstimateCompletion = "estimate_completion"
)

const (
	apiChatCompletions = "chat.completions"
	apiEmbeddings      = "embeddings"
	apiResponses       = "responses"
	apiMessages        = "claude.messages"

	apiGeminiGenerateContent       = "gemini.generatecontent"
	apiGeminiStreamGenerateContent = "gemini.streamgeneratecontent"
)

type Input struct {
	API   string
	Model string

	UpstreamUsage *dslconfig.Usage

	// Upstream request/response bodies (JSON for non-stream, SSE for stream).
	RequestBody  []byte
	RequestRoot  map[string]any
	ResponseBody []byte
	StreamTail   []byte
}

type Output struct {
	Usage *dslconfig.Usage
	Stage string
}

type parsedRequestBody struct {
	raw  []byte
	obj  any
	root map[string]any
	err  error
}

type tokenEstimateContext struct {
	text                     string
	completion               bool // is output
	numTools                 int  // number of tool definitions
	numMessages              int
	numFunctionCalls         int
	numFunctionCallOutputs   int
	numCustomToolCalls       int
	numCustomToolCallOutputs int
}

func Estimate(cfg *Config, in Input) Output {
	if cfg == nil || !cfg.IsAPIEnabled(in.API) {
		u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
		return Output{Usage: u, Stage: stage}
	}

	u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
	if u != nil {
		if !cfg.EstimateWhenMissingOrZero {
			return Output{Usage: u, Stage: stage}
		}
		// Upstream usage exists; optionally estimate missing fields (common in streaming).
		if stage == StageUpstream {
			if outU, outStage := estimateMissingFields(cfg, in, u); outStage != StageUpstream {
				return Output{Usage: outU, Stage: outStage}
			}
			return Output{Usage: u, Stage: stage}
		}
		// All-zero (or effectively missing) usage: allow estimation.
		if !isAllZero(u) {
			return Output{Usage: u, Stage: stage}
		}
	}
	if u == nil && !cfg.EstimateWhenMissingOrZero {
		return Output{Usage: nil, Stage: ""}
	}

	reqParsed := parseRequestBody(in.RequestBody, in.RequestRoot, cfg.MaxRequestBytes)
	reqCtx := extractRequestTextFromParsed(in.API, reqParsed)
	respText := ""
	if len(in.StreamTail) > 0 {
		respText = extractStreamText(in.API, in.StreamTail, cfg.MaxStreamCollectBytes)
	} else {
		respText = extractResponseTextForModel(in.API, in.Model, in.ResponseBody, cfg.MaxResponseBytes)
	}

	respCtx := &tokenEstimateContext{text: respText, completion: true, numTools: reqCtx.numTools}
	est := &dslconfig.Usage{
		InputTokens:  EstimateTokenByModel(in.Model, reqCtx),
		OutputTokens: EstimateTokenByModel(in.Model, respCtx),
	}
	est.TotalTokens = est.InputTokens + est.OutputTokens

	// Best-effort overhead for OpenAI-style chat messages.
	if normalizeAPI(in.API) == apiChatCompletions {
		msgCount := countMessagesFromParsed(reqParsed)
		if msgCount > 0 {
			est.InputTokens += msgCount*3 + 3
			est.TotalTokens = est.InputTokens + est.OutputTokens
		}
	}

	return Output{Usage: est, Stage: StageEstimateBoth}
}

func estimateMissingFields(cfg *Config, in Input, u *dslconfig.Usage) (*dslconfig.Usage, string) {
	if cfg == nil || u == nil {
		return u, StageUpstream
	}
	needPrompt := u.InputTokens == 0
	needCompletion := u.OutputTokens == 0
	if !needPrompt && !needCompletion {
		return u, StageUpstream
	}

	reqParsed := parseRequestBody(in.RequestBody, in.RequestRoot, cfg.MaxRequestBytes)
	reqCtx := &tokenEstimateContext{}
	if needPrompt {
		reqCtx = extractRequestTextFromParsed(in.API, reqParsed)
		if strings.TrimSpace(reqCtx.text) == "" {
			needPrompt = false
		}
	}

	respCtx := &tokenEstimateContext{}
	var respText string
	if needCompletion {
		if len(in.StreamTail) > 0 {
			respText = extractStreamText(in.API, in.StreamTail, cfg.MaxStreamCollectBytes)
		} else {
			respText = extractResponseTextForModel(in.API, in.Model, in.ResponseBody, cfg.MaxResponseBytes)
		}
		if strings.TrimSpace(respText) == "" {
			needCompletion = false
		}
	}

	if !needPrompt && !needCompletion {
		return u, StageUpstream
	}

	out := *u
	if needPrompt {
		out.InputTokens = EstimateTokenByModel(in.Model, reqCtx)
	}
	if needCompletion {
		respCtx = &tokenEstimateContext{text: respText, completion: true}
		out.OutputTokens = EstimateTokenByModel(in.Model, respCtx)
	}
	out.TotalTokens = out.InputTokens + out.OutputTokens

	// Best-effort overhead for OpenAI-style chat messages only when prompt is estimated.
	if needPrompt && normalizeAPI(in.API) == apiChatCompletions {
		msgCount := countMessagesFromParsed(reqParsed)
		if msgCount > 0 {
			out.InputTokens += msgCount*3 + 3
			out.TotalTokens = out.InputTokens + out.OutputTokens
		}
	}

	switch {
	case needPrompt && needCompletion:
		return &out, StageEstimateBoth
	case needPrompt:
		return &out, StageEstimatePrompt
	case needCompletion:
		return &out, StageEstimateCompletion
	default:
		return u, StageUpstream
	}
}

func normalizeUpstreamUsage(u *dslconfig.Usage) (*dslconfig.Usage, string) {
	if u == nil {
		return nil, ""
	}
	// Copy to avoid mutating callers.
	out := *u

	// Normalize legacy OpenAI fields.
	if out.InputTokens == 0 && out.PromptTokens != 0 {
		out.InputTokens = out.PromptTokens
	}
	if out.OutputTokens == 0 && out.CompletionTokens != 0 {
		out.OutputTokens = out.CompletionTokens
	}
	if out.TotalTokens == 0 && (out.InputTokens != 0 || out.OutputTokens != 0) {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}

	if isAllZero(&out) {
		return &out, ""
	}
	return &out, StageUpstream
}

func isAllZero(u *dslconfig.Usage) bool {
	if u == nil {
		return true
	}
	return u.InputTokens == 0 && u.OutputTokens == 0 && u.TotalTokens == 0 &&
		(u.InputTokenDetails == nil || (u.InputTokenDetails.CachedTokens == 0 && u.InputTokenDetails.CacheWriteTokens == 0))
}
