package requestcanon

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

type InspectOptions struct {
	AllowNonJSON bool
}

type Snapshot struct {
	Body        []byte
	Root        map[string]any
	Model       string
	Stream      bool
	ContentType string
}

func Inspect(body []byte, contentType string, opts InspectOptions) (Snapshot, error) {
	snapshot := Snapshot{
		Body:        body,
		ContentType: strings.TrimSpace(contentType),
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return snapshot, nil
	}

	if IsMultipartFormData(snapshot.ContentType) {
		root, model, stream, err := inspectMultipartBody(body, snapshot.ContentType)
		if err != nil {
			return snapshot, err
		}
		snapshot.Root = root
		snapshot.Model = model
		snapshot.Stream = stream
		return snapshot, nil
	}

	var reqObj any
	if err := json.Unmarshal(body, &reqObj); err != nil {
		if opts.AllowNonJSON && !DeclaresJSON(snapshot.ContentType) {
			return snapshot, nil
		}
		return snapshot, fmt.Errorf("invalid json: %w", err)
	}

	root, _ := reqObj.(map[string]any)
	snapshot.Root = root
	if root != nil {
		if v, ok := root["model"].(string); ok {
			snapshot.Model = strings.TrimSpace(v)
		}
		if v, ok := root["stream"].(bool); ok {
			snapshot.Stream = v
		}
	}
	return snapshot, nil
}

func ParseRoot(body []byte, contentType string) map[string]any {
	snapshot, err := Inspect(body, contentType, InspectOptions{AllowNonJSON: true})
	if err != nil {
		return nil
	}
	return snapshot.Root
}

func RootFromMultipartForm(form *multipart.Form) map[string]any {
	if form == nil || len(form.Value) == 0 {
		return nil
	}

	root := make(map[string]any, len(form.Value))
	for key, values := range form.Value {
		switch len(values) {
		case 0:
			continue
		case 1:
			root[key] = values[0]
		default:
			items := make([]any, 0, len(values))
			for _, value := range values {
				items = append(items, value)
			}
			root[key] = items
		}
	}
	if len(root) == 0 {
		return nil
	}
	return root
}

func IsMultipartFormData(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "multipart/form-data")
}

func DeclaresJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil && strings.TrimSpace(mediaType) != "" {
		return strings.Contains(strings.ToLower(mediaType), "json")
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "json")
}

func inspectMultipartBody(body []byte, contentType string) (root map[string]any, model string, stream bool, err error) {
	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return nil, "", false, fmt.Errorf("parse multipart content-type: %w", err)
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, "", false, fmt.Errorf("multipart boundary is empty")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
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
	for key, vals := range values {
		switch len(vals) {
		case 0:
			continue
		case 1:
			root[key] = vals[0]
		default:
			items := make([]any, 0, len(vals))
			for _, value := range vals {
				items = append(items, value)
			}
			root[key] = items
		}
	}
	if len(root) == 0 {
		return nil
	}
	return root
}
