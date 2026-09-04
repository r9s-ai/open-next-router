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
	"claude.messages.count_tokens": {},
	"embeddings":                   {},
	"images.generations":           {},
	"images.edits":                 {},
	"audio.speech":                 {},
	"audio.transcriptions":         {},
	"audio.translations":           {},
	"videos.generations":           {},
	"videos.remix":                 {},
	"videos.list":                  {},
	"videos.get":                   {},
	"videos.content":               {},
	"videos.delete":                {},
	"gemini.generateContent":       {},
	"gemini.streamGenerateContent": {},
	"gemini.predictLongRunning":    {},
	"gemini.getOperation":          {},
	"gemini.videoContent":          {},
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
