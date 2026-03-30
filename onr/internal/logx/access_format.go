package logx

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

type formatPart struct {
	literal string
	varName string
}

type AccessLogFormatter struct {
	parts []formatPart
}

var accessLogFormatPresets = map[string]string{
	"onr_combined": "$time_local | $status | $latency | $client_ip | $method $path | request_id=$request_id appname=$appname provider=$provider provider_source=$provider_source api=$api stream=$stream model=$model usage_stage=$usage_stage input_tokens=$input_tokens output_tokens=$output_tokens total_tokens=$total_tokens cache_read_tokens=$cache_read_tokens cache_write_tokens=$cache_write_tokens cost_total=$cost_total cost_input=$cost_input cost_output=$cost_output cost_cache_read=$cost_cache_read cost_cache_write=$cost_cache_write billable_input_tokens=$billable_input_tokens cost_multiplier=$cost_multiplier cost_model=$cost_model cost_channel=$cost_channel cost_unit=$cost_unit upstream_status=$upstream_status finish_reason=$finish_reason ttft_ms=$ttft_ms tps=$tps",
	"onr_minimal":  "$time_local | $status | $latency | $method $path | request_id=$request_id appname=$appname provider=$provider model=$model total_tokens=$total_tokens cost_total=$cost_total",
}

func ResolveAccessLogFormat(format string, preset string) (string, error) {
	if strings.TrimSpace(format) != "" {
		return format, nil
	}
	p := strings.ToLower(strings.TrimSpace(preset))
	if p == "" {
		return "", nil
	}
	out, ok := accessLogFormatPresets[p]
	if !ok {
		return "", fmt.Errorf("invalid access_log_format_preset: %q", preset)
	}
	return out, nil
}

func CompileAccessLogFormat(format string) (*AccessLogFormatter, error) {
	s := strings.TrimSpace(format)
	if s == "" {
		return nil, nil
	}
	parts := make([]formatPart, 0, 8)
	var lit strings.Builder

	flushLiteral := func() {
		if lit.Len() == 0 {
			return
		}
		parts = append(parts, formatPart{literal: lit.String()})
		lit.Reset()
	}

	for i := 0; i < len(format); i++ {
		ch := format[i]
		if ch != '$' {
			lit.WriteByte(ch)
			continue
		}
		if i+1 < len(format) && format[i+1] == '$' {
			lit.WriteByte('$')
			i++
			continue
		}
		flushLiteral()
		j := i + 1
		for j < len(format) {
			r := rune(format[j])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				break
			}
			j++
		}
		if j == i+1 {
			return nil, fmt.Errorf("invalid access_log_format: missing variable name after '$' at pos %d", i)
		}
		name := format[i+1 : j]
		if _, ok := allowedAccessLogVars[name]; !ok {
			return nil, fmt.Errorf("invalid access_log_format: unknown variable $%s", name)
		}
		parts = append(parts, formatPart{varName: name})
		i = j - 1
	}
	flushLiteral()
	return &AccessLogFormatter{parts: parts}, nil
}

func (f *AccessLogFormatter) Format(
	ts time.Time,
	status int,
	latency time.Duration,
	clientIP string,
	method string,
	path string,
	fields map[string]any,
	color bool,
) string {
	if f == nil || len(f.parts) == 0 {
		return ""
	}
	vars := map[string]string{
		"time_local": ts.Format("2006/01/02 - 15:04:05"),
		"status":     ColorizeStatusWith(status, color),
		"latency":    latency.String(),
		"latency_ms": fmt.Sprintf("%d", latency.Milliseconds()),
		"client_ip":  strings.TrimSpace(clientIP),
		"method":     strings.TrimSpace(method),
		"path":       path,
	}
	for k, v := range fields {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" || s == "<nil>" {
			continue
		}
		vars[k] = s
	}

	var b strings.Builder
	for _, p := range f.parts {
		if p.literal != "" {
			b.WriteString(p.literal)
			continue
		}
		v := strings.TrimSpace(vars[p.varName])
		if v == "" {
			b.WriteByte('-')
			continue
		}
		b.WriteString(v)
	}
	return b.String()
}
