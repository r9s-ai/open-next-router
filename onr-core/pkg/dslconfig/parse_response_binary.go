package dslconfig

import (
	"strconv"
	"strings"
)

var supportedBinaryDecodeModes = map[string]struct{}{
	"hex":    {},
	"base64": {},
}

var supportedContentTypeKinds = map[string]struct{}{
	"audio": {},
}

// parseRespBodyExtractStmt parses:
// resp_body_extract path="$.data.audio" decode=hex;
func parseRespBodyExtractStmt(s *scanner, resp *ResponseDirective) error {
	opts, err := consumeDirectiveOptions(s, "resp_body_extract")
	if err != nil {
		return err
	}
	rule := &RespBodyExtractRule{}
	for _, opt := range opts {
		switch opt.key {
		case "path":
			rule.Path = opt.value
		case "decode":
			rule.Decode = strings.ToLower(opt.value)
		default:
			return s.errAt(opt.tok, "unsupported resp_body_extract option "+opt.key)
		}
	}
	if strings.TrimSpace(rule.Path) == "" {
		return s.errAt(token{pos: s.lastPos}, "resp_body_extract requires path")
	}
	if _, ok := supportedBinaryDecodeModes[rule.Decode]; !ok {
		return s.errAt(token{pos: s.lastPos}, "resp_body_extract decode expects hex or base64")
	}
	resp.BodyExtract = rule
	return nil
}

// parseRespContentTypeStmt parses:
// resp_content_type from_path="$.extra_info.audio_format" kind=audio [default="mp3"];
func parseRespContentTypeStmt(s *scanner, resp *ResponseDirective) error {
	opts, err := consumeDirectiveOptions(s, "resp_content_type")
	if err != nil {
		return err
	}
	rule := &RespContentTypeRule{}
	for _, opt := range opts {
		switch opt.key {
		case "from_path":
			rule.FromPath = opt.value
		case "kind":
			rule.Kind = strings.ToLower(opt.value)
		case "default":
			rule.Default = opt.value
		default:
			return s.errAt(opt.tok, "unsupported resp_content_type option "+opt.key)
		}
	}
	if strings.TrimSpace(rule.FromPath) == "" && strings.TrimSpace(rule.Default) == "" {
		return s.errAt(token{pos: s.lastPos}, "resp_content_type requires from_path or default")
	}
	if _, ok := supportedContentTypeKinds[rule.Kind]; !ok {
		return s.errAt(token{pos: s.lastPos}, "resp_content_type kind expects audio")
	}
	resp.ContentTypeRule = rule
	return nil
}

// parseSSEBinaryExtractStmt parses:
// sse_binary_extract path="$.data.audio" decode=hex stop_path="$.data.status" stop_eq=2;
func parseSSEBinaryExtractStmt(s *scanner, resp *ResponseDirective) error {
	opts, err := consumeDirectiveOptions(s, "sse_binary_extract")
	if err != nil {
		return err
	}
	rule := &SSEBinaryExtractRule{}
	for _, opt := range opts {
		switch opt.key {
		case "path":
			rule.Path = opt.value
		case "decode":
			rule.Decode = strings.ToLower(opt.value)
		case "stop_path":
			rule.StopPath = opt.value
		case "stop_eq":
			rule.StopEquals = opt.value
		default:
			return s.errAt(opt.tok, "unsupported sse_binary_extract option "+opt.key)
		}
	}
	if strings.TrimSpace(rule.Path) == "" {
		return s.errAt(token{pos: s.lastPos}, "sse_binary_extract requires path")
	}
	if _, ok := supportedBinaryDecodeModes[rule.Decode]; !ok {
		return s.errAt(token{pos: s.lastPos}, "sse_binary_extract decode expects hex or base64")
	}
	if (strings.TrimSpace(rule.StopPath) == "") != (strings.TrimSpace(rule.StopEquals) == "") {
		return s.errAt(token{pos: s.lastPos}, "sse_binary_extract requires stop_path and stop_eq together")
	}
	resp.SSEBinaryExtract = rule
	return nil
}

// parseErrorWhenStmt parses:
// error_when path="$.base_resp.status_code" ne=0 [status=400];
// error_when path="$.status" eq="error" [status=400];
func parseErrorWhenStmt(s *scanner, errDir *ErrorDirective) error {
	opts, err := consumeDirectiveOptions(s, "error_when")
	if err != nil {
		return err
	}
	rule := ErrorWhenRule{Status: 400}
	condSet := false
	for _, opt := range opts {
		switch opt.key {
		case "path":
			rule.Path = opt.value
		case "eq":
			if condSet {
				return s.errAt(opt.tok, "error_when allows only one of eq or ne")
			}
			rule.Equals = opt.value
			condSet = true
		case "ne":
			if condSet {
				return s.errAt(opt.tok, "error_when allows only one of eq or ne")
			}
			rule.NotEquals = opt.value
			condSet = true
		case "status":
			n, err := strconv.Atoi(opt.value)
			if err != nil || n < 400 || n > 599 {
				return s.errAt(opt.tok, "error_when status expects HTTP status in [400, 599]")
			}
			rule.Status = n
		default:
			return s.errAt(opt.tok, "unsupported error_when option "+opt.key)
		}
	}
	if strings.TrimSpace(rule.Path) == "" {
		return s.errAt(token{pos: s.lastPos}, "error_when requires path")
	}
	if !condSet {
		return s.errAt(token{pos: s.lastPos}, "error_when requires eq or ne")
	}
	errDir.ErrorWhen = append(errDir.ErrorWhen, rule)
	return nil
}

