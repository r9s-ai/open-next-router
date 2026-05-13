package usageestimate

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

// Test Anthropic request token estimation. This test is for development use only.
func TestEstimate_AnthropicInput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat_resp",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/agent_chat/req.json",
			want: 2342},
		{name: "chinese_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_chat/req.json",
			want: 1955},
		{name: "english_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_chat/req.json",
			want: 1559},
		{name: "chinese_agent",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_agent/req.json",
			want: 3661},
		{name: "english_agent",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent/req.json",
			want: 4246},
		{name: "english_agent_1tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent_1tool/req.json",
			want: 537},
		{name: "english_agent_2tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent_2tool/req.json",
			want: 570},
		{name: "english_agent_4tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent_4tool/req.json",
			want: 636},
		{name: "chinese_agent_1tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_agent_1tool/req.json",
			want: 1548},
		{name: "chinese_agent_2tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_agent_2tool/req.json",
			want: 1911},
		{name: "chinese_chat_short",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_chat_short/req.json",
			want: 534},
		{name: "chinese_chat_long",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_chat_long/req.json",
			want: 3305},
		{name: "english_chat_short",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_chat_short/req.json",
			want: 343},
		{name: "english_chat_long",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_chat_long/req.json",
			want: 2141},
		{name: "code_review_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/code_review_chat/req.json",
			want: 1584},
		{name: "code_writing_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/code_writing_chat/req.json",
			want: 2110},
		{name: "mixed_en_zh_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/mixed_en_zh_chat/req.json",
			want: 1276},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "claude.messages",
				Model:         "claude-3-5-sonnet",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 6, TotalTokens: 6},
				RequestBody:   data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official input=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.InputTokens, deviation(item.want, out.Usage.InputTokens))
		})
	}
}

// Test Anthropic response token estimation. This test is for development use only.
func TestEstimate_AnthropicOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat_resp",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/agent_chat/resp.json",
			want: 512},
		{name: "chinese_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_chat/resp.json",
			want: 107},
		{name: "english_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_chat/resp.json",
			want: 453},
		{name: "chinese_agent",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_agent/resp.json",
			want: 2576},
		{name: "english_agent",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent/resp.json",
			want: 1283},
		{name: "english_agent_1tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent_1tool/resp.json",
			want: 16},
		{name: "english_agent_2tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent_2tool/resp.json",
			want: 16},
		{name: "english_agent_4tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_agent_4tool/resp.json",
			want: 18},
		{name: "chinese_agent_1tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_agent_1tool/resp.json",
			want: 703},
		{name: "chinese_agent_2tool",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_agent_2tool/resp.json",
			want: 641},
		{name: "chinese_chat_short",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_chat_short/resp.json",
			want: 545},
		{name: "chinese_chat_long",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/chinese_chat_long/resp.json",
			want: 738},
		{name: "english_chat_short",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_chat_short/resp.json",
			want: 591},
		{name: "english_chat_long",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/english_chat_long/resp.json",
			want: 734},
		{name: "code_review_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/code_review_chat/resp.json",
			want: 439},
		{name: "code_writing_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/code_writing_chat/resp.json",
			want: 1487},
		{name: "mixed_en_zh_chat",
			in:   "testdata/anthropic/messages/claude-haiku-4-5/mixed_en_zh_chat/resp.json",
			want: 787},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "claude.messages",
				Model:         "claude-3-5-sonnet",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
				ResponseBody:  data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official output=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.OutputTokens, deviation(item.want, out.Usage.OutputTokens))
		})
	}
}

