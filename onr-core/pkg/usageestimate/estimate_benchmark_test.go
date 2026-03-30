package usageestimate

import (
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func BenchmarkEstimateChatCompletions_New(b *testing.B) {
	cfg := benchmarkEstimateConfig()
	in := benchmarkEstimateInput(nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out := Estimate(cfg, in)
		if out.Usage == nil || out.Usage.InputTokens == 0 {
			b.Fatalf("unexpected output %#v", out)
		}
	}
}

func BenchmarkEstimateChatCompletions_OldDoubleParse(b *testing.B) {
	cfg := benchmarkEstimateConfig()
	in := benchmarkEstimateInput(nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out := benchmarkLegacyEstimate(cfg, in)
		if out.Usage == nil || out.Usage.InputTokens == 0 {
			b.Fatalf("unexpected output %#v", out)
		}
	}
}

func BenchmarkEstimateChatCompletions_WithRequestRoot(b *testing.B) {
	cfg := benchmarkEstimateConfig()
	root := map[string]any{
		"model": "gpt-4.1",
		"messages": []any{
			map[string]any{"role": "system", "content": "You are helpful."},
			map[string]any{"role": "user", "content": "Summarize this message."},
			map[string]any{"role": "user", "content": "Add key bullet points and risks."},
		},
	}
	in := benchmarkEstimateInput(root)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out := Estimate(cfg, in)
		if out.Usage == nil || out.Usage.InputTokens == 0 {
			b.Fatalf("unexpected output %#v", out)
		}
	}
}

func benchmarkEstimateConfig() *Config {
	cfg := &Config{}
	ApplyDefaults(cfg)
	return cfg
}

func benchmarkEstimateInput(root map[string]any) Input {
	return Input{
		API:          "chat.completions",
		Model:        "gpt-4.1",
		RequestBody:  []byte(`{"model":"gpt-4.1","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Summarize this message."},{"role":"user","content":"Add key bullet points and risks."}]}`),
		RequestRoot:  root,
		ResponseBody: []byte(`{"choices":[{"message":{"content":"Summary:\n- point one\n- point two\nRisk: stale input."}}]}`),
	}
}

func benchmarkLegacyEstimate(cfg *Config, in Input) Output {
	if cfg == nil || !cfg.IsAPIEnabled(in.API) {
		u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
		return Output{Usage: u, Stage: stage}
	}

	u, stage := normalizeUpstreamUsage(in.UpstreamUsage)
	if u != nil {
		if !cfg.EstimateWhenMissingOrZero {
			return Output{Usage: u, Stage: stage}
		}
		if stage == StageUpstream {
			return Output{Usage: u, Stage: stage}
		}
		if !isAllZero(u) {
			return Output{Usage: u, Stage: stage}
		}
	}
	if u == nil && !cfg.EstimateWhenMissingOrZero {
		return Output{Usage: nil, Stage: ""}
	}

	reqText := extractRequestText(in.API, in.RequestBody, cfg.MaxRequestBytes)
	respText := ""
	if len(in.StreamTail) > 0 {
		respText = extractStreamText(in.API, in.StreamTail, cfg.MaxStreamCollectBytes)
	} else {
		respText = extractResponseText(in.API, in.ResponseBody, cfg.MaxResponseBytes)
	}

	est := &dslconfig.Usage{
		InputTokens:  EstimateTokenByModel(in.Model, reqText),
		OutputTokens: EstimateTokenByModel(in.Model, respText),
	}
	est.TotalTokens = est.InputTokens + est.OutputTokens

	if strings.ToLower(strings.TrimSpace(in.API)) == apiChatCompletions {
		msgCount := countMessages(in.RequestBody, cfg.MaxRequestBytes)
		if msgCount > 0 {
			est.InputTokens += msgCount*3 + 3
			est.TotalTokens = est.InputTokens + est.OutputTokens
		}
	}

	return Output{Usage: est, Stage: StageEstimateBoth}
}
