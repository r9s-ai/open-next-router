package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"
)

const multipartFieldValueLimit = 1 << 20

type RequestBodyInfo struct {
	Root   map[string]any
	Model  string
	Stream bool
}

func InspectRequestBody(bodyBytes []byte, contentType string, allowNonJSON bool) (RequestBodyInfo, error) {
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return RequestBodyInfo{}, nil
	}

	if isMultipartFormData(contentType) {
		root, model, stream, err := inspectMultipartForm(bodyBytes, contentType)
		if err != nil {
			return RequestBodyInfo{}, err
		}
		return RequestBodyInfo{
			Root:   root,
			Model:  model,
			Stream: stream,
		}, nil
	}

	var reqObj any
	if err := json.Unmarshal(bodyBytes, &reqObj); err != nil {
		if allowNonJSON && !declaresJSON(contentType) {
			return RequestBodyInfo{}, nil
		}
		return RequestBodyInfo{}, fmt.Errorf("invalid json: %w", err)
	}

	root, _ := reqObj.(map[string]any)
	info := RequestBodyInfo{Root: root}
	if root != nil {
		if v, ok := root["model"].(string); ok {
			info.Model = strings.TrimSpace(v)
		}
		if v, ok := root["stream"].(bool); ok {
			info.Stream = v
		}
	}
	return info, nil
}

func isMultipartFormData(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "multipart/form-data")
}

func declaresJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil && strings.TrimSpace(mediaType) != "" {
		return strings.Contains(strings.ToLower(mediaType), "json")
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "json")
}

func inspectMultipartForm(bodyBytes []byte, contentType string) (root map[string]any, model string, stream bool, err error) {
	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return nil, "", false, fmt.Errorf("parse multipart content-type: %w", err)
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, "", false, fmt.Errorf("multipart boundary is empty")
	}

	reader := multipart.NewReader(bytes.NewReader(bodyBytes), boundary)
	values := make(map[string][]string)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			root = multipartValuesRoot(values)
			return root, strings.TrimSpace(model), stream, nil
		}
		if err != nil {
			return nil, "", false, fmt.Errorf("read multipart form: %w", err)
		}

		name := strings.TrimSpace(part.FormName())
		isFile := strings.TrimSpace(part.FileName()) != ""
		if isFile || name == "" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}

		valueBytes, rerr := io.ReadAll(io.LimitReader(part, multipartFieldValueLimit+1))
		_ = part.Close()
		if rerr != nil {
			return nil, "", false, fmt.Errorf("read multipart field %q: %w", name, rerr)
		}
		if len(valueBytes) > multipartFieldValueLimit {
			return nil, "", false, fmt.Errorf("multipart field %q too large", name)
		}
		value := strings.TrimSpace(string(valueBytes))
		values[name] = append(values[name], value)

		switch name {
		case "model":
			model = value
		case "stream":
			if value == "" {
				continue
			}
			b, perr := strconv.ParseBool(value)
			if perr != nil {
				return nil, "", false, fmt.Errorf("parse multipart stream field: %w", perr)
			}
			stream = b
		}
	}
}

func multipartValuesRoot(values map[string][]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	root := make(map[string]any, len(values))
	for k, vals := range values {
		switch len(vals) {
		case 0:
			continue
		case 1:
			root[k] = vals[0]
		default:
			items := make([]any, 0, len(vals))
			for _, v := range vals {
				items = append(items, v)
			}
			root[k] = items
		}
	}
	if len(root) == 0 {
		return nil
	}
	return root
}