// Test Anthropic 4.7 request token estimation. This test is for development use only.
func TestEstimate_Anthropic47Input(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/agent_chat/req.json",
			want: 1162},
		{name: "chinese_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_chat/req.json",
			want: 1734},
		{name: "english_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_chat/req.json",
			want: 1854},
		{name: "chinese_agent",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_agent/req.json",
			want: 1748},
		{name: "english_agent",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent/req.json",
			want: 2734},
		{name: "english_agent_0tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_0tool/req.json",
			want: 2},
		{name: "english_agent_1tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_1tool/req.json",
			want: 58},
		{name: "english_agent_2tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_2tool/req.json",
			want: 114},
		{name: "english_agent_4tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_4tool/req.json",
			want: 225},
		{name: "chinese_agent_1tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_agent_1tool/req.json",
			want: 626},
		{name: "chinese_agent_2tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_agent_2tool/req.json",
			want: 815},
		{name: "chinese_chat_short",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_chat_short/req.json",
			want: 432},
		{name: "chinese_chat_long",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_chat_long/req.json",
			want: 2756},
		{name: "english_chat_short",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_chat_short/req.json",
			want: 411},
		{name: "english_chat_long",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_chat_long/req.json",
			want: 2445},
		{name: "code_review_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/code_review_chat/req.json",
			want: 1658},
		{name: "code_writing_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/code_writing_chat/req.json",
			want: 1913},
		{name: "mixed_en_zh_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/mixed_en_zh_chat/req.json",
			want: 1410},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "claude.messages",
				Model:         "claude-opus-4-7",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 6, TotalTokens: 6},
				RequestBody:   data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official input=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.InputTokens, deviation(item.want, out.Usage.InputTokens))
		})
	}
}

// Test Anthropic 4.7 response token estimation. This test is for development use only.
func TestEstimate_Anthropic47Output(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/agent_chat/resp.json",
			want: 264},
		{name: "chinese_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_chat/resp.json",
			want: 142},
		{name: "english_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_chat/resp.json",
			want: 690},
		{name: "chinese_agent",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_agent/resp.json",
			want: 1685},
		{name: "english_agent",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent/resp.json",
			want: 1302},
		{name: "english_agent_0tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_0tool/resp.json",
			want: 22},
		{name: "english_agent_1tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_1tool/resp.json",
			want: 22},
		{name: "english_agent_2tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_2tool/resp.json",
			want: 22},
		{name: "english_agent_4tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_agent_4tool/resp.json",
			want: 22},
		{name: "chinese_agent_1tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_agent_1tool/resp.json",
			want: 404},
		{name: "chinese_agent_2tool",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_agent_2tool/resp.json",
			want: 581},
		{name: "chinese_chat_short",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_chat_short/resp.json",
			want: 563},
		{name: "chinese_chat_long",
			in:   "testdata/anthropic/messages/claude-opus-4-7/chinese_chat_long/resp.json",
			want: 841},
		{name: "english_chat_short",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_chat_short/resp.json",
			want: 740},
		{name: "english_chat_long",
			in:   "testdata/anthropic/messages/claude-opus-4-7/english_chat_long/resp.json",
			want: 808},
		{name: "code_review_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/code_review_chat/resp.json",
			want: 478},
		{name: "code_writing_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/code_writing_chat/resp.json",
			want: 123},
		{name: "mixed_en_zh_chat",
			in:   "testdata/anthropic/messages/claude-opus-4-7/mixed_en_zh_chat/resp.json",
			want: 780},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "claude.messages",
				Model:         "claude-opus-4-7",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
				ResponseBody:  data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official output=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.OutputTokens, deviation(item.want, out.Usage.OutputTokens))
		})
	}
}

func TestEstimate_OpenaiResponsesInput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat",
			in:   "testdata/openai/responses/gpt-5-mini/agent_chat/req.json",
			want: 1127},
		{name: "chinese_chat",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_chat/req.json",
			want: 1423},
		{name: "english_chat",
			in:   "testdata/openai/responses/gpt-5-mini/english_chat/req.json",
			want: 1416},
		{name: "chinese_agent",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_agent/req.json",
			want: 1944},
		{name: "english_agent",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent/req.json",
			want: 2795},
		{name: "english_agent_0tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_0tool/req.json",
			want: 7},
		{name: "english_agent_1tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_1tool/req.json",
			want: 32},
		{name: "english_agent_2tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_2tool/req.json",
			want: 41},
		{name: "english_agent_4tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_4tool/req.json",
			want: 59},
		{name: "english_agent_5tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_5tool/req.json",
			want: 68},
		{name: "chinese_agent_1tool",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_agent_1tool/req.json",
			want: 603},
		{name: "chinese_agent_2tool",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_agent_2tool/req.json",
			want: 744},
		{name: "chinese_chat_short",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_chat_short/req.json",
			want: 368},
		{name: "chinese_chat_long",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_chat_long/req.json",
			want: 2415},
		{name: "english_chat_short",
			in:   "testdata/openai/responses/gpt-5-mini/english_chat_short/req.json",
			want: 288},
		{name: "english_chat_long",
			in:   "testdata/openai/responses/gpt-5-mini/english_chat_long/req.json",
			want: 1865},
		{name: "code_review_chat",
			in:   "testdata/openai/responses/gpt-5-mini/code_review_chat/req.json",
			want: 1294},
		{name: "code_writing_chat",
			in:   "testdata/openai/responses/gpt-5-mini/code_writing_chat/req.json",
			want: 1634},
		{name: "mixed_en_zh_chat",
			in:   "testdata/openai/responses/gpt-5-mini/mixed_en_zh_chat/req.json",
			want: 1071},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "responses",
				Model:         "gpt-5-mini",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 6, TotalTokens: 6},
				RequestBody:   data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official input=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.InputTokens, deviation(item.want, out.Usage.InputTokens))
		})
	}
}

