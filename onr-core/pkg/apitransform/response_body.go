package apitransform

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

// DecodeResponseBody decodes an upstream response body based on Content-Encoding.
func DecodeResponseBody(body []byte, contentEncoding string) ([]byte, bool, error) {
	switch strings.ToLower(strings.TrimSpace(contentEncoding)) {
	case "", "identity":
		return body, false, nil
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, false, err
		}
		defer func() { _ = gr.Close() }()
		decoded, err := io.ReadAll(gr)
		if err != nil {
			return nil, false, err
		}
		return decoded, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported Content-Encoding %q", contentEncoding)
	}
}

// ResponseBodyLooksLikeJSON reports whether the response body should be treated as a JSON object body.
func ResponseBodyLooksLikeJSON(contentType string, body []byte) bool {
	ctLower := strings.ToLower(strings.TrimSpace(contentType))
	trim := bytes.TrimSpace(body)
	if strings.Contains(ctLower, "json") {
		return true
	}
	return len(trim) > 0 && trim[0] == '{'
}

// ApplyResponseJSONOpsBody applies JSON ops on an object-root response payload.
func ApplyResponseJSONOpsBody(
	body map[string]any,
	apply func(map[string]any) (map[string]any, error),
) (map[string]any, bool, error) {
	if body == nil {
		return nil, false, nil
	}

	out, err := apply(body)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}
