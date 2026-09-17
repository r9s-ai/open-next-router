package dslconfig

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
)

// parseProviderJS walks the structural DSL independently of the existing phase
// parsers. A slot belongs to a defaults/match scope and a full block path, so
// repeated request/after_req_map blocks cannot hide duplicate handlers.
func parseProviderJS(path, content, provider string) (jsext.Provider, error) {
	s := newScanner(path, content)
	var out jsext.Provider
	var walk func([]string, *jsext.Config, bool) error
	walk = func(stack []string, cfg *jsext.Config, selected bool) error {
		for {
			name := s.nextNonTrivia()
			switch name.kind {
			case tokEOF:
				if len(stack) > 0 {
					return s.errAt(name, "unclosed block")
				}
				return nil
			case tokRBrace:
				return nil
			case tokJSError:
				return s.errAt(name, name.text)
			}
			if name.kind != tokIdent {
				continue
			}
			args, end := jsStatementTokens(s)
			if end.kind == tokJSError {
				return s.errAt(end, end.text)
			}
			isJS := strings.Contains(name.text, "_by_js") || strings.HasPrefix(name.text, "js_")
			if isJS {
				if err := parseJSDirective(s, name, args, end, stack, cfg); err != nil {
					return err
				}
				continue
			}
			if end.kind == tokJSBlock {
				return s.errAt(end, "unexpected JavaScript body")
			}
			if end.kind == tokLBrace {
				child := append(append([]string(nil), stack...), name.text)
				nextCfg := cfg
				nextSelected := selected
				if name.text == "provider" {
					nextSelected = len(args) == 1 && normalizeProviderName(unquoteString(args[0].text)) == provider
				}
				if len(stack) == 1 && stack[0] == "provider" && (name.text == "defaults" || name.text == "match") {
					nextCfg = &jsext.Config{}
					if name.text == "defaults" && nextSelected {
						nextCfg = &out.Defaults
					}
					if name.text == "match" {
						m := jsext.Match{}
						for i := 0; i+2 < len(args); i++ {
							if args[i+1].text != "=" {
								continue
							}
							switch args[i].text {
							case "api":
								m.API = strings.TrimSpace(unquoteString(args[i+2].text))
							case "stream":
								v := args[i+2].text == "true"
								m.Stream = &v
							}
						}
						if nextSelected {
							out.Matches = append(out.Matches, m)
							nextCfg = &out.Matches[len(out.Matches)-1].Config
						}
					}
				}
				if err := walk(child, nextCfg, nextSelected); err != nil {
					return err
				}
			} else if end.kind != tokSemicolon {
				return s.errAt(end, "expected ';' or block")
			}
		}
	}
	if err := walk(nil, nil, false); err != nil {
		return out, err
	}
	return out, nil
}
func parseJSDirective(s *scanner, name token, args []token, end token, stack []string, cfg *jsext.Config) error {
	fail := func(msg string) error { return s.errAt(name, msg) }
	if cfg == nil || len(stack) < 2 || stack[0] != "provider" || (stack[1] != "defaults" && stack[1] != "match") {
		return fail("JavaScript directive requires defaults or match scope")
	}
	if strings.HasPrefix(name.text, "js_") {
		if len(stack) != 2 || end.kind != tokSemicolon {
			return fail("JS limits belong directly to defaults/match")
		}
		return parseJSLimit(s, name, args, cfg)
	}
	base := name.text
	form := "off"
	if strings.HasSuffix(base, "_block") {
		base = strings.TrimSuffix(base, "_block")
		form = "block"
	} else if strings.HasSuffix(base, "_file") {
		base = strings.TrimSuffix(base, "_file")
		form = "file"
	}
	stage := jsStage(base, strings.Join(stack[2:], "."))
	if stage == "" {
		return fail("unknown or misplaced JS handler: " + name.text)
	}
	if cfg.Handlers == nil {
		cfg.Handlers = map[jsext.Stage]jsext.HandlerSource{}
	}
	if _, ok := cfg.Handlers[stage]; ok {
		return fail("duplicate JS handler slot: " + string(stage))
	}
	line, col := s.lineCol(end.pos + 1)
	h := jsext.HandlerSource{Location: s.path, Line: line, Column: col}
	if end.origin != nil {
		h.Location = end.origin.File
		h.Line = end.origin.Line
		h.Column = end.origin.Column
	}
	switch form {
	case "block":
		if len(args) != 0 || end.kind != tokJSBlock {
			return fail("expected JavaScript block")
		}
		h.Body = end.text[1 : len(end.text)-1]
		if _, err := jsext.CompileAt(h.Location, h.Body, h.Line, h.Column); err != nil {
			return err
		}
	case "file":
		if len(args) == 0 || end.kind != tokSemicolon {
			return fail("expected script path and ';'")
		}
		raw, err := jsSingleArgument(args)
		if err != nil {
			return fail(err.Error())
		}
		h.File = raw
		if strings.ContainsAny(h.File, "\n\r") || !filepath.IsLocal(h.File) || strings.Contains(h.File, "://") {
			return fail("invalid script path")
		}
	case "off":
		if len(args) != 1 || args[0].text != "off" || end.kind != tokSemicolon {
			return fail("expected off;")
		}
		h.Off = true
	}
	cfg.Handlers[stage] = h
	return nil
}

