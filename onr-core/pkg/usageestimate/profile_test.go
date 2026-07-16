package usageestimate

import (
	"math"
	"testing"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

func mustCloseSourceTokenizer(t *testing.T, ctx *EstimateContext) *CloseSourceTokenizer {
	t.Helper()
	tokenizer, err := NewCloseSourceTokenizer(ctx)
	if err != nil {
		t.Fatalf("NewCloseSourceTokenizer: %v", err)
	}
	return tokenizer
}

func TestCloseSourceTokenizerReusesEncoding(t *testing.T) {
	enc, err := tiktoken.GetEncoding("o200k_base")
	if err != nil {
		t.Fatalf("GetEncoding: %v", err)
	}
	firstCtx := NewEstimateContext("gpt-5", apiResponses, EstimateInput)
	firstCtx.TokenEncoder = enc
	secondCtx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateOutput)
	secondCtx.TokenEncoder = enc
	first := mustCloseSourceTokenizer(t, firstCtx)
	second := mustCloseSourceTokenizer(t, secondCtx)
	if first.enc != second.enc {
		t.Fatal("expected close source tokenizers to reuse the caller-provided encoding")
	}
}

func TestCloseSourceTokenizerDefaultsToTextLenCounterWithoutEncoding(t *testing.T) {
	ctx := NewEstimateContext("gpt-5.5", apiResponses, EstimateInput)
	tokenizer := mustCloseSourceTokenizer(t, ctx)
	if tokenizer.enc != nil {
		t.Fatal("expected tokenizer without caller-provided encoding to keep enc nil")
	}
	tokenizer.profile = ClosedSourceProfile{
		TextCounter:    textCounterTiktoken,
		TextMultiplier: 1,
		OverheadWeights: map[OverHeadItemKind]int{
			ItemRoleAssistant: 6,
		},
	}
	ctx.AddOverHead(ItemRoleAssistant, 1)

	if got, want := tokenizer.CountToken("abcdefghijkl"), 9; got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestCloseSourceTokenizerTextLenCounterUsesLengthDividedByFour(t *testing.T) {
	ctx := NewEstimateContext("gpt-5.5", apiResponses, EstimateInput)
	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextCounter:     textCounterTextLen,
		TextMultiplier:  2,
		OverheadWeights: map[OverHeadItemKind]int{},
	}

	if got, want := tokenizer.CountToken("abcdefghijklmnop"), 8; got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestCloseSourceTokenizerUsesActualTextCounterBands(t *testing.T) {
	ctx := NewEstimateContext("gpt-5.5", apiResponses, EstimateInput)
	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextCounter:    textCounterTiktoken,
		TextMultiplier: 1,
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: {
				{MaxTokens: 0, Multiplier: 10},
			},
			textCounterTextLen: {
				{MaxTokens: 0, Multiplier: 2},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{},
	}

	if got, want := tokenizer.CountToken("abcdefgh"), 4; got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestClosedSourceProfileForAPIModelPrefersAPISpecificProfile(t *testing.T) {
	chatProfile := closedSourceProfileForAPIModel(apiChatCompletions, "gpt-5.5")
	assertProfileMatch(t, chatProfile, apiChatCompletions, "gpt-5.5")

	responsesProfile := closedSourceProfileForAPIModel(apiResponses, "gpt-5.5")
	assertProfileMatch(t, responsesProfile, apiResponses, "gpt-5.5")

	geminiProfile := closedSourceProfileForAPIModel(apiGeminiGenerateContent, "gemini-2.5-pro")
	assertProfileMatch(t, geminiProfile, apiGeminiGenerateContent, "gemini-2.5-pro")

	gemini3Profile := closedSourceProfileForAPIModel(apiGeminiGenerateContent, "gemini-3-flash-preview")
	assertProfileMatch(t, gemini3Profile, apiGeminiGenerateContent, "gemini-3-flash-preview")
}

func TestClosedSourceProfileForAPIModelMatchesCopiedClaudeProfiles(t *testing.T) {
	for _, model := range []string{
		"claude-opus-4-7",
		"claude-sonnet-5",
		"claude-sonnet-4-6",
	} {
		t.Run(model, func(t *testing.T) {
			profile := closedSourceProfileForAPIModel(apiMessages, model)
			assertProfileMatch(t, profile, apiMessages, model)
		})
	}
}

func TestClosedSourceProfileForAPIModelFallsBackByRoute(t *testing.T) {
	tests := []struct {
		name          string
		api           string
		model         string
		wantAPI       string
		wantModelPart string
	}{
		{
			name:          "chat completions",
			api:           apiChatCompletions,
			model:         "unknown-model",
			wantAPI:       apiChatCompletions,
			wantModelPart: "gpt-5.5",
		},
		{
			name:          "responses",
			api:           apiResponses,
			model:         "unknown-model",
			wantAPI:       apiResponses,
			wantModelPart: "gpt-5.5",
		},
		{
			name:          "claude messages",
			api:           apiMessages,
			model:         "claude-sonnet-4-7",
			wantAPI:       apiMessages,
			wantModelPart: "claude-opus-4-8",
		},
		{
			name:          "gemini 2 generate",
			api:           apiGeminiGenerateContent,
			model:         "gemini-2.0-flash",
			wantAPI:       apiGeminiGenerateContent,
			wantModelPart: "gemini-2.5-pro",
		},
		{
			name:          "gemini 3 stream",
			api:           apiGeminiStreamGenerateContent,
			model:         "gemini-3-pro-preview",
			wantAPI:       apiGeminiStreamGenerateContent,
			wantModelPart: "gemini-3-flash-preview",
		},
		{
			name:          "gemini default",
			api:           apiGeminiGenerateContent,
			model:         "unknown-model",
			wantAPI:       apiGeminiGenerateContent,
			wantModelPart: "gemini-2.5-pro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := closedSourceProfileForAPIModel(tt.api, tt.model)
			assertProfileMatch(t, profile, tt.wantAPI, tt.wantModelPart)
		})
	}
}

func TestClosedSourceProfileForAPIModelDoesNotUseModelOnlyMatch(t *testing.T) {
	profile := closedSourceProfileForAPIModel(apiResponses, "claude-opus-4-8")
	assertProfileMatch(t, profile, apiResponses, "gpt-5.5")
}

func assertProfileMatch(t *testing.T, profile ClosedSourceProfile, api, modelFragment string) {
	t.Helper()
	if got, want := profile.API, api; got != want {
		t.Fatalf("profile api=%q want=%q", got, want)
	}
	if len(profile.ModelContains) != 1 || profile.ModelContains[0] != modelFragment {
		t.Fatalf("profile model fragments=%v want [%q]", profile.ModelContains, modelFragment)
	}
}

func TestClosedSourceProfileTextMultiplierForBands(t *testing.T) {
	profile := ClosedSourceProfile{
		TextMultiplier: 1.65,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 10, Multiplier: 2.0},
			{MaxTokens: 20, Multiplier: 1.5},
			{MaxTokens: 0, Multiplier: 1.25},
		},
	}

	tests := []struct {
		textTokens int
		want       float64
	}{
		{textTokens: 0, want: 2.0},
		{textTokens: 10, want: 2.0},
		{textTokens: 11, want: 1.5},
		{textTokens: 20, want: 1.5},
		{textTokens: 21, want: 1.25},
	}

	for _, tt := range tests {
		got := profile.textMultiplierFor(tt.textTokens)
		if got != tt.want {
			t.Fatalf("textMultiplierFor(%d)=%v want %v", tt.textTokens, got, tt.want)
		}
	}
}

