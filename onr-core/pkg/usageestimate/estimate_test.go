package usageestimate

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

func TestEstimate_WhenMissingUsage_EstimateBoth(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:   "chat.completions",
		Model: "gpt-4o-mini",
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"hello"}]
		}`),
		ResponseBody: []byte(`{
			"id":"x",
			"choices":[{"index":0,"message":{"role":"assistant","content":"world"}}]
		}`),
	})

	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage = %q, want %q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.TotalTokens <= 0 {
		t.Fatalf("total_tokens = %d, want > 0", out.Usage.TotalTokens)
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens = %d, want > 0", out.Usage.InputTokens)
	}
}

func TestEstimate_UsesProvidedRequestRootWithoutParsingBody(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:         "chat.completions",
		Model:       "gpt-4o-mini",
		RequestBody: []byte("{"),
		RequestRoot: map[string]any{
			"messages": []any{
				map[string]any{"role": "user", "content": "hello"},
			},
		},
		ResponseBody: []byte(`{
			"id":"x",
			"choices":[{"index":0,"message":{"role":"assistant","content":"world"}}]
		}`),
	})

	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage = %q, want %q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens = %d, want > 0", out.Usage.InputTokens)
	}
}

func TestEstimate_WhenUpstreamUsagePresent_Upstream(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 12 {
		t.Fatalf("usage total_tokens = %#v, want 12", out.Usage)
	}
}

func TestEstimate_WhenUpstreamFactsPresent_PreservesFactLevelFields(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:   "claude.messages",
		Model: "claude-haiku-4-5",
		UpstreamUsage: &dslconfig.Usage{
			InputTokens:  10,
			OutputTokens: 2,
			TotalTokens:  12,
			FlatFields: map[string]any{
				"cache_write_ttl_5m_tokens": 6802,
			},
			DebugFacts: []dslconfig.UsageFact{
				{
					Dimension: "cache_write",
					Unit:      "token",
					Quantity:  6802,
					Attributes: map[string]string{
						"ttl": "5m",
					},
				},
			},
		},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if got, want := out.Usage.FlatFields["cache_write_ttl_5m_tokens"], 6802; got != want {
		t.Fatalf("flat field = %#v, want %d", got, want)
	}
	if len(out.Usage.DebugFacts) != 1 {
		t.Fatalf("debug facts len = %d, want 1", len(out.Usage.DebugFacts))
	}
	if out.Usage.DebugFacts[0].Dimension != "cache_write" {
		t.Fatalf("debug fact dimension = %q, want cache_write", out.Usage.DebugFacts[0].Dimension)
	}
}

func TestEstimate_NormalizeTotalTokens(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 11, OutputTokens: 9, TotalTokens: 0},
	})

	if out.Stage != StageUpstream {
		t.Fatalf("stage = %q, want %q", out.Stage, StageUpstream)
	}
	if out.Usage == nil || out.Usage.TotalTokens != 20 {
		t.Fatalf("total_tokens = %v, want 20", out.Usage)
	}
}

func TestEstimate_WhenAllZeroUsage_Estimates(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 0, TotalTokens: 0},
		RequestBody:   []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
		ResponseBody:  []byte(`{"choices":[{"message":{"role":"assistant","content":"world"}}]}`),
	})
	if out.Stage != StageEstimateBoth {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateBoth)
	}
	if out.Usage == nil || out.Usage.TotalTokens <= 0 {
		t.Fatalf("expected estimated usage, got %#v", out.Usage)
	}
}

func TestEstimate_WhenUpstreamMissingOutputTokens_EstimateCompletion(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	sse := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		StreamTail:    []byte(sse),
	})
	if out.Stage != StageEstimateCompletion {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateCompletion)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.InputTokens != 6 {
		t.Fatalf("input_tokens=%d want=6", out.Usage.InputTokens)
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatalf("output_tokens=%d want > 0", out.Usage.OutputTokens)
	}
	if out.Usage.TotalTokens != out.Usage.InputTokens+out.Usage.OutputTokens {
		t.Fatalf("total_tokens=%d want=%d", out.Usage.TotalTokens, out.Usage.InputTokens+out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenEstimatingMissingScalarFields_DoesNotSynthesizeFactLevelFields(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	sse := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	out := Estimate(cfg, Input{
		API:   "claude.messages",
		Model: "claude-haiku-4-5",
		UpstreamUsage: &dslconfig.Usage{
			InputTokens:  6,
			OutputTokens: 0,
			TotalTokens:  6,
			FlatFields: map[string]any{
				"cache_write_ttl_5m_tokens": 6802,
			},
			DebugFacts: []dslconfig.UsageFact{
				{
					Dimension: "cache_write",
					Unit:      "token",
					Quantity:  6802,
					Attributes: map[string]string{
						"ttl": "5m",
					},
				},
			},
		},
		StreamTail: []byte(sse),
	})
	if out.Stage != StageEstimateCompletion {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimateCompletion)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatalf("output_tokens=%d want > 0", out.Usage.OutputTokens)
	}
	if got, want := out.Usage.FlatFields["cache_write_ttl_5m_tokens"], 6802; got != want {
		t.Fatalf("flat field = %#v, want %d", got, want)
	}
	if len(out.Usage.FlatFields) != 1 {
		t.Fatalf("flat fields len = %d, want 1", len(out.Usage.FlatFields))
	}
	if len(out.Usage.DebugFacts) != 1 {
		t.Fatalf("debug facts len = %d, want 1", len(out.Usage.DebugFacts))
	}
	if out.Usage.DebugFacts[0].Dimension != "cache_write" {
		t.Fatalf("debug fact dimension = %q, want cache_write", out.Usage.DebugFacts[0].Dimension)
	}
}

func TestEstimate_WhenUpstreamMissingInputTokens_EstimatePrompt(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "chat.completions",
		Model:         "gpt-4o-mini",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 0, OutputTokens: 8, TotalTokens: 8},
		RequestBody:   []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
	})
	if out.Stage != StageEstimatePrompt {
		t.Fatalf("stage=%q want=%q", out.Stage, StageEstimatePrompt)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens != 8 {
		t.Fatalf("output_tokens=%d want=8", out.Usage.OutputTokens)
	}
	if out.Usage.InputTokens <= 0 {
		t.Fatalf("input_tokens=%d want > 0", out.Usage.InputTokens)
	}
	if out.Usage.TotalTokens != out.Usage.InputTokens+out.Usage.OutputTokens {
		t.Fatalf("total_tokens=%d want=%d", out.Usage.TotalTokens, out.Usage.InputTokens+out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenMissingOutputTokensButNoText_DontEstimateCompletion(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		StreamTail:    []byte("data: [DONE]\n\n"),
	})
	if out.Stage != StageUpstream {
		t.Fatalf("stage=%q want=%q", out.Stage, StageUpstream)
	}
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.OutputTokens != 0 {
		t.Fatalf("output_tokens=%d want=0", out.Usage.OutputTokens)
	}
}

func TestEstimate_WhenEstimationDisabled_ReturnsNilOnMissing(t *testing.T) {
	cfg := &Config{
		Enabled:                   true,
		EstimateWhenMissingOrZero: false,
		Strategy:                  "heuristic",
		MaxRequestBytes:           1024,
		MaxResponseBytes:          1024,
		MaxStreamCollectBytes:     1024,
		APIs:                      []string{"chat.completions"},
	}

	out := Estimate(cfg, Input{
		API:   "chat.completions",
		Model: "gpt-4o-mini",
	})
	if out.Stage != "" || out.Usage != nil {
		t.Fatalf("expected empty output, got stage=%q usage=%#v", out.Stage, out.Usage)
	}
}

// Test Anthropic SSE extraction and token estimation. This test is for development use only.
func TestEstimate_AnthropicStreamOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)
	streamTail := []byte(``)
	esc := `
