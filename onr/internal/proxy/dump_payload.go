package proxy

import (
	"strings"
	"unicode/utf8"
)

// isBinaryDumpPayload decides whether dump content should be written as base64.
// It first respects Content-Type when present; if missing, it falls back to a
// small payload sniffing heuristic to avoid misclassifying textual SSE as binary.
func isBinaryDumpPayload(contentType string, payload []byte) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct != "" {
		return !strings.Contains(ct, "json") && !strings.HasPrefix(ct, "text/")
	}
	if len(payload) == 0 {
		return false
	}

	sample := payload
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	if !utf8.Valid(sample) {
		return true
	}

	control := 0
	for _, b := range sample {
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
			control++
		}
	}
	return control*20 > len(sample) // >5% control bytes.
}
