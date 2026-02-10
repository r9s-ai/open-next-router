package logx

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

var enableColor = isatty.IsTerminal(os.Stdout.Fd()) && strings.TrimSpace(os.Getenv("NO_COLOR")) == ""

func ColorEnabled() bool { return enableColor }

func ColorizeStatus(status int) string {
	return ColorizeStatusWith(status, enableColor)
}

func ColorizeStatusWith(status int, color bool) string {
	if !color {
		return fmt.Sprintf("%d", status)
	}
	// ANSI colors
	const (
		reset  = "\x1b[0m"
		red    = "\x1b[31m"
		green  = "\x1b[32m"
		yellow = "\x1b[33m"
		cyan   = "\x1b[36m"
	)
	switch {
	case status >= 200 && status < 300:
		return green + fmt.Sprintf("%d", status) + reset
	case status >= 300 && status < 400:
		return cyan + fmt.Sprintf("%d", status) + reset
	case status >= 400 && status < 500:
		return yellow + fmt.Sprintf("%d", status) + reset
	default:
		return red + fmt.Sprintf("%d", status) + reset
	}
}

// FormatRequestLine prints a single line request log.
//
// Example:
// [ONR] 2026/01/26 - 17:44:22 | 200 | 12.3ms | 127.0.0.1 | GET "/v1/models" | provider=openai model=gpt-4o-mini
func FormatRequestLine(
	ts time.Time,
	status int,
	latency time.Duration,
	clientIP string,
	method string,
	path string,
	fields map[string]any,
) string {
	return FormatRequestLineWithColor(ts, status, latency, clientIP, method, path, fields, enableColor)
}

func FormatRequestLineWithColor(
	ts time.Time,
	status int,
	latency time.Duration,
	clientIP string,
	method string,
	path string,
	fields map[string]any,
	color bool,
) string {
	base := fmt.Sprintf(
		`[ONR] %s | %s | %s | %s | %s %q`,
		ts.Format("2006/01/02 - 15:04:05"),
		ColorizeStatusWith(status, color),
		latency.String(),
		strings.TrimSpace(clientIP),
		strings.TrimSpace(method),
		path,
	)
	extra := formatFields(fields)
	if extra == "" {
		return base
	}
	return base + " | " + extra
}

func formatFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	tokenKeys := map[string]struct{}{
		"input_tokens":          {},
		"output_tokens":         {},
		"total_tokens":          {},
		"cache_read_tokens":     {},
		"cache_write_tokens":    {},
		"billable_input_tokens": {},
		"cost_total":            {},
		"cost_input":            {},
		"cost_output":           {},
		"cost_cache_read":       {},
		"cost_cache_write":      {},
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		if _, ok := tokenKeys[k]; ok {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	appendIfPresent := func(k string) {
		v, ok := fields[k]
		if !ok || v == nil {
			return
		}
		if _, isToken := tokenKeys[k]; isToken {
			switch t := v.(type) {
			case int:
				if t == 0 {
					return
				}
			case int64:
				if t == 0 {
					return
				}
			case float64:
				if t == 0 {
					return
				}
			}
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				return
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, t))
		case float64:
			s := strings.TrimSpace(strconv.FormatFloat(t, 'f', 12, 64))
			s = strings.TrimRight(s, "0")
			s = strings.TrimRight(s, ".")
			if s == "" || s == "-" {
				s = "0"
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, s))
		default:
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s == "" || s == "<nil>" {
				return
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, s))
		}
	}

	for _, k := range keys {
		appendIfPresent(k)
	}

	// Keep token usage fields at the end for readability.
	for _, k := range []string{
		"input_tokens",
		"output_tokens",
		"total_tokens",
		"cache_read_tokens",
		"cache_write_tokens",
		"billable_input_tokens",
		"cost_input",
		"cost_output",
		"cost_cache_read",
		"cost_cache_write",
		"cost_total",
	} {
		appendIfPresent(k)
	}
	return strings.Join(parts, " ")
}
