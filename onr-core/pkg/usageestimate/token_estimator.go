package usageestimate

import (
	"math"
	"strings"
	"unicode"
)

type provider string

const (
	providerOpenAI provider = "openai"
	providerClaude provider = "claude"
	providerGemini provider = "gemini"
)

type multipliers struct {
	Word                   float64
	Number                 float64
	CJK                    float64
	Symbol                 float64
	MathSymbol             float64
	URLDelim               float64
	AtSign                 float64
	Emoji                  float64
	Newline                float64
	Space                  float64
	BasePad                int
	ToolsExist             int // base token estimate added when tools are present
	PerTool                int // additional token estimate per tool
	FunctionCallItem       int
	FunctionCallOutputItem int
	CustomToolCallItem     int
	CustomToolOutputItem   int
	ThinkingBlockInput     int
	ThinkingBlockOutput    int
}

var multipliersMap = map[provider]map[string]multipliers{
	providerGemini: {
		"default": {Word: 1.15, Number: 2.8, CJK: 0.68, Symbol: 0.38, MathSymbol: 1.05,
			URLDelim: 1.2, AtSign: 2.5, Emoji: 1.08, Newline: 1.15, Space: 0.2, BasePad: 0,
			ToolsExist: 0, PerTool: 0},
	},

	providerClaude: {
		"default": {Word: 1.05, Number: 1.63, CJK: 1.25, Symbol: 0.4, MathSymbol: 4.52,
			URLDelim: 1.26, AtSign: 2.82, Emoji: 2.6, Newline: 0.89, Space: 0.39, BasePad: 0,
			ToolsExist: 496, PerTool: 33},
		"default-output": {Word: 1.2, Number: 1.85, CJK: 1.35, Symbol: 0.52, MathSymbol: 4.52,
			URLDelim: 1.26, AtSign: 2.82, Emoji: 2.6, Newline: 1.0, Space: 0.45, BasePad: 24,
			ToolsExist: 0, PerTool: 0},
		"opus-4-7": {Word: 1.15, Number: 1.25, CJK: 0.99, Symbol: 0.25, MathSymbol: 3.56,
			URLDelim: 0.45, AtSign: 2.42, Emoji: 2.64, Newline: 1.10, Space: 0.29, BasePad: 0,
			ToolsExist: 0, PerTool: 39},
		"opus-4-7-output": {Word: 2.25, Number: 2.25, CJK: 1.44, Symbol: 1.88, MathSymbol: 1.88,
			URLDelim: 1.88, AtSign: 1.88, Emoji: 1.88, Newline: 1.72, Space: 1.72, BasePad: 0,
			ToolsExist: 0, PerTool: 0},
	},

	providerOpenAI: {
		"default": {Word: 1.02, Number: 1.55, CJK: 0.85, Symbol: 0.4, MathSymbol: 2.68,
			URLDelim: 1.0, AtSign: 2.0, Emoji: 2.12, Newline: 0.5, Space: 0.16, BasePad: 0,
			ToolsExist: 10, PerTool: 20,
			FunctionCallItem:       6,
			FunctionCallOutputItem: 12,
			CustomToolCallItem:     6,
			CustomToolOutputItem:   12,
			ThinkingBlockInput:     300,
			ThinkingBlockOutput:    0,
		},
		"default-output": {Word: 1.02, Number: 1.55, CJK: 0.85, Symbol: 0.4, MathSymbol: 2.68,
			URLDelim: 1.0, AtSign: 2.0, Emoji: 2.12, Newline: 0.5, Space: 0.16, BasePad: 12,
			ToolsExist: 0, PerTool: 0,
			FunctionCallItem:       6,
			FunctionCallOutputItem: 12,
			CustomToolCallItem:     6,
			CustomToolOutputItem:   12,
			ThinkingBlockInput:     300,
			ThinkingBlockOutput:    0,
		},
	},
}

func getMultipliers(modelName string, completion bool) multipliers {
	m := strings.ToLower(strings.TrimSpace(modelName))
	if strings.Contains(m, "claude") {
		if isAnthropic47Model(m) {
			if completion {
				return multipliersMap[providerClaude]["opus-4-7-output"]
			}
			return multipliersMap[providerClaude]["opus-4-7"]
		}
		if completion {
			return multipliersMap[providerClaude]["default-output"]
		}
		return multipliersMap[providerClaude]["default"]
	} else if strings.Contains(m, "gpt") {
		if completion {
			return multipliersMap[providerOpenAI]["default-output"]
		}
		return multipliersMap[providerOpenAI]["default"]
	} else if strings.Contains(m, "gemini") {
		return multipliersMap[providerGemini]["default"]
	}
	return multipliersMap[providerClaude]["default"]

}

func isAnthropic47Model(modelName string) bool {
	m := strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(m, "claude") && (strings.Contains(m, "4-7") || strings.Contains(m, "4.7"))
}

func EstimateTokenByModel(model string, ctx *tokenEstimateContext) int {
	if strings.TrimSpace(ctx.text) == "" && !hasEstimateOnlyCounts(ctx) {
		return 0
	}
	multipliers := getMultipliers(model, ctx.completion)
	return estimateToken(ctx, multipliers)

}

