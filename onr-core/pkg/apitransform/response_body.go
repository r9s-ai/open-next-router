package apitransform

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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

func ResponseBodyLooksLikeJSON(contentType string, body []byte) bool {
	ctLower := strings.ToLower(strings.TrimSpace(contentType))
	trim := bytes.TrimSpace(body)
	if strings.Contains(ctLower, "json") {
		return true
	}
	return len(trim) > 0 && trim[0] == '{'
}

func ApplyResponseJSONOpsBody(
	body []byte,
	contentType string,
	apply func(map[string]any) (any, error),
) ([]byte, bool, error) {
	if apply == nil || !ResponseBodyLooksLikeJSON(contentType, body) {
		return body, false, nil
	}

	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, err
	}
	obj, _ := root.(map[string]any)
	if obj == nil {
		return body, false, nil
	}

	out, err := apply(obj)
	if err != nil {
		return nil, false, err
	}
	outBytes, err := json.Marshal(out)
	if err != nil {
		return nil, false, err
	}
	return outBytes, true, nil
}