func TestEstimate_OpenaiResponsesOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat",
			in:   "testdata/openai/responses/gpt-5-mini/agent_chat/resp.json",
			want: 119},
		{name: "chinese_chat",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_chat/resp.json",
			want: 1119},
		{name: "english_chat",
			in:   "testdata/openai/responses/gpt-5-mini/english_chat/resp.json",
			want: 995},
		{name: "chinese_agent",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_agent/resp.json",
			want: 351},
		{name: "english_agent",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent/resp.json",
			want: 2048},
		{name: "english_agent_0tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_0tool/resp.json",
			want: 72},
		{name: "english_agent_1tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_1tool/resp.json",
			want: 86},
		{name: "english_agent_2tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_2tool/resp.json",
			want: 65},
		{name: "english_agent_4tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_4tool/resp.json",
			want: 85},
		{name: "english_agent_5tool",
			in:   "testdata/openai/responses/gpt-5-mini/english_agent_5tool/resp.json",
			want: 66},
		{name: "chinese_agent_1tool",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_agent_1tool/resp.json",
			want: 1578},
		{name: "chinese_agent_2tool",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_agent_2tool/resp.json",
			want: 1768},
		{name: "chinese_chat_short",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_chat_short/resp.json",
			want: 1291},
		{name: "chinese_chat_long",
			in:   "testdata/openai/responses/gpt-5-mini/chinese_chat_long/resp.json",
			want: 2336},
		{name: "english_chat_short",
			in:   "testdata/openai/responses/gpt-5-mini/english_chat_short/resp.json",
			want: 1545},
		{name: "english_chat_long",
			in:   "testdata/openai/responses/gpt-5-mini/english_chat_long/resp.json",
			want: 2069},
		{name: "code_review_chat",
			in:   "testdata/openai/responses/gpt-5-mini/code_review_chat/resp.json",
			want: 813},
		{name: "code_writing_chat",
			in:   "testdata/openai/responses/gpt-5-mini/code_writing_chat/resp.json",
			want: 3152},
		{name: "mixed_en_zh_chat",
			in:   "testdata/openai/responses/gpt-5-mini/mixed_en_zh_chat/resp.json",
			want: 3302},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "responses",
				Model:         "gpt-5-mini",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
				ResponseBody:  data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			reasoningTokens := openAIResponsesReasoningTokens(t, data)
			visibleWant := item.want - reasoningTokens
			t.Logf("case %s, official output=%d, reasoning=%d, visible output=%d, estimated=%d, deviation=%.1f%%, visible deviation=%.1f%%",
				item.name, item.want, reasoningTokens, visibleWant, out.Usage.OutputTokens,
				deviation(item.want, out.Usage.OutputTokens), deviation(visibleWant, out.Usage.OutputTokens))
		})
	}
}

