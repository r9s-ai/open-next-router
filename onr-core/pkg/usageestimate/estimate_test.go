package usageestimate

import (
	"encoding/json"
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

// 测试anthropic的请求token预估，该测试仅仅开发使用，不保证功能测试正确性
func TestEstimate_AnthropicInput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)
	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		RequestBody: []byte(`{
  "model":"claude-haiku-4-5",
 "max_tokens": 2048,
  "thinking": {
    "type": "enabled",
    "budget_tokens": 1024
  },
  "stream":true,
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
      "content": "我们线上有个问题：Anthropic /v1/messages 流式响应内容正常，但有时 billing 里 output_tokens 是 0。请帮我排查。先不要改代码，先看 token 估计相关逻辑。"
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "我会先找 'output_tokens'、'stream_text' 和估计逻辑的入口，确认缺失上游 usage 时是否会从流式文本或 tool_use 内容估计。"
        },
        {
          "type": "tool_use",
          "id": "toolu_search_estimate_01",
          "name": "search_code",
          "input": {
            "pattern": "ensureEstimatedUsageIfNeeded|StreamText|UpstreamCompletionTokens",
            "path": "relay/internal"
          }
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_search_estimate_01",
          "content": "relay/internal/billing/extractor/configfile_extractor.go:55: ensureEstimatedUsageIfNeeded(ctx)\nrelay/internal/billing/extractor/configfile_extractor.go:338: func ensureEstimatedUsageIfNeeded(ctx *gin.Context) {\nrelay/internal/billing/extractor/configfile_extractor.go:390: StreamText: ctx.GetString(meta.StreamText),\nrelay/internal/channel/claudemessages/native_passthrough.go:438: c.Set(meta.UpstreamCompletionTokensEstimated, true)\nrelay/internal/meta/key.go:41: StreamText = \"stream_text\"\nrelay/internal/meta/key.go:76: UpstreamCompletionTokens = \"upstream_completion_tokens\""
        }
      ]
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "入口在 'configfile_extractor.go'，我需要看 'ensureEstimatedUsageIfNeeded' 如何构造估计输入，以及 stream 文本在哪里写入。"
        },
        {
          "type": "tool_use",
          "id": "toolu_read_configfile_01",
          "name": "read_file",
          "input": {
            "path": "relay/internal/billing/extractor/configfile_extractor.go",
            "start_line": 338,
            "end_line": 430
          }
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_read_configfile_01",
          "content": "func ensureEstimatedUsageIfNeeded(ctx *gin.Context) {\n    if ctx == nil {\n        return\n    }\n    prompt := ctx.GetInt(meta.UpstreamPromptTokens)\n    completion := ctx.GetInt(meta.UpstreamCompletionTokens)\n    total := ctx.GetInt(meta.UpstreamTotalTokens)\n    if prompt > 0 && completion > 0 {\n        return\n    }\n\n    cfg := &onrusageestimate.Config{}\n    onrusageestimate.ApplyDefaults(cfg)\n\n    in := onrusageestimate.Input{\n        Model: ctx.GetString(meta.RequestModel),\n        PromptTokens: prompt,\n        CompletionTokens: completion,\n        TotalTokens: total,\n        RequestBody: ctx.GetString(meta.RequestBody),\n        StreamText: ctx.GetString(meta.StreamText),\n        ResponseBody: ctx.GetString(meta.ResponseBody),\n    }\n    out := onrusageestimate.Estimate(cfg, in)\n    if out.PromptTokens > 0 {\n        ctx.Set(meta.UpstreamPromptTokens, out.PromptTokens)\n    }\n    if out.CompletionTokens > 0 {\n        ctx.Set(meta.UpstreamCompletionTokens, out.CompletionTokens)\n    }\n    if out.TotalTokens > 0 {\n        ctx.Set(meta.UpstreamTotalTokens, out.TotalTokens)\n    }\n}"
        }
      ]
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "这里的早退条件是 'prompt > 0 && completion > 0'，所以只要 'completion == 0' 仍会进入估计。下一步要确认 'meta.StreamText' 和 'meta.ResponseBody' 是否在 Anthropic stream/tool_use 场景被正确填充。"
        },
        {
          "type": "tool_use",
          "id": "toolu_search_streamtext_01",
          "name": "search_code",
          "input": {
            "pattern": "Set\\(meta\\.StreamText|StreamText",
            "path": "relay/internal/channel"
          }
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_search_streamtext_01",
          "content": "relay/internal/channel/claudemessages/native_passthrough.go:220: current := c.GetString(meta.StreamText)\nrelay/internal/channel/claudemessages/native_passthrough.go:224: c.Set(meta.StreamText, current + text)\nrelay/internal/channel/claudemessages/native_passthrough.go:246: // tool_use input_json_delta is not appended to StreamText\nrelay/internal/channel/adaptor/configfile/response_phase.go:118: c.Set(meta.ResponseBody, bodyString)"
        }
      ]
    },
    {
      "role": "user",
      "content": "请继续排查：如果输出是 'tool_use'而不是文本，'completion_tokens' 为 0 的原因可能是什么？请先指出你还要读哪个函数，然后给出你当前的初步判断。"
    }
  ]
}
`),
	})
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
	if out.Usage.InputTokens == 0 {
		t.Fatalf("output_tokens=%d", out.Usage.InputTokens)
	}
}