func TestClosedSourceProfileTextMultiplierForFallbacks(t *testing.T) {
	tests := []struct {
		name    string
		profile ClosedSourceProfile
		want    float64
	}{
		{
			name:    "fallback to scalar",
			profile: ClosedSourceProfile{TextMultiplier: 1.65},
			want:    1.65,
		},
		{
			name: "skip invalid band multiplier",
			profile: ClosedSourceProfile{
				TextMultiplier: 1.65,
				TextMultiplierBands: []TextMultiplierBand{
					{MaxTokens: 10, Multiplier: 0},
				},
			},
			want: 1.65,
		},
		{
			name:    "default to one",
			profile: ClosedSourceProfile{},
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.textMultiplierFor(5)
			if got != tt.want {
				t.Fatalf("textMultiplierFor=%v want %v", got, tt.want)
			}
		})
	}
}

func TestCloseSourceTokenizerCountTokenScalesTextOnly(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateInput)
	ctx.AddOverHead(ItemRoleAssistant, 2)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextMultiplier: 2,
		OverheadWeights: map[OverHeadItemKind]int{
			ItemRoleAssistant: 3,
		},
	}

	prompt := "hello world"
	textTokens, _ := tokenizer.countTextTokens(prompt)
	want := int(math.Round(float64(textTokens)*2)) + 6

	if got := tokenizer.CountToken(prompt); got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestCloseSourceTokenizerCountTokenSkipsHiddenReasoningForOutput(t *testing.T) {
	ctx := NewEstimateContext("gpt-5.5", apiResponses, EstimateOutput)
	ctx.AddOverHead(ItemRoleAssistant, 1)
	ctx.AddOverHead(ItemHiddenReasoningBlock, 1)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		OverheadWeights: map[OverHeadItemKind]int{
			ItemRoleAssistant:        6,
			ItemHiddenReasoningBlock: 300,
		},
	}

	prompt := "visible output"
	textTokens, _ := tokenizer.countTextTokens(prompt)
	want := int(math.Round(float64(textTokens+6) * 1.3))

	if got := tokenizer.CountToken(prompt); got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestCloseSourceTokenizerCountTokenUsesBandMultiplier(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateInput)
	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextMultiplier: 1,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 128, Multiplier: 3},
			{MaxTokens: 0, Multiplier: 2},
		},
		OverheadWeights: map[OverHeadItemKind]int{},
	}

	prompt := "hello"
	textTokens, _ := tokenizer.countTextTokens(prompt)
	want := int(math.Round(float64(textTokens) * 3))

	if got := tokenizer.CountToken(prompt); got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestCloseSourceTokenizerCountTokenUsesOutputTextMultiplier(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateOutput)
	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextMultiplier:       2,
		OutputTextMultiplier: 3,
		OverheadWeights:      map[OverHeadItemKind]int{},
	}

	prompt := "hello"
	textTokens, _ := tokenizer.countTextTokens(prompt)
	want := int(math.Round(float64(textTokens) * 3))

	if got := tokenizer.CountToken(prompt); got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}

func TestCloseSourceTokenizerCountTokenOutputFallsBackToInputMultiplier(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateOutput)
	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.profile = ClosedSourceProfile{
		TextMultiplier:  2,
		OverheadWeights: map[OverHeadItemKind]int{},
	}

	prompt := "hello"
	textTokens, _ := tokenizer.countTextTokens(prompt)
	want := int(math.Round(float64(textTokens) * 2))

	if got := tokenizer.CountToken(prompt); got != want {
		t.Fatalf("CountToken=%d want %d", got, want)
	}
}
