package logx

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

var enableColor = isatty.IsTerminal(os.Stdout.Fd()) && strings.TrimSpace(os.Getenv("NO_COLOR")) == ""

func ColorizeStatus(status int) string {
	if !enableColor {
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
	base := fmt.Sprintf(
		`[ONR] %s | %s | %s | %s | %s %q`,
		ts.Format("2006/01/02 - 15:04:05"),
		ColorizeStatus(status),
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
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v, ok := fields[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, t))
		default:
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s == "" || s == "<nil>" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, s))
		}
	}
	return strings.Join(parts, " ")
}