用户指出可能是 tool_use 而不是文本的问题。我已经看到了：
1. 'meta.StreamText' 在 native_passthrough.go 中被追加文本内容
2. 注释说 "tool_use input_json_delta is not appended to StreamText"

这很关键！说明 tool_use 内容**不会**被添加到 StreamText。

我需要进一步排查：
1. 'onrusageestimate.Estimate()' 函数如何根据 StreamText 和 ResponseBody 来估计 completion_tokens
2. 当是 tool_use 输出时，ResponseBody 和 StreamText 的内容是什么
3. 估计逻辑是否对 tool_use 有特殊处理

我应该读：
1. 'relay/internal/onrusageestimate' 包中的估计逻辑
2. 确认 tool_use 时 ResponseBody 和 StreamText 的填充情况

初步判断：如果 tool_use 不被追加到 StreamText，且 ResponseBody 可能为空（因为是流式），那么估计器没有足够的输出内容来计算 completion_tokens，导致为 0。基于目前的发现，我需要读以下函数来完整判断：

1. **'onrusageestimate.Estimate()' 函数** - 看它如何从 StreamText 和 ResponseBody 来估计 completion_tokens
2. **native_passthrough.go 中的 tool_use 处理部分** - 确认 tool_use 块是否被正确存储

**初步判断：**
根据注释 '// tool_use input_json_delta is not appended to StreamText'，我怀疑问题根源是：
- 当输出是 'tool_use' 块而非文本时，**tool_use 内容没有被追加到 'meta.StreamText'**
- 而流式响应的 'meta.ResponseBody' 可能为空（因为是流式，没有完整响应体）
- 导致估计器接收到的 'StreamText' 为空，无法估算 'completion_tokens'，最终为 0