func parseJSLimit(s *scanner, name token, args []token, cfg *jsext.Config) error {
	fail := func(msg string) error { return s.errAt(name, msg) }
	raw, err := jsSingleArgument(args)
	if err != nil {
		return fail(err.Error())
	}
	switch name.text {
	case "js_timeout", "js_stream_timeout":
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			return fail("JS timeout must be a positive duration")
		}
		target := &cfg.Limits.Timeout
		if name.text == "js_stream_timeout" {
			target = &cfg.Limits.StreamTimeout
		}
		if *target != 0 {
			return fail("duplicate JS limit")
		}
		*target = d
	case "js_body_limit", "js_event_limit":
		factor := int64(1)
		raw = strings.ToLower(raw)
		if strings.HasSuffix(raw, "k") {
			factor = 1 << 10
			raw = strings.TrimSuffix(raw, "k")
		}
		if strings.HasSuffix(raw, "m") {
			factor = 1 << 20
			raw = strings.TrimSuffix(raw, "m")
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 || n > (1<<30)/factor {
			return fail("JS size limit must be positive and at most 1 GiB")
		}
		target := &cfg.Limits.BodyBytes
		if name.text == "js_event_limit" {
			target = &cfg.Limits.EventBytes
		}
		if *target != 0 {
			return fail("duplicate JS limit")
		}
		*target = n * factor
	default:
		return fail("unknown JS limit")
	}
	return nil
}

func jsStage(base, sub string) jsext.Stage {
	var stage jsext.Stage
	switch base {
	case "request_by_js":
		switch sub {
		case "request":
			stage = jsext.Request
		case "request.after_req_map":
			stage = jsext.AfterRequestMap
		}
	case "response_headers_by_js":
		if sub == "response" {
			stage = jsext.ResponseHeaders
		}
	case "response_by_js":
		if sub == "response" {
			stage = jsext.Response
		}
	case "sse_event_by_js":
		if sub == "response" {
			stage = jsext.SSEEvent
		}
	case "log_by_js":
		if sub == "" {
			stage = jsext.Log
		}
	}
	return stage
}

func jsStatementTokens(s *scanner) ([]token, token) {
	var args []token
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF, tokLBrace, tokJSBlock, tokJSError, tokSemicolon, tokRBrace:
			return args, tok
		}
		args = append(args, tok)
	}
}

func jsSingleArgument(args []token) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("expected one argument")
	}
	var out strings.Builder
	for i, tok := range args {
		if i > 0 && (tok.pos != args[i-1].pos+len(args[i-1].text) || tok.kind == tokString || args[i-1].kind == tokString) {
			return "", fmt.Errorf("expected one argument")
		}
		out.WriteString(unquoteString(tok.text))
	}
	return out.String(), nil
}
