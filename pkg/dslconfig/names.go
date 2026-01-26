package dslconfig

import (
	"strings"
	"unicode"
)

func NormalizeProviderName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ChannelTypeStringToProviderName converts a ChannelType.String() (e.g. "OpenAICompatible")
// into the provider config name (e.g. "openai-compatible").
func ChannelTypeStringToProviderName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var parts []string
	var cur []rune
	r := []rune(s)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		parts = append(parts, string(cur))
		cur = cur[:0]
	}
	for i := 0; i < len(r); i++ {
		ch := r[i]
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
			flush()
			continue
		}
		if len(cur) == 0 {
			cur = append(cur, ch)
			continue
		}
		prev := cur[len(cur)-1]
		var next rune
		if i+1 < len(r) {
			next = r[i+1]
		}
		isBoundary := unicode.IsUpper(ch) && (unicode.IsLower(prev) || (unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next)))
		if isBoundary {
			flush()
		}
		cur = append(cur, ch)
	}
	flush()

	out := make([]string, 0, len(parts))
	isAcronym := func(token string) bool {
		if token == "" {
			return false
		}
		for _, ch := range token {
			if !unicode.IsUpper(ch) {
				return false
			}
		}
		return true
	}
	for _, token := range parts {
		tokLower := strings.ToLower(token)
		if len(out) == 0 {
			out = append(out, tokLower)
			continue
		}
		if isAcronym(token) && len(token) <= 3 {
			out[len(out)-1] += tokLower
			continue
		}
		out = append(out, tokLower)
	}
	return strings.Join(out, "-")
}
