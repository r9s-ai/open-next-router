package logx

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

type SystemLogLevel int

const (
	SystemLogLevelDebug SystemLogLevel = iota
	SystemLogLevelInfo
	SystemLogLevelWarn
	SystemLogLevelError
)

const (
	SystemCategoryStartup   = "startup"
	SystemCategoryServer    = "server"
	SystemCategoryReload    = "reload"
	SystemCategoryProviders = "providers"
)

var allowedSystemCategories = map[string]struct{}{
	SystemCategoryStartup:   {},
	SystemCategoryServer:    {},
	SystemCategoryReload:    {},
	SystemCategoryProviders: {},
}

type SystemLoggerOptions struct {
	Writer io.Writer
	Level  string
	Color  *bool
}

type SystemLogger struct {
	mu       sync.Mutex
	writer   io.Writer
	minLevel SystemLogLevel
	color    bool
	now      func() time.Time
}

// SetNowFunc is intended for tests to override timestamp generation.
// It requires a non-nil logger receiver and non-nil clock function.
func (l *SystemLogger) SetNowFunc(now func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = now
}

func NewSystemLogger(level string) (*SystemLogger, error) {
	return NewSystemLoggerWithOptions(SystemLoggerOptions{
		Writer: os.Stderr,
		Level:  level,
	})
}

func NewSystemLoggerWithOptions(opts SystemLoggerOptions) (*SystemLogger, error) {
	minLevel, err := ParseSystemLogLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	writer := opts.Writer
	if writer == nil {
		writer = os.Stderr
	}
	color := false
	if opts.Color != nil {
		color = *opts.Color
	} else {
		color = shouldEnableSystemLogColor(isTerminalWriter(writer), os.Getenv("NO_COLOR"))
	}

	return &SystemLogger{
		writer:   writer,
		minLevel: minLevel,
		color:    color,
		now:      time.Now,
	}, nil
}

func ParseSystemLogLevel(level string) (SystemLogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return SystemLogLevelDebug, nil
	case "info", "":
		return SystemLogLevelInfo, nil
	case "warn":
		return SystemLogLevelWarn, nil
	case "error":
		return SystemLogLevelError, nil
	default:
		return SystemLogLevelInfo, fmt.Errorf("invalid logging.level: %q (allowed: debug|info|warn|error)", strings.TrimSpace(level))
	}
}

func NormalizeSystemLogLevel(level string) (string, error) {
	parsed, err := ParseSystemLogLevel(level)
	if err != nil {
		return "", err
	}
	return systemLogLevelString(parsed), nil
}

func shouldEnableSystemLogColor(isTTY bool, noColor string) bool {
	return isTTY && strings.TrimSpace(noColor) == ""
}

func isTerminalWriter(w io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	fdw, ok := w.(fdWriter)
	if !ok {
		return false
	}
	return isatty.IsTerminal(fdw.Fd())
}

func (l *SystemLogger) Debug(category string, msg string, fields map[string]any) {
	l.log(SystemLogLevelDebug, category, msg, fields)
}

func (l *SystemLogger) Info(category string, msg string, fields map[string]any) {
	l.log(SystemLogLevelInfo, category, msg, fields)
}

func (l *SystemLogger) Warn(category string, msg string, fields map[string]any) {
	l.log(SystemLogLevelWarn, category, msg, fields)
}

func (l *SystemLogger) Error(category string, msg string, fields map[string]any) {
	l.log(SystemLogLevelError, category, msg, fields)
}

func (l *SystemLogger) log(level SystemLogLevel, category string, msg string, fields map[string]any) {
	if l == nil {
		return
	}
	if level < l.minLevel {
		return
	}

	cat := strings.ToLower(strings.TrimSpace(category))
	if _, ok := allowedSystemCategories[cat]; !ok {
		cat = SystemCategoryServer
	}
	l.mu.Lock()
	nowFn := l.now
	l.mu.Unlock()
	if nowFn == nil {
		nowFn = time.Now
	}

	ts := nowFn().Format("2006/01/02 - 15:04:05")
	base := fmt.Sprintf("[ONR] %s | %s | %s | %s", ts, l.colorizedLevel(level), cat, sanitizeSystemMessage(msg))

	entry := map[string]any{}
	for k, v := range fields {
		if strings.TrimSpace(k) == "" || strings.ContainsAny(k, " \t\r\n") {
			continue
		}
		entry[k] = v
	}
	extra := formatKVLine(entry)
	line := base
	if strings.TrimSpace(extra) != "" {
		line += " | " + extra
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = io.WriteString(l.writer, line+"\n")
}

func sanitizeSystemMessage(msg string) string {
	s := strings.TrimSpace(msg)
	if s == "" {
		return "-"
	}
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func (l *SystemLogger) colorizedLevel(level SystemLogLevel) string {
	raw := systemLogLevelString(level)
	if !l.color {
		return strings.ToUpper(raw)
	}
	const (
		reset  = "\x1b[0m"
		red    = "\x1b[31m"
		green  = "\x1b[32m"
		yellow = "\x1b[33m"
		cyan   = "\x1b[36m"
	)
	switch level {
	case SystemLogLevelDebug:
		return cyan + strings.ToUpper(raw) + reset
	case SystemLogLevelInfo:
		return green + strings.ToUpper(raw) + reset
	case SystemLogLevelWarn:
		return yellow + strings.ToUpper(raw) + reset
	case SystemLogLevelError:
		return red + strings.ToUpper(raw) + reset
	default:
		return strings.ToUpper(raw)
	}
}

func systemLogLevelString(level SystemLogLevel) string {
	switch level {
	case SystemLogLevelDebug:
		return "debug"
	case SystemLogLevelInfo:
		return "info"
	case SystemLogLevelWarn:
		return "warn"
	case SystemLogLevelError:
		return "error"
	default:
		return "info"
	}
}

func formatKVLine(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if strings.TrimSpace(k) == "" || strings.ContainsAny(k, " \t\r\n") {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := fields[k]
		if shouldSkipKVValue(v) {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatKVValue(v)))
	}
	return strings.Join(parts, " ")
}

func shouldSkipKVValue(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == ""
	}
	return false
}

func formatKVValue(v any) string {
	switch t := v.(type) {
	case string:
		return quoteIfNeeded(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint8:
		return strconv.FormatUint(uint64(t), 10)
	case uint16:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 64)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return quoteIfNeeded(fmt.Sprintf("%v", v))
	}
}

func quoteIfNeeded(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return `""`
	}
	if strings.ContainsRune(s, '\n') || strings.ContainsRune(s, '\r') || strings.ContainsRune(s, '\t') {
		return strconv.Quote(s)
	}
	if strings.ContainsAny(s, ` ="`) {
		return strconv.Quote(s)
	}
	return s
}
