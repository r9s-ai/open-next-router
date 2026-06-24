package proxy

import (
	"strings"
)

func parseGeminiModelFromPath(path string) (model string, ok bool) {
	p := strings.TrimSpace(path)
	// /v1beta/models/{model}:{action}
	const prefix = "/v1beta/models/"
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(p, prefix)
	// rest: {model}:{action}
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	model = strings.TrimSpace(parts[0])
	if model == "" {
		return "", false
	}
	return model, true
}

func replaceGeminiModelInPath(pathWithQuery string, newModel string) (string, bool) {
	p := strings.TrimSpace(pathWithQuery)
	const prefix = "/v1beta/models/"
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(p, prefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", false
	}
	return prefix + strings.TrimSpace(newModel) + ":" + parts[1], true
}
