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
	Word       float64
	Number     float64
	CJK        float64
	Symbol     float64
	MathSymbol float64
	URLDelim   float64
	AtSign     float64
	Emoji      float64
	Newline    float64
	Space      float64
	BasePad    int
}

var multipliersMap = map[provider]multipliers{
	providerGemini: {Word: 1.15, Number: 2.8, CJK: 0.68, Symbol: 0.38, MathSymbol: 1.05, URLDelim: 1.2, AtSign: 2.5, Emoji: 1.08, Newline: 1.15, Space: 0.2, BasePad: 0},
	providerClaude: {Word: 1.13, Number: 1.63, CJK: 1.21, Symbol: 0.4, MathSymbol: 4.52, URLDelim: 1.26, AtSign: 2.82, Emoji: 2.6, Newline: 0.89, Space: 0.39, BasePad: 0},
	providerOpenAI: {Word: 1.02, Number: 1.55, CJK: 0.85, Symbol: 0.4, MathSymbol: 2.68, URLDelim: 1.0, AtSign: 2.0, Emoji: 2.12, Newline: 0.5, Space: 0.42, BasePad: 0},
}

func EstimateTokenByModel(model, text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "gemini"):
		return estimateToken(providerGemini, text)
	case strings.Contains(m, "claude"):
		return estimateToken(providerClaude, text)
	default:
		return estimateToken(providerOpenAI, text)
	}
}

func estimateToken(p provider, text string) int {
	m := multipliersMap[p]
	var count float64

	type wordType int
	const (
		none wordType = iota
		latin
		number
	)
	cur := none

	for _, r := range text {
		if unicode.IsSpace(r) {
			cur = none
			if r == '\n' || r == '\t' {
				count += m.Newline
			} else {
				count += m.Space
			}
			continue
		}
		if isCJK(r) {
			cur = none
			count += m.CJK
			continue
		}
		if isEmoji(r) {
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
				if newType == number {
					count += m.Number
				} else {
					count += m.Word
				}
				cur = newType
			}
			continue
		}

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
	return int(math.Ceil(count)) + m.BasePad
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
