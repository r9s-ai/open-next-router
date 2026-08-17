package dslconfig

import (
	"strconv"
	"strings"
)

// Bounds for req_inline_file. The default byte cap matches what an image edit
// upload needs; the maxima keep a misconfigured rule from letting one request
// pull an unbounded amount of client data into memory.
const (
	reqInlineFileDefaultMaxBytes = 20 << 20 // 20 MiB
	reqInlineFileLimitMaxBytes   = 64 << 20
	reqInlineFileDefaultMaxCount = 4
	reqInlineFileLimitMaxCount   = 16
)

// parseReqInlineFileStmt parses:
// req_inline_file field="image" [max_bytes=20971520] [max_count=4];
func parseReqInlineFileStmt(s *scanner, transform *RequestTransform) error {
	opts, err := consumeDirectiveOptions(s, "req_inline_file")
	if err != nil {
		return err
	}
	rule := ReqInlineFileRule{
		MaxBytes: reqInlineFileDefaultMaxBytes,
		MaxCount: reqInlineFileDefaultMaxCount,
	}
	for _, opt := range opts {
		switch opt.key {
		case "field":
			rule.Field = opt.value
		case "max_bytes":
			v, perr := strconv.ParseInt(strings.TrimSpace(opt.value), 10, 64)
			if perr != nil || v <= 0 || v > reqInlineFileLimitMaxBytes {
				return s.errAt(opt.tok, "req_inline_file max_bytes expects an integer in (0, 67108864]")
			}
			rule.MaxBytes = v
		case "max_count":
			v, perr := strconv.Atoi(strings.TrimSpace(opt.value))
			if perr != nil || v <= 0 || v > reqInlineFileLimitMaxCount {
				return s.errAt(opt.tok, "req_inline_file max_count expects an integer in (0, 16]")
			}
			rule.MaxCount = v
		default:
			return s.errAt(opt.tok, "unsupported req_inline_file option "+opt.key)
		}
	}
	if strings.TrimSpace(rule.Field) == "" {
		return s.errAt(token{pos: s.lastPos}, "req_inline_file requires field")
	}
	rule.Field = strings.TrimSpace(rule.Field)
	transform.InlineFiles = append(transform.InlineFiles, rule)
	return nil
}