// 测试anthropic的响应token预估，该测试仅仅开发使用，不保证功能测试正确性
func TestEstimate_AnthropicOutput(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)
	out := Estimate(cfg, Input{
		API:           "claude.messages",
		Model:         "claude-3-5-sonnet",
		UpstreamUsage: &dslconfig.Usage{InputTokens: 6, OutputTokens: 0, TotalTokens: 6},
		ResponseBody: []byte(`{
		    "model": "claude-haiku-4-5-20251001",
    "id": "msg_01PS8Kg2hMsvMzMKDB9RYqD9",
    "type": "message",
    "role": "assistant",
	 "content":[
	    {
            "type": "thinking",
            "thinking": "用户问：如果输出是 tool_use 而不是文本，completion_tokens 为 0 的原因可能是什么？\n\n我需要：\n1. 看 native_passthrough.go 中 tool_use 相关的处理逻辑\n2. 看 onrusageestimate 的估计逻辑，特别是 tool_use 场景\n3. 理解为什么 tool_use 时 completion_tokens 会为 0\n\n根据搜索结果显示：\n- Line 246: '// tool_use input_json_delta is not appended to StreamText'\n- 这说明 tool_use input_json_delta 不会被追加到 StreamText\n\n让我先读一下 native_passthrough.go 中关于 tool_use 和 completion_tokens 的处理，然后再看 onrusageestimate 的估计逻辑。",
            "signature": "ErcGCmMIDRgCKkBXN98wrl0k1p98sEWAyQK5bUGwwPXjTMTvUmRIWYQsjFOqb63J0UvCGwgX/FXp+UPX0kWqcZEkD+PeIxs3ydDpMhljbGF1ZGUtaGFpa3UtNC01LTIwMjUxMDAxOAASDBHxQoqBr8stY+UzERoM24S+W0jYL/FBPmjTIjCF6a8wp4gWyTuyWFNfYpSVrVgXDPNppBseJswscGyg3ficmuY8tnNFhZ5VF/jZswsqgQXab8qhxGjzapxWa4wsiWN4ndkE8/YGOqh+vHVVSDi9stNVw19Ey1merMLCFOqHwv8b+qnXWXMOYuqbbg/ahT/x3ewqgMYJyYYkbU/XSSywsoHBFNDfz++PcSR7cbw41lF7Stj7BtqgzJmvDcGvHIw+YsQZpGOkpMKL90ymNDBGbRv7H6PmXJVA+3BvOAwTLnyZa/zXLci3ghhnQhebgcexICivsO052a3bm6ZtFLyjYinmHm8h5P1Ntt377+bJyiiAwTjOjn0Ryrkx4Xk6jC9gapq0VqlEyx5s8t+IIK8m5N9peHbsQMd1NFlSeAAg9Ax62EoX5nj5A5SJaZP5C7DjV6/LGtz36uEUac06avH6t7TQK8cNIalKXoqKvJcgSBhXGsOtTi04WiG88dOWQXpf1z+5hUoZ9ZkgiGuFNWouXjQZFZlByUe0u2oX0fZaM1ZWu7lL636VT50Bx7d6W8nEAHxMbJGjVASn4DtfpVzw6y7SBAfU2wAquhGnEAhAPTXt9fpzimaVKlZd4FW2eybNH7GW6IILdRXtX9MMVPkQfBMn43vGYCmuaeUDJcAZe3EXHOokuimLE16GDT/VbqOh2wCX+L7Ekp3eA5Hcce64/TJc6ERVmRRKtaSYJLqWcq/JOfwJLqpvnNk1a82SRD9mAir2s1c/Qp0APBZ84Ct/8AMQOxY0/JhjEt3cNxsFtP+KIoQyjeUOtD8jhkg7Bt76DU4U7C+sgUyOqFxeAdYn5mv4WT8PbdMTAPBorA3zJZBamSSh5ihFlVhx3YXa/5iiPh8Nm9EPIAncpiMySqW3di6G9g06FXHg1ZeLj4LeDiBD4yvyrHGzpMJz8FoYx2DotRgB"
        },
		  {
            "type": "text",
            "text": "我需要先确认以下关键路径。根据搜索结果，有个重要提示：'tool_use input_json_delta is not appended to StreamText'。\n\n**我先指出需要读的函数：**\n\n1. 'relay/internal/channel/claudemessages/native_passthrough.go' 中处理 tool_use 的部分（特别是围绕 line 220-246 的逻辑）\n2. 'onrusageestimate.Estimate()' 函数的实现（看如何估计 CompletionTokens，特别是 tool_use 场景）\n3. 检查 Anthropic stream 响应中是否包含 usage 字段、何时会丢失\n\n让我先读这些关键位置："
        },
        {
            "type": "tool_use",
            "id": "toolu_0126msd1pN8y1UTwF3Aovp8p",
            "name": "read_file",
            "input": {
                "path": "relay/internal/channel/claudemessages/native_passthrough.go",
                "start_line": 210,
                "end_line": 280
            },
            "caller": {
                "type": "direct"
            }
        },
        {
            "type": "tool_use",
            "id": "toolu_01PdC19Dg2GTcDYVp6yd44yR",
            "name": "search_code",
            "input": {
                "pattern": "func Estimate",
                "path": "relay/internal/billing"
            },
            "caller": {
                "type": "direct"
            }
        }
	 ],
	  "stop_reason": "tool_use",
    "stop_sequence": null,
    "stop_details": null,
    "usage": {
        "input_tokens": 2342,
        "cache_creation_input_tokens": 0,
        "cache_read_input_tokens": 0,
        "cache_creation": {
            "ephemeral_5m_input_tokens": 0,
            "ephemeral_1h_input_tokens": 0
        },
        "output_tokens": 612,
        "service_tier": "standard",
        "inference_geo": "not_available"
    }
		
		}`),
	})
	if out.Usage == nil {
		t.Fatalf("usage is nil")
	}
}

// 测试anthropic的sse提取和token预估，该测试仅仅开发使用，不保证功能测试正确性
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
	payload, err := json.Marshal(map[string]string{"text": esc}) //转义特殊字符
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
	t.Logf("anthropic stream out token = %d expected 785", out.Usage.OutputTokens)
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
	s := stringifyAnthropicRequest(req)
	if s == "" {
		t.Fatalf("expected normal s,but get \"\"")
	}
	t.Log(s)

}