func hasEstimateOnlyCounts(ctx *tokenEstimateContext) bool {
	if ctx == nil {
		return false
	}
	return ctx.numTools != 0 ||
		ctx.numThinkingBlockInput != 0 ||
		ctx.numThinkingBlockOutput != 0 ||
		ctx.numFunctionCalls != 0 ||
		ctx.numFunctionCallOutputs != 0 ||
		ctx.numCustomToolCalls != 0 ||
		ctx.numCustomToolCallOutputs != 0
}

func estimateToken(ctx *tokenEstimateContext, m multipliers) int {
	var count float64

	type wordType int
	const (
		none wordType = iota
		latin
		number
	)
	cur := none
	runLen := 0

	flushRun := func() {
		if runLen == 0 {
			return
		}
		switch cur {
		case number:
			count += estimateNumberRun(runLen, m, ctx.completion)
		case latin:
			count += estimateLatinRun(runLen, m, ctx.completion)
		}
		runLen = 0
	}

	for _, r := range ctx.text {
		if unicode.IsSpace(r) {
			flushRun()
			cur = none
			if r == '\n' || r == '\t' {
				count += m.Newline
			} else {
				count += m.Space
			}
			continue
		}
		if isCJK(r) {
			flushRun()
			cur = none
			count += m.CJK
			continue
		}
		if isEmoji(r) {
			flushRun()
			cur = none
			count += m.Emoji
			continue
		}
		if isLatinOrNumber(r) {
			isNum := unicode.IsNumber(r)
			newType := latin
			if isNum {
				newType = number
			}
			if cur == none || cur != newType {
				flushRun()
				cur = newType
			}
			runLen++
			continue
		}

		flushRun()
		cur = none
		switch {
		case isMathSymbol(r):
			count += m.MathSymbol
		case r == '@':
			count += m.AtSign
		case isURLDelim(r):
			count += m.URLDelim
		default:
			count += m.Symbol
		}
	}
	flushRun()
	sum := int(math.Ceil(count))
	if ctx.completion {
		sum += completionBasePad(sum, m.BasePad)
	} else {
		sum += m.BasePad
	}
	if ctx.numTools != 0 { // add tool token estimate
		sum += m.ToolsExist + ctx.numTools*m.PerTool
	}
	sum += ctx.numFunctionCalls*m.FunctionCallItem +
		ctx.numFunctionCallOutputs*m.FunctionCallOutputItem +
		ctx.numCustomToolCalls*m.CustomToolCallItem +
		ctx.numCustomToolCallOutputs*m.CustomToolOutputItem +
		ctx.numThinkingBlockInput*m.ThinkingBlockInput +
		ctx.numThinkingBlockOutput*m.ThinkingBlockOutput

	return sum
}

func completionBasePad(sum, pad int) int {
	if pad <= 0 {
		return 0
	}
	if sum >= 25 {
		return pad
	}
	return 0
}

// estimateLatinRun keeps short words close to the legacy one-word estimate,
// while making long identifiers, code symbols and URL chunks scale with length.
// The input is expected to be a contiguous run of unicode letters/numbers that
// has already been split by whitespace and punctuation.
func estimateLatinRun(runeLen int, m multipliers, completion bool) float64 {
	if runeLen <= 0 {
		return 0
	}
	if runeLen <= 8 {
		return m.Word
	}
	if completion {
		return m.Word + float64(runeLen-8)/8.0*m.Word
	}
	return m.Word + float64(runeLen-8)/6.0*m.Word
}

// estimateNumberRun treats long numeric spans such as timestamps, IDs and
// decimal fragments as multiple token-like units instead of one flat segment.
func estimateNumberRun(runeLen int, m multipliers, completion bool) float64 {
	if runeLen <= 0 {
		return 0
	}
	if runeLen <= 4 {
		return m.Number
	}
	if completion {
		return m.Number + float64(runeLen-4)/4.0*m.Number
	}
	return m.Number + float64(runeLen-4)/3.0*m.Number
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7A3)
}

func isLatinOrNumber(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1F9FF) ||
		(r >= 0x2600 && r <= 0x26FF) ||
		(r >= 0x2700 && r <= 0x27BF) ||
		(r >= 0x1F600 && r <= 0x1F64F) ||
		(r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x1FA00 && r <= 0x1FAFF)
}

func isMathSymbol(r rune) bool {
	if r >= 0x2200 && r <= 0x22FF {
		return true
	}
	if r >= 0x2A00 && r <= 0x2AFF {
		return true
	}
	if r >= 0x1D400 && r <= 0x1D7FF {
		return true
	}
	// quick check for common symbols
	switch r {
	case '∑', '∫', '∂', '√', '∞', '≤', '≥', '≠', '≈', '±', '×', '÷':
		return true
	default:
		return false
	}
}

func isURLDelim(r rune) bool {
	switch r {
	case '/', ':', '?', '&', '=', ';', '#', '%':
		return true
	default:
		return false
	}
}
