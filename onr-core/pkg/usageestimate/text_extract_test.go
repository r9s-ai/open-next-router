package usageestimate

import (
	"strings"
	"testing"
)

func TestExtractResponseText_FallbackCollectsNestedText(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"level1": {
			"level2": [
				{"text": "a"},
				{"x": {"text": "b"}}
			]
		},
		"other": "ignore"
	}`)
	got := extractResponseText("unknown.api", body, 1024)
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Fatalf("got=%q want to contain a and b", got)
	}
}

func TestExtractRequestText_GeminiContents(t *testing.T) {
	t.Parallel()

	req := []byte(`{
		"contents":[
			{"parts":[{"text":"hello"},{"text":"world"}]}
		]
	}`)
	got := extractRequestText("gemini.generateContent", req, 1024)
	if !strings.Contains(got.text, "hello") || !strings.Contains(got.text, "world") {
		t.Fatalf("got=%q", got.text)
	}
}

func TestExtractResponseText_GeminiCandidates(t *testing.T) {
	t.Parallel()

	resp := []byte(`{
		"candidates":[
			{"content":{"parts":[{"text":"hello"},{"text":"world"}]}}
		]
	}`)
	got := extractResponseText("gemini.generateContent", resp, 1024)
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("got=%q", got)
	}
}

func TestOpenAIResponsesReasoningSummaryAvoidsHiddenBlockPad(t *testing.T) {
	t.Parallel()

	req := map[string]any{
		"input": []any{
			map[string]any{
				"type": "reasoning",
				"summary": []any{
					map[string]any{"type": "summary_text", "text": "visible reasoning"},
				},
			},
			map[string]any{
				"type":    "reasoning",
				"summary": []any{},
			},
		},
	}
	ctx := stringfyOpenaiResponsesRequest(req)
	if !strings.Contains(ctx.text, "visible reasoning") {
		t.Fatalf("text=%q want visible reasoning", ctx.text)
	}
	if got, want := ctx.numThinkingBlockInput, 1; got != want {
		t.Fatalf("numThinkingBlockInput=%d want=%d", got, want)
	}
}

func TestEstimateTokenByModel_CountsHiddenReasoningWithoutText(t *testing.T) {
	t.Parallel()

	got := EstimateTokenByModel("gpt-5.5", &tokenEstimateContext{
		numThinkingBlockInput: 2,
	})
	if got != 600 {
		t.Fatalf("tokens=%d want=600", got)
	}
}

func TestStringifyAny_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "string", in: "x", want: "x"},
		{name: "array", in: []any{"a", "b"}, want: "a\nb\n"},
		{name: "map_text", in: map[string]any{"text": "t"}, want: "t"},
		{name: "map_nested_text", in: map[string]any{"x": map[string]any{"text": "t"}}, want: "t\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringifyAny(tc.in)
			if got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}
