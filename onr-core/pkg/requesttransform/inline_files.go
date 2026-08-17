package requesttransform

import (
	"encoding/base64"
	"errors"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
)

// applyInlineFiles reads the multipart file fields a config declared and places
// them into the request object root, so req_map builtins can reach the uploaded
// bytes. It returns the root to continue with, which may be freshly allocated
// when the incoming root was nil (a multipart body with no text fields at all).
//
// Each field lands under its own form field name as an array of
// {filename, content_type, b64} objects. The array is used even for a single
// file so builtins read one shape; OpenAI's images.edits allows the image field
// to repeat, and a shape that changes with the file count is a trap.
//
// A missing field is not an error: configs declare optional uploads (a mask)
// unconditionally, and the builtin decides whether absence is acceptable.
func applyInlineFiles(rules []dslconfig.ReqInlineFileRule, body []byte, contentType string, root map[string]any) (map[string]any, error) {
	if len(rules) == 0 {
		return root, nil
	}
	if !requestcanon.IsMultipartFormData(contentType) {
		// Not an upload, so there is nothing to inline. Configs pair
		// req_inline_file with a multipart API; a JSON request to the same
		// match simply carries its fields inline already.
		return root, nil
	}

	requests := make([]requestcanon.FileFieldRequest, 0, len(rules))
	for _, rule := range rules {
		requests = append(requests, requestcanon.FileFieldRequest{
			Field:    rule.Field,
			MaxBytes: rule.MaxBytes,
			MaxCount: rule.MaxCount,
		})
	}

	files, err := requestcanon.ExtractMultipartFiles(body, contentType, requests)
	if err != nil {
		// Budget violations are the client's doing, so they surface as a
		// validation error (400) rather than an opaque transform failure (500).
		var tooLarge *requestcanon.TooLargeError
		var tooMany *requestcanon.TooManyError
		if errors.As(err, &tooLarge) || errors.As(err, &tooMany) {
			return nil, &ValidationError{Message: err.Error()}
		}
		return nil, err
	}
	if len(files) == 0 {
		return root, nil
	}

	if root == nil {
		root = make(map[string]any, len(files))
	}
	for field, parts := range files {
		items := make([]any, 0, len(parts))
		for _, part := range parts {
			items = append(items, map[string]any{
				"filename":     part.Filename,
				"content_type": part.ContentType,
				"b64":          base64.StdEncoding.EncodeToString(part.Data),
			})
		}
		root[field] = items
	}
	return root, nil
}
