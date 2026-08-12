package requestcanon

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

// FilePart is one uploaded file read out of a multipart request body.
//
// Data holds the whole file in memory. Inspect deliberately discards file parts
// for exactly that reason, so nothing here runs unless a config asked for a
// specific field by name and gave it a byte budget.
type FilePart struct {
	Field       string
	Filename    string
	ContentType string
	Data        []byte
}

// FileFieldRequest describes one field a config asked to read, with the byte
// budget and file count it is allowed to consume.
type FileFieldRequest struct {
	Field    string
	MaxBytes int64
	MaxCount int
}

// TooLargeError reports a file field that exceeded its configured budget. It is
// distinct from a malformed-body error because callers surface it to the client
// as a 400 rather than a 500: the request is well formed, just too big.
type TooLargeError struct {
	Field    string
	MaxBytes int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("multipart file field %q exceeds the configured limit of %d bytes", e.Field, e.MaxBytes)
}

// TooManyError reports a field that repeated more times than the config allows.
type TooManyError struct {
	Field    string
	MaxCount int
}

func (e *TooManyError) Error() string {
	return fmt.Sprintf("multipart file field %q repeats more than the configured limit of %d files", e.Field, e.MaxCount)
}

// ExtractMultipartFiles reads the requested file fields out of a multipart body
// and returns them keyed by field name. Fields that are absent from the body are
// absent from the result; asking for a field that was not uploaded is not an
// error, so a config can declare an optional field (a mask, say) unconditionally.
//
// Parts that are not files, and files whose field was not requested, are drained
// and dropped without being buffered.
func ExtractMultipartFiles(body []byte, contentType string, requests []FileFieldRequest) (map[string][]FilePart, error) {
	if len(requests) == 0 || len(body) == 0 {
		return nil, nil
	}
	if !IsMultipartFormData(contentType) {
		return nil, nil
	}

	wanted := make(map[string]FileFieldRequest, len(requests))
	for _, req := range requests {
		field := strings.TrimSpace(req.Field)
		if field == "" {
			continue
		}
		// A field declared twice keeps the tighter budget, so a later, laxer
		// rule cannot widen a limit an earlier one set.
		if prev, ok := wanted[field]; ok {
			if req.MaxBytes > prev.MaxBytes {
				req.MaxBytes = prev.MaxBytes
			}
			if req.MaxCount > prev.MaxCount {
				req.MaxCount = prev.MaxCount
			}
		}
		req.Field = field
		wanted[field] = req
	}
	if len(wanted) == 0 {
		return nil, nil
	}

	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return nil, fmt.Errorf("parse multipart content-type: %w", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("multipart boundary is empty")
	}

	out := make(map[string][]FilePart)
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, perr := reader.NextPart()
		if perr == io.EOF {
			return out, nil
		}
		if perr != nil {
			return nil, fmt.Errorf("read multipart form: %w", perr)
		}

		name := strings.TrimSpace(part.FormName())
		filename := strings.TrimSpace(part.FileName())
		req, requested := wanted[name]
		if filename == "" || !requested {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}
		if req.MaxCount > 0 && len(out[name]) >= req.MaxCount {
			_ = part.Close()
			return nil, &TooManyError{Field: name, MaxCount: req.MaxCount}
		}

		// Read one byte past the budget so an oversized upload is rejected
		// rather than silently truncated into a corrupt image.
		data, rerr := io.ReadAll(io.LimitReader(part, req.MaxBytes+1))
		_ = part.Close()
		if rerr != nil {
			return nil, fmt.Errorf("read multipart file field %q: %w", name, rerr)
		}
		if int64(len(data)) > req.MaxBytes {
			return nil, &TooLargeError{Field: name, MaxBytes: req.MaxBytes}
		}

		out[name] = append(out[name], FilePart{
			Field:       name,
			Filename:    filename,
			ContentType: filePartContentType(part.Header.Get("Content-Type"), filename, data),
			Data:        data,
		})
	}
}

const octetStream = "application/octet-stream"

// filePartContentType resolves the MIME type an upstream will be told the file
// has. It follows the relay Go adaptor's GetFileContentType: the declared type
// wins unless it is the generic octet-stream, in which case the filename
// extension decides. Many HTTP clients label every upload octet-stream, and
// Gemini's inlineData needs a real image type to accept the part.
//
// Sniffing the bytes is the last resort, for a client that both declared
// nothing useful and sent an extensionless filename.
func filePartContentType(declared, filename string, data []byte) string {
	declared = normalizeMediaType(strings.TrimSpace(declared))
	if declared != "" && declared != octetStream {
		return declared
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
		if byExt := normalizeMediaType(mime.TypeByExtension(ext)); byExt != "" {
			return byExt
		}
	}
	if len(data) > 0 {
		if sniffed := normalizeMediaType(http.DetectContentType(data)); sniffed != "" && sniffed != octetStream {
			return sniffed
		}
	}
	if declared != "" {
		return declared
	}
	return octetStream
}

// normalizeMediaType strips parameters such as "; charset=utf-8", which are
// noise in an inlineData mimeType field.
func normalizeMediaType(value string) string {
	if value == "" {
		return ""
	}
	if mediaType, _, err := mime.ParseMediaType(value); err == nil {
		return mediaType
	}
	return value
}