func TestEstimate_OpenaiChatCompletionsInput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/agent_chat/req.json",
			want: 1106},
		{name: "chinese_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_chat/req.json",
			want: 1406},
		{name: "english_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_chat/req.json",
			want: 1407},
		{name: "chinese_agent",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_agent/req.json",
			want: 1905},
		{name: "english_agent",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent/req.json",
			want: 2758},
		{name: "english_agent_0tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_0tool/req.json",
			want: 8},
		{name: "english_agent_1tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_1tool/req.json",
			want: 33},
		{name: "english_agent_2tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_2tool/req.json",
			want: 42},
		{name: "english_agent_4tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_4tool/req.json",
			want: 60},
		{name: "english_agent_5tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_5tool/req.json",
			want: 69},
		{name: "chinese_agent_1tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_agent_1tool/req.json",
			want: 586},
		{name: "chinese_agent_2tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_agent_2tool/req.json",
			want: 727},
		{name: "chinese_chat_short",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_chat_short/req.json",
			want: 365},
		{name: "chinese_chat_long",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_chat_long/req.json",
			want: 2402},
		{name: "english_chat_short",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_chat_short/req.json",
			want: 285},
		{name: "english_chat_long",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_chat_long/req.json",
			want: 1852},
		{name: "code_review_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/code_review_chat/req.json",
			want: 1289},
		{name: "code_writing_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/code_writing_chat/req.json",
			want: 1629},
		{name: "mixed_en_zh_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/mixed_en_zh_chat/req.json",
			want: 1066},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "chat.completions",
				Model:         "gpt-4o-mini",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 6, TotalTokens: 6},
				RequestBody:   data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official input=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.InputTokens, deviation(item.want, out.Usage.InputTokens))
		})
	}
}

func TestEstimate_OpenaiChatCompletionsOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "agent_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/agent_chat/resp.json",
			want: 280},
		{name: "chinese_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_chat/resp.json",
			want: 29},
		{name: "english_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_chat/resp.json",
			want: 307},
		{name: "chinese_agent",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_agent/resp.json",
			want: 328},
		{name: "english_agent",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent/resp.json",
			want: 496},
		{name: "english_agent_0tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_0tool/resp.json",
			want: 9},
		{name: "english_agent_1tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_1tool/resp.json",
			want: 10},
		{name: "english_agent_2tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_2tool/resp.json",
			want: 10},
		{name: "english_agent_4tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_4tool/resp.json",
			want: 10},
		{name: "english_agent_5tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_agent_5tool/resp.json",
			want: 10},
		{name: "chinese_agent_1tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_agent_1tool/resp.json",
			want: 206},
		{name: "chinese_agent_2tool",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_agent_2tool/resp.json",
			want: 199},
		{name: "chinese_chat_short",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_chat_short/resp.json",
			want: 258},
		{name: "chinese_chat_long",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/chinese_chat_long/resp.json",
			want: 538},
		{name: "english_chat_short",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_chat_short/resp.json",
			want: 384},
		{name: "english_chat_long",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/english_chat_long/resp.json",
			want: 545},
		{name: "code_review_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/code_review_chat/resp.json",
			want: 231},
		{name: "code_writing_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/code_writing_chat/resp.json",
			want: 1219},
		{name: "mixed_en_zh_chat",
			in:   "testdata/openai/chatCompletions/gpt-4o-mini/mixed_en_zh_chat/resp.json",
			want: 524},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			data, err := os.ReadFile(item.in)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}
			out := Estimate(cfg, Input{
				API:           "chat.completions",
				Model:         "gpt-4o-mini",
				UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
				ResponseBody:  data,
			})
			if out.Usage == nil {
				t.Fatalf("usage is nil")
			}
			t.Logf("case %s, official output=%d, estimated=%d, deviation=%.1f%%", item.name, item.want, out.Usage.OutputTokens, deviation(item.want, out.Usage.OutputTokens))
		})
	}
}

func openAIResponsesReasoningTokens(t *testing.T, data []byte) int {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}
	usage, ok := root["usage"].(map[string]any)
	if !ok {
		return 0
	}
	details, ok := usage["output_tokens_details"].(map[string]any)
	if !ok {
		return 0
	}
	v, ok := details["reasoning_tokens"].(float64)
	if !ok {
		return 0
	}
	return int(v)
}

func deviation(official, estimated int) float64 {
	if official == 0 {
		return 0
	}
	return float64(estimated-official) / float64(official) * 100
}
