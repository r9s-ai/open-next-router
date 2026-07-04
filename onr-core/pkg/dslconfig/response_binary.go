package dslconfig

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// audioContentTypeByFormat maps audio format names to MIME types for
// resp_content_type kind=audio. Unknown formats fall back to audio/mpeg.
var audioContentTypeByFormat = map[string]string{
	"mp3":  "audio/mpeg",
	"wav":  "audio/wav",
	"flac": "audio/flac",
	"pcm":  "audio/pcm",
	"aac":  "audio/aac",
	"opus": "audio/opus",
	"ogg":  "audio/ogg",
}

// AudioContentTypeForFormat returns the MIME type for one audio format name.
func AudioContentTypeForFormat(format string) string {
	if ct, ok := audioContentTypeByFormat[strings.ToLower(strings.TrimSpace(format))]; ok {
		return ct
	}
	return "audio/mpeg"
}

func decodeBinaryField(value string, decode string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(decode)) {
	case "hex":
		return hex.DecodeString(value)
	case "base64":
		return base64.StdEncoding.DecodeString(value)
	default:
		return nil, fmt.Errorf("unsupported decode mode %q", decode)
	}
}

// ExtractBody requires a non-nil rule; root is the parsed upstream response
// JSON object. It returns the decoded binary body.
func (r *RespBodyExtractRule) ExtractBody(root map[string]any) ([]byte, error) {
	value := jsonutil.GetStringByPath(root, r.Path)
	if value == "" {
		return nil, fmt.Errorf("resp_body_extract: no string value at %s", r.Path)
	}
	data, err := decodeBinaryField(value, r.Decode)
	if err != nil {
		return nil, fmt.Errorf("resp_body_extract: decode %s failed: %w", r.Decode, err)
	}
	return data, nil
}

// ResolveContentType requires a non-nil rule; root may be nil. It resolves the
// downstream Content-Type from FromPath, then Default, then the fallback format.
// fallbackFormat may be empty; the final fallback is mp3.
func (r *RespContentTypeRule) ResolveContentType(root map[string]any, fallbackFormat string) string {
	format := ""
	if r.FromPath != "" && root != nil {
		format = jsonutil.GetStringByPath(root, r.FromPath)
	}
	if format == "" {
		format = r.Default
	}
	if format == "" {
		format = fallbackFormat
	}
	if format == "" {
		format = "mp3"
	}
	switch strings.ToLower(strings.TrimSpace(r.Kind)) {
	case "audio":
		return AudioContentTypeForFormat(format)
	default:
		// The parser restricts Kind to supported values; treat unknown kinds as
		// literal content types only when they already look like one.
		if strings.Contains(format, "/") {
			return format
		}
		return AudioContentTypeForFormat(format)
	}
}

// DecodeChunk requires a non-nil rule; payload is one SSE event's JSON object.
// It returns the decoded binary chunk (nil when the path is missing/empty) and
// whether the stream should stop after this chunk. Decode errors on individual
// chunks are returned so callers can decide to skip or abort.
func (r *SSEBinaryExtractRule) DecodeChunk(payload map[string]any) (data []byte, stop bool, err error) {
	stop = r.chunkMatchesStop(payload)
	value := jsonutil.GetStringByPath(payload, r.Path)
	if value == "" {
		return nil, stop, nil
	}
	data, decodeErr := decodeBinaryField(value, r.Decode)
	if decodeErr != nil {
		return nil, stop, fmt.Errorf("sse_binary_extract: decode %s failed: %w", r.Decode, decodeErr)
	}
	return data, stop, nil
}

func (r *SSEBinaryExtractRule) chunkMatchesStop(payload map[string]any) bool {
	if strings.TrimSpace(r.StopPath) == "" {
		return false
	}
	return jsonValueEqualsLiteral(payload, r.StopPath, r.StopEquals)
}

// Matches requires root to be the parsed upstream response JSON object.
// Missing paths never match, for both eq and ne, so absent error envelopes on
// success responses do not trigger false errors.
func (e ErrorWhenRule) Matches(root map[string]any) bool {
	if len(root) == 0 {
		return false
	}
	switch {
	case e.Equals != "":
		return jsonValueEqualsLiteral(root, e.Path, e.Equals)
	case e.NotEquals != "":
		if !jsonPathHasValue(root, e.Path) {
			return false
		}
		return !jsonValueEqualsLiteral(root, e.Path, e.NotEquals)
	default:
		return false
	}
}

func jsonPathHasValue(root map[string]any, path string) bool {
	values, ok := jsonutil.GetValuesByPath(root, path)
	return ok && len(values) > 0
}

// jsonValueEqualsLiteral compares the value at path with a literal string.
// When both sides parse as numbers they are compared numerically, so "2"
// matches JSON number 2; otherwise trimmed string equality applies.
func jsonValueEqualsLiteral(root map[string]any, path string, literal string) bool {
	values, ok := jsonutil.GetValuesByPath(root, path)
	if !ok || len(values) == 0 {
		return false
	}
	lit := strings.TrimSpace(literal)
	litParsed, litErr := strconv.ParseFloat(lit, 64)
	for _, v := range values {
		if f, ok := jsonutil.CoerceFloatOK(v); ok && litErr == nil {
			if f == litParsed {
				return true
			}
			continue
		}
		if strings.TrimSpace(jsonutil.CoerceString(v)) == lit {
			return true
		}
	}
	return false
}