让我读这两个关键位置确认：read_file{"path": "relay/internal/channel/claudemessages/native_passthrough.go", "start_line": 210, "end_line": 260}search_code{"pattern": "func Estimate", "path": "relay/internal/onrusageestimate"}`
	payload, err := json.Marshal(map[string]string{"text": esc}) // Escape special characters.
	if err == nil {
		streamTail = []byte("data: " + string(payload) + "\n\n")
	} else {
		streamTail = []byte("data: \"\"\n\n")
	}
	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6}, //in 2342 out785
		StreamTail:    streamTail,
	})
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	t.Logf("official output=785, estimated=%d", out.Usage.OutputTokens)
}

func TestExtractStreamText_ChatCompletionsDelta(t *testing.T) {
	t.Parallel()

	sse := strings.Join([]string{
		`data: {"id":"x","choices":[{"delta":{"content":"hel"}}]}`,
		"",
		`data: {"id":"x","choices":[{"delta":{"content":"lo"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	got := extractStreamText("chat.completions", []byte(sse), 1024)
	if strings.ReplaceAll(got, "\n", "") != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}

func Test_stringifyAnthropicRequest(t *testing.T) {
	rawBody := []byte(`{
  "model":"claude-haiku-4-5",
 "max_tokens": 2048,
  "thinking": {
    "type": "enabled",
    "budget_tokens": 1024
  },
  "system": [
    {
      "type": "text",
      "text": "你是一个资深 code agent，正在一个 Go relay 服务仓库中帮助用户排查计费 token 估计问题。你可以使用工具读取文件、搜索代码和查看 git diff。回答使用中文，但保留代码标识符、文件路径、函数名和变量名的英文原文。排查时先基于证据，不要臆测；需要更多上下文时调用工具。"
    }
  ],
  "tools": [
    {
      "name": "read_file",
      "description": "Read a repository file and optionally return a selected line range.",
      "input_schema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "Repository-relative file path."
          },
          "start_line": {
            "type": "integer",
            "description": "1-based start line."
          },
          "end_line": {
            "type": "integer",
            "description": "1-based end line."
          }
        },
        "required": ["path"]
      }
    },
    {
      "name": "search_code",
      "description": "Search repository code with a ripgrep-compatible pattern and return matching file paths and line snippets.",
      "input_schema": {
        "type": "object",
        "properties": {
          "pattern": {
            "type": "string",
            "description": "Search pattern."
          },
          "path": {
            "type": "string",
            "description": "Optional repository-relative directory."
          }
        },
        "required": ["pattern"]
      }
    },
    {
      "name": "show_diff",
      "description": "Show the current git diff for selected files.",
      "input_schema": {
        "type": "object",
        "properties": {
          "paths": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "description": "Repository-relative file paths. Empty or omitted means all changed files."
          }
        }
      }
    }
  ],
   "messages": [
    {
      "role": "user",
      "content": "请帮我查一下上海今天的天气，然后用中文总结。"
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "我需要先调用天气工具获取实时数据。"
        },
        {
          "type": "thinking",
          "thinking": "用户想知道上海今天的天气，并要求中文总结。应该调用 get_weather 工具，地点为 Shanghai。"
        },
        {
          "type": "redacted_thinking",
          "data": "EmUCDCkIAxgCIkB..."
        },
        {
          "type": "tool_use",
          "id": "toolu_01ABC",
          "name": "get_weather",
          "input": {
            "city": "Shanghai",
            "country": "CN",
            "unit": "celsius"
          }
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_01ABC",
          "content": "上海今天多云，气温 18-24 摄氏度，东南风 3 级。"
        }
      ]
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "上海今天以多云为主，气温大约 18 到 24 摄氏度，体感较舒适。"
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "再判断一下是否适合夜跑。"
        },
        {
          "type": "tool_result",
          "tool_use_id": "toolu_02DEF",
          "content": [
            {
              "type": "text",
              "text": "空气质量良好，PM2.5 为 22。"
            },
            {
              "type": "text",
              "text": "夜间降雨概率 10%。"
            }
          ]
        }
      ]
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "tool_use",
          "id": "toolu_03XYZ",
          "name": "calculate_running_score",
          "input": {
            "temperature_c": 21,
            "rain_probability": 0.1,
            "pm25": 22,
            "wind_level": 3
          }
        },
        {
          "type": "text",
          "text": "我会结合气温、降雨概率、空气质量和风力判断。"
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_03XYZ",
          "is_error": false,
          "content": {
            "score": 86,
            "level": "good",
            "reason": "气温适中，空气质量良好，降雨概率低。"
          }
        }
      ]
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "thinking",
          "thinking": "工具返回 score=86，level=good。应该给出适合夜跑的结论，同时提醒补水和注意风。"
        },
        {
          "type": "text",
          "text": "今晚整体适合夜跑。建议选择常规强度，注意补水；如果体感风较明显，可以减少高强度间歇。"
        }
      ]
    }
  ]
}`)
	var req map[string]any
	if err := json.Unmarshal(rawBody, &req); err != nil {
		t.Fatal(err.Error())
	}
	ctx := stringifyAnthropicRequest(req)
	if ctx.text == "" {
		t.Fatalf("expected normal s,but get \"\"")
	}
	if ctx.numTools != 3 {
		t.Fatalf("expected num of tools is 3 s,but get %d\"\"", ctx.numTools)
	}
	t.Log(ctx.text)

}

func Test_stringifyOpenaiResponsesRequest(t *testing.T) {

	rawBody, err := os.ReadFile("testdata/openai/responses/stringfy/test_stringfyRequset.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	var req map[string]any
	if err := json.Unmarshal(rawBody, &req); err != nil {
		t.Fatal(err.Error())
	}
	ctx := stringfyOpenaiResponsesRequest(req)
	if ctx.text == "" {
		t.Fatalf("expected normal s,but get \"\"")
	}
	if ctx.numTools != 4 {
		t.Fatalf("expected num of tools is 4 s,but get %d\"\"", ctx.numTools)
	}
	t.Log(ctx.text)

}

func Test_stringifyOpenaiResponsesResponses(t *testing.T) {

	rawBody, err := os.ReadFile("testdata/openai/responses/stringfy/test_stringfyResponse.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	s := extractResponseText("responses", rawBody, -1)
	if s == "" {
		t.Fatalf("expected normal s,but get \"\"")
	}
	t.Log(s)

}

func Test_stringifyOpenaiChatCompletionsRequest(t *testing.T) {
	rawBody, err := os.ReadFile("testdata/openai/chatCompletions/stringfy/test_stringfyRequset.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	var req map[string]any
	if err := json.Unmarshal(rawBody, &req); err != nil {
		t.Fatal(err.Error())
	}
	ctx := stringfyOpenaiChatCompletionsRequest(req)
	if ctx.text == "" {
		t.Fatalf("expected normal s,but get \"\"")
	}
	if ctx.numTools != 2 {
		t.Fatalf("expected num of tools is 2 s,but get %d\"\"", ctx.numTools)
	}
	for _, want := range []string{
		"Analyze these two charts",
		"get_weather",
		"Beijing",
		"search_database",
		"historical weather Beijing Shanghai May",
	} {
		if !strings.Contains(ctx.text, want) {
			t.Fatalf("expected request text to contain %q, got %q", want, ctx.text)
		}
	}
	t.Log(ctx.text)
}

func Test_stringifyOpenaiChatCompletionsResponses(t *testing.T) {
	rawBody, err := os.ReadFile("testdata/openai/chatCompletions/stringfy/test_stringfyResponse.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	s := extractResponseText("chat.completions", rawBody, -1)
	if s == "" {
		t.Fatalf("expected normal s,but get \"\"")
	}
	for _, want := range []string{
		"综合分析报告",
		"get_weather",
		"Beijing",
		"Legacy text choice fallback",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("expected response text to contain %q, got %q", want, s)
		}
	}
	t.Log(s)
}