type directiveOption struct {
	key   string
	value string
	tok   token
}

// consumeDirectiveOptions reads `key=value` pairs until ';'. Values may be
// string literals, identifiers, or bare numbers (re-joined from single-char tokens).
func consumeDirectiveOptions(s *scanner, directive string) ([]directiveOption, error) {
	var out []directiveOption
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return nil, s.errAt(tok, "unexpected EOF in "+directive)
		case tokSemicolon:
			return out, nil
		case tokIdent:
			key := strings.ToLower(strings.TrimSpace(tok.text))
			if err := consumeEquals(s); err != nil {
				return nil, err
			}
			valTok := s.nextNonTrivia()
			var value string
			switch valTok.kind {
			case tokString:
				value = unquoteString(valTok.text)
			case tokIdent:
				value = strings.TrimSpace(valTok.text)
			case tokOther:
				f, err := parseNumberValueTokens(s, valTok)
				if err != nil {
					return nil, s.errAt(valTok, directive+" "+key+" expects string or number value")
				}
				value = strconv.FormatFloat(f, 'f', -1, 64)
			default:
				return nil, s.errAt(valTok, directive+" "+key+" expects value")
			}
			out = append(out, directiveOption{key: key, value: value, tok: tok})
		default:
			return nil, s.errAt(tok, "expected "+directive+" option or ';'")
		}
	}
}

// Bounds for resp_inline_url. The defaults match what a single image response
// needs; the maxima keep a misconfigured rule from turning one client request
// into an unbounded fan-out of large fetches.
const (
	respInlineURLDefaultMaxBytes    = 10 << 20 // 10 MiB
	respInlineURLLimitMaxBytes      = 64 << 20
	respInlineURLDefaultTimeoutMS   = 30000
	respInlineURLLimitTimeoutMS     = 120000
	respInlineURLDefaultConcurrency = 4
	respInlineURLLimitConcurrency   = 16
)

// parseRespInlineURLStmt parses:
// resp_inline_url path="$.data[*].url" set=b64_json
//
//	[when_request="$.response_format" when_eq="b64_json"]
//	[max_bytes=10485760] [timeout_ms=30000] [concurrency=4];
func parseRespInlineURLStmt(s *scanner, resp *ResponseDirective) error {
	opts, err := consumeDirectiveOptions(s, "resp_inline_url")
	if err != nil {
		return err
	}
	rule := &RespInlineURLRule{
		MaxBytes:    respInlineURLDefaultMaxBytes,
		TimeoutMS:   respInlineURLDefaultTimeoutMS,
		Concurrency: respInlineURLDefaultConcurrency,
	}
	for _, opt := range opts {
		switch opt.key {
		case "path":
			rule.Path = opt.value
		case "set":
			rule.SetField = opt.value
		case "when_request":
			rule.WhenRequestPath = opt.value
		case "when_eq":
			rule.WhenEquals = opt.value
		case "max_bytes":
			v, perr := strconv.ParseInt(strings.TrimSpace(opt.value), 10, 64)
			if perr != nil || v <= 0 || v > respInlineURLLimitMaxBytes {
				return s.errAt(opt.tok, "resp_inline_url max_bytes expects an integer in (0, 67108864]")
			}
			rule.MaxBytes = v
		case "timeout_ms":
			v, perr := strconv.Atoi(strings.TrimSpace(opt.value))
			if perr != nil || v <= 0 || v > respInlineURLLimitTimeoutMS {
				return s.errAt(opt.tok, "resp_inline_url timeout_ms expects an integer in (0, 120000]")
			}
			rule.TimeoutMS = v
		case "concurrency":
			v, perr := strconv.Atoi(strings.TrimSpace(opt.value))
			if perr != nil || v <= 0 || v > respInlineURLLimitConcurrency {
				return s.errAt(opt.tok, "resp_inline_url concurrency expects an integer in (0, 16]")
			}
			rule.Concurrency = v
		default:
			return s.errAt(opt.tok, "unsupported resp_inline_url option "+opt.key)
		}
	}
	if strings.TrimSpace(rule.Path) == "" {
		return s.errAt(token{pos: s.lastPos}, "resp_inline_url requires path")
	}
	if strings.TrimSpace(rule.SetField) == "" {
		return s.errAt(token{pos: s.lastPos}, "resp_inline_url requires set")
	}
	// when_eq without when_request would read as a gate but never gate anything.
	if strings.TrimSpace(rule.WhenRequestPath) == "" && strings.TrimSpace(rule.WhenEquals) != "" {
		return s.errAt(token{pos: s.lastPos}, "resp_inline_url when_eq requires when_request")
	}
	resp.InlineURL = rule
	return nil
}
