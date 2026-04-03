package dslconfig

import (
	"fmt"
	"strings"
)

var supportedMatchAPIs = map[string]struct{}{
	"completions":                  {},
	"chat.completions":             {},
	"responses":                    {},
	"claude.messages":              {},
	"embeddings":                   {},
	"images.generations":           {},
	"images.edits":                 {},
	"audio.speech":                 {},
	"audio.transcriptions":         {},
	"audio.translations":           {},
	"gemini.generateContent":       {},
	"gemini.streamGenerateContent": {},
}

func validateProviderMatchAPIs(path, providerName string, routing ProviderRouting) error {
	for i, match := range routing.Matches {
		api := strings.TrimSpace(match.API)
		if api == "" {
			continue
		}
		if _, ok := supportedMatchAPIs[api]; ok {
			continue
		}
		return fmt.Errorf(
			"provider %q in %q: match[%d].api %q is unsupported",
			providerName,
			path,
			i,
			api,
		)
	}
	return nil
}
