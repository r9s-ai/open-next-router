package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/streamtext"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
)

type options struct {
	file           string
	api            string
	route          string
	model          string
	allowTruncated bool
	debugID        string
	debugDir       string
}

type dumpEntry struct {
	ID       any      `json:"id"`
	Request  dumpSide `json:"request"`
	Response dumpSide `json:"response"`
}

type dumpSide struct {
	Body dumpBody `json:"body"`
}

type dumpBody struct {
	Format    string          `json:"format"`
	Size      int             `json:"size"`
	Truncated bool            `json:"truncated"`
	Content   json.RawMessage `json:"content"`
	Events    []dumpSSEEvent  `json:"events"`
}

type dumpSSEEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type estimateRow struct {
	Status       string
	ID           string
	Stage        string
	InputActual  string
	InputEst     string
	InputDelta   string
	OutputActual string
	OutputEst    string
	OutputDelta  string
	Reason       string
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}
	api, err := resolveAPI(opts.api, opts.route)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}

	entries, err := readDumpEntries(opts.file)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: "+err.Error())
		return 1
	}

	if strings.TrimSpace(opts.debugID) != "" {
		return runDebugDump(entries, api, opts.model, opts.file, opts.debugID, opts.debugDir, stdout, stderr)
	}

	cfg := &usageestimate.Config{}
	usageestimate.ApplyDefaults(cfg)
	processed := 0
	skipped := 0
	rows := make([]estimateRow, 0, len(entries))
	for _, entry := range entries {
		in, err := buildEstimateInput(entry, api, opts.model, opts.allowTruncated)
		if err != nil {
			rows = append(rows, skippedRow(entry.ID, err.Error()))
			skipped++
			continue
		}
		actual := in.UpstreamUsage
		if !hasCompleteUsage(actual) {
			rows = append(rows, skippedRow(entry.ID, "token usage not detected"))
			skipped++
			continue
		}

		estimateIn := in
		estimateIn.UpstreamUsage = nil
		estimated := usageestimate.Estimate(cfg, estimateIn)
		if estimated.Usage == nil {
			rows = append(rows, skippedRow(entry.ID, "estimate unavailable"))
			skipped++
			continue
		}

		processed++
		rows = append(rows, estimatedRow(entry.ID, estimated.Stage, actual, estimated.Usage))
	}
	printRows(stdout, rows)
	_, _ = fmt.Fprintf(stdout, "summary entries=%d estimated=%d skipped=%d\n", len(entries), processed, skipped)
	return 0
}

func skippedRow(id any, reason string) estimateRow {
	return estimateRow{
		Status: "skipped",
		ID:     fmt.Sprint(id),
		Reason: reason,
	}
}

func estimatedRow(id any, stage string, actual, estimated *dslconfig.Usage) estimateRow {
	return estimateRow{
		Status:       "estimated",
		ID:           fmt.Sprint(id),
		Stage:        stage,
		InputActual:  fmt.Sprintf("%d", actual.InputTokens),
		InputEst:     fmt.Sprintf("%d", estimated.InputTokens),
		InputDelta:   fmt.Sprintf("%+.2f%%", percentDelta(estimated.InputTokens, actual.InputTokens)),
		OutputActual: fmt.Sprintf("%d", actual.OutputTokens),
		OutputEst:    fmt.Sprintf("%d", estimated.OutputTokens),
		OutputDelta:  fmt.Sprintf("%+.2f%%", percentDelta(estimated.OutputTokens, actual.OutputTokens)),
	}
}

func printRows(out io.Writer, rows []estimateRow) {
	headers := []string{"status", "id", "stage", "in.actual", "in.est", "in.delta", "out.actual", "out.est", "out.delta", "reason"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		values := rowValues(row)
		for i, value := range values {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}

	printTableLine(out, headers, widths)
	separators := make([]string, len(headers))
	for i, w := range widths {
		separators[i] = strings.Repeat("-", w)
	}
	printTableLine(out, separators, widths)
	for _, row := range rows {
		printTableLine(out, rowValues(row), widths)
	}
}

func rowValues(row estimateRow) []string {
	return []string{
		row.Status,
		row.ID,
		row.Stage,
		row.InputActual,
		row.InputEst,
		row.InputDelta,
		row.OutputActual,
		row.OutputEst,
		row.OutputDelta,
		row.Reason,
	}
}

func printTableLine(out io.Writer, values []string, widths []int) {
	for i, value := range values {
		if i > 0 {
			_, _ = fmt.Fprint(out, "  ")
		}
		if isNumericColumn(i) {
			_, _ = fmt.Fprintf(out, "%*s", widths[i], value)
			continue
		}
		_, _ = fmt.Fprintf(out, "%-*s", widths[i], value)
	}
	_, _ = fmt.Fprintln(out)
}

func isNumericColumn(index int) bool {
	switch index {
	case 1, 3, 4, 5, 6, 7, 8:
		return true
	default:
		return false
	}
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{}
	fs := flag.NewFlagSet("onr-token-estimate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.file, "file", "", "path to dump json file")
	fs.StringVar(&opts.file, "f", "", "path to dump json file (alias of --file)")
	fs.StringVar(&opts.api, "api", "", "usage estimate API name")
	fs.StringVar(&opts.route, "route", "", "route alias for API name")
	fs.StringVar(&opts.model, "model", "", "model name")
	fs.StringVar(&opts.model, "m", "", "model name (alias of --model)")
	fs.BoolVar(&opts.allowTruncated, "allow-truncated", false, "allow truncated dump bodies")
	fs.StringVar(&opts.debugID, "debug-id", "", "dump id to write extracted request and response text for, without estimating")
	fs.StringVar(&opts.debugDir, "debug-dir", "", "directory to write --debug-id extracted files; default is dump file directory")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, errors.New("unexpected positional arguments")
	}
	if strings.TrimSpace(opts.file) == "" {
		return options{}, errors.New("missing --file")
	}
	if strings.TrimSpace(opts.model) == "" {
		return options{}, errors.New("missing --model")
	}
	return opts, nil
}

func shouldDebugEntry(debugID string, id any) bool {
	debugID = strings.TrimSpace(debugID)
	if debugID == "" {
		return false
	}
	return fmt.Sprint(id) == debugID
}

var debugFileIDReplacer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func runDebugDump(entries []dumpEntry, api, model, dumpPath, debugID, debugDir string, stdout, stderr io.Writer) int {
	for _, entry := range entries {
		if shouldDebugEntry(debugID, entry.ID) {
			if err := writeDebugDumpFiles(api, model, dumpPath, debugID, debugDir, entry, stdout); err != nil {
				_, _ = fmt.Fprintln(stderr, "error: "+err.Error())
				return 1
			}
			return 0
		}
	}
	_, _ = fmt.Fprintf(stderr, "error: debug dump id %s not found\n", strings.TrimSpace(debugID))
	return 1
}

func writeDebugDumpFiles(api, model, dumpPath, debugID, debugDir string, entry dumpEntry, out io.Writer) error {
	requestText, requestErr := extractRequestDebugText(api, entry.Request.Body)
	if requestErr != nil {
		return fmt.Errorf("extract request debug text: %w", requestErr)
	}
	responseText, responseErr := extractResponseDebugText(api, model, entry.Response.Body)
	if responseErr != nil {
		return fmt.Errorf("extract response debug text: %w", responseErr)
	}

	dir := strings.TrimSpace(debugDir)
	if dir == "" {
		dir = filepath.Dir(strings.TrimSpace(dumpPath))
		if dir == "" || dir == "." {
			dir = "."
		}
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create debug dir: %w", err)
	}
	safeID := safeDebugFileID(debugID)
	requestPath := filepath.Join(dir, fmt.Sprintf("onr-token-estimate-%s-request.txt", safeID))
	responsePath := filepath.Join(dir, fmt.Sprintf("onr-token-estimate-%s-response.txt", safeID))

	if err := os.WriteFile(requestPath, []byte(requestText), 0o600); err != nil {
		return fmt.Errorf("write request debug file: %w", err)
	}
	if err := os.WriteFile(responsePath, []byte(responseText), 0o600); err != nil {
		return fmt.Errorf("write response debug file: %w", err)
	}

	_, _ = fmt.Fprintf(out, "debug dump id=%v\n", entry.ID)
	_, _ = fmt.Fprintf(out, "request_file=%s request_chars=%d\n", requestPath, len([]rune(requestText)))
	_, _ = fmt.Fprintf(out, "response_file=%s response_chars=%d\n", responsePath, len([]rune(responseText)))
	return nil
}

func safeDebugFileID(id string) string {
	safe := strings.Trim(debugFileIDReplacer.ReplaceAllString(strings.TrimSpace(id), "_"), "_")
	if safe == "" {
		return "dump"
	}
	return safe
}

func extractRequestDebugText(api string, body dumpBody) (string, error) {
	switch strings.ToLower(strings.TrimSpace(body.Format)) {
	case "json":
		if len(bytes.TrimSpace(body.Content)) == 0 {
			return "", nil
		}
		return usageestimate.ExtractRequestText(api, body.Content, 0), nil
	case "", "empty":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported request format %q", body.Format)
	}
}

func extractResponseDebugText(api, model string, body dumpBody) (string, error) {
	switch strings.ToLower(strings.TrimSpace(body.Format)) {
	case "sse":
		return extractSSEDeltaText(api, body.Events)
	case "json":
		if len(bytes.TrimSpace(body.Content)) == 0 {
			return "", nil
		}
		return usageestimate.ExtractResponseTextForModel(api, model, body.Content, 0), nil
	case "", "empty":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported response format %q", body.Format)
	}
}

func resolveAPI(apiFlag, routeFlag string) (string, error) {
	api := strings.TrimSpace(apiFlag)
	route := strings.ToLower(strings.TrimSpace(routeFlag))
	if api != "" && route == "" {
		return api, nil
	}
	if route == "" {
		return "", errors.New("missing --api or --route")
	}

	mapped, ok := map[string]string{
		"chat.completions":               "chat.completions",
		"openai-chat":                    "chat.completions",
		"openai-chat-completions":        "chat.completions",
		"responses":                      "responses",
		"openai-responses":               "responses",
		"claude.messages":                "claude.messages",
		"anthropic-messages":             "claude.messages",
		"claude-messages":                "claude.messages",
		"embeddings":                     "embeddings",
		"gemini.generatecontent":         "gemini.generateContent",
		"gemini-generate-content":        "gemini.generateContent",
		"gemini.streamgeneratecontent":   "gemini.streamGenerateContent",
		"gemini-stream-generate-content": "gemini.streamGenerateContent",
	}[route]
	if !ok {
		return "", fmt.Errorf("unknown route %q", routeFlag)
	}
	if api != "" && strings.ToLower(api) != strings.ToLower(mapped) {
		return "", fmt.Errorf("--api %q conflicts with --route %q (%s)", api, routeFlag, mapped)
	}
	return mapped, nil
}

func readDumpEntries(path string) ([]dumpEntry, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("dump file path is empty")
	}
	// #nosec G304 -- CLI reads a user-provided local dump path.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dump file: %w", err)
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil, errors.New("dump file is empty")
	}

	if b[0] == '[' {
		var entries []dumpEntry
		if err := json.Unmarshal(b, &entries); err != nil {
			entries, fallbackErr := readLooseJSONArrayEntries(b)
			if fallbackErr != nil {
				return nil, fmt.Errorf("parse dump json array: %w", err)
			}
			return entries, nil
		}
		if len(entries) == 0 {
			return nil, errors.New("dump file has no entries")
		}
		return entries, nil
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	var entries []dumpEntry
	for {
		var entry dumpEntry
		if err := dec.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse dump json stream: %w", err)
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("dump file has no entries")
	}
	return entries, nil
}

func readLooseJSONArrayEntries(b []byte) ([]dumpEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil, errors.New("not a json array")
	}

	var entries []dumpEntry
	for dec.More() {
		var entry dumpEntry
		if err := dec.Decode(&entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("dump file has no entries")
	}
	return entries, nil
}

func buildEstimateInput(entry dumpEntry, api, model string, allowTruncated bool) (usageestimate.Input, error) {
	req, _, err := extractDumpBody("request", entry.Request.Body, api, allowTruncated)
	if err != nil {
		return usageestimate.Input{}, err
	}
	resp, stream, err := extractDumpBody("response", entry.Response.Body, api, allowTruncated)
	if err != nil {
		return usageestimate.Input{}, err
	}
	return usageestimate.Input{
		API:           strings.TrimSpace(api),
		Model:         strings.TrimSpace(model),
		UpstreamUsage: extractUpstreamUsage(entry, api),
		RequestBody:   req,
		ResponseBody:  resp,
		StreamTail:    stream,
	}, nil
}

func extractDumpBody(label string, body dumpBody, api string, allowTruncated bool) (jsonBody []byte, sseBody []byte, err error) {
	if body.Truncated && !allowTruncated {
		return nil, nil, fmt.Errorf("%s body is truncated", label)
	}
	switch strings.ToLower(strings.TrimSpace(body.Format)) {
	case "", "empty":
		return nil, nil, nil
	case "json":
		if len(bytes.TrimSpace(body.Content)) == 0 {
			return nil, nil, fmt.Errorf("%s json body content is empty", label)
		}
		if !json.Valid(body.Content) {
			return nil, nil, fmt.Errorf("%s json body content is invalid", label)
		}
		return append([]byte(nil), body.Content...), nil, nil
	case "sse":
		text, err := extractSSEDeltaText(api, body.Events)
		if err != nil {
			return nil, nil, fmt.Errorf("%s sse body: %w", label, err)
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil, nil
		}
		b, err := buildResponseBodyFromText(api, text)
		if err != nil {
			return nil, nil, fmt.Errorf("%s sse body: %w", label, err)
		}
		return b, nil, nil
	default:
		return nil, nil, fmt.Errorf("%s body has unsupported format %q", label, body.Format)
	}
}

func extractSSEDeltaText(api string, events []dumpSSEEvent) (string, error) {
	var out strings.Builder
	for i, ev := range events {
		if len(bytes.TrimSpace(ev.Data)) == 0 {
			return "", fmt.Errorf("event %d has empty data", i)
		}
		if text := streamtext.ExtractDeltaText(api, ev.Data); text != "" {
			out.WriteString(text)
		}
	}
	return out.String(), nil
}

func buildResponseBodyFromText(api, text string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(api)) {
	case "chat.completions":
		return json.Marshal(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{"content": text},
				},
			},
		})
	case "claude.messages":
		return json.Marshal(map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": text},
			},
		})
	case "gemini.generatecontent", "gemini.streamgeneratecontent":
		return json.Marshal(map[string]any{
			"candidates": []any{
				map[string]any{
					"content": map[string]any{
						"parts": []any{
							map[string]any{"text": text},
						},
					},
				},
			},
		})
	default:
		return json.Marshal(map[string]any{
			"output": []any{
				map[string]any{
					"type": "message",
					"content": []any{
						map[string]any{"type": "output_text", "text": text},
					},
				},
			},
		})
	}
}

func extractUpstreamUsage(entry dumpEntry, api string) *dslconfig.Usage {
	usage := &dslconfig.Usage{}
	includeAnthropicCacheInput := isAnthropicMessagesAPI(api)
	mergeUsageFromBody(usage, entry.Response.Body, includeAnthropicCacheInput)
	normalizeUsage(usage)
	if includeAnthropicCacheInput && usage.TotalTokens < usage.InputTokens+usage.OutputTokens {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.PromptTokens == 0 &&
		usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	return usage
}

func isAnthropicMessagesAPI(api string) bool {
	return strings.EqualFold(strings.TrimSpace(api), "claude.messages")
}

func mergeUsageFromBody(out *dslconfig.Usage, body dumpBody, includeAnthropicCacheInput bool) {
	switch strings.ToLower(strings.TrimSpace(body.Format)) {
	case "json":
		var obj any
		if json.Unmarshal(body.Content, &obj) == nil {
			mergeUsageFromValue(out, obj, includeAnthropicCacheInput)
		}
	case "sse":
		for _, ev := range body.Events {
			var obj any
			if json.Unmarshal(ev.Data, &obj) == nil {
				mergeUsageFromValue(out, obj, includeAnthropicCacheInput)
			}
		}
	}
}

func mergeUsageFromValue(out *dslconfig.Usage, v any, includeAnthropicCacheInput bool) {
	switch t := v.(type) {
	case map[string]any:
		if hasUsageTokenFields(t) {
			mergeUsageMap(out, t, includeAnthropicCacheInput)
		}
		for k, vv := range t {
			if strings.EqualFold(k, "usage") {
				if m, ok := vv.(map[string]any); ok {
					mergeUsageMap(out, m, includeAnthropicCacheInput)
				}
			}
			mergeUsageFromValue(out, vv, includeAnthropicCacheInput)
		}
	case []any:
		for _, it := range t {
			mergeUsageFromValue(out, it, includeAnthropicCacheInput)
		}
	}
}

func hasUsageTokenFields(m map[string]any) bool {
	for _, key := range []string{
		"input_tokens",
		"output_tokens",
		"prompt_tokens",
		"completion_tokens",
		"total_tokens",
		"cache_creation_input_tokens",
		"cache_read_input_tokens",
	} {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func mergeUsageMap(out *dslconfig.Usage, m map[string]any, includeAnthropicCacheInput bool) {
	inputTokens := intField(m, "input_tokens")
	cacheReadTokens := intField(m, "cache_read_input_tokens")
	cacheWriteTokens := intField(m, "cache_creation_input_tokens")
	if includeAnthropicCacheInput {
		inputTokens += cacheReadTokens + cacheWriteTokens
		if cacheReadTokens > 0 || cacheWriteTokens > 0 {
			if out.InputTokenDetails == nil {
				out.InputTokenDetails = &dslconfig.ResponseTokenDetails{}
			}
			setMax(&out.InputTokenDetails.CachedTokens, cacheReadTokens)
			setMax(&out.InputTokenDetails.CacheWriteTokens, cacheWriteTokens)
		}
	}
	setMax(&out.InputTokens, inputTokens)
	setMax(&out.OutputTokens, intField(m, "output_tokens"))
	setMax(&out.PromptTokens, intField(m, "prompt_tokens"))
	setMax(&out.CompletionTokens, intField(m, "completion_tokens"))
	setMax(&out.TotalTokens, intField(m, "total_tokens"))
}

func intField(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		if t > 0 {
			return int(t)
		}
	case int:
		if t > 0 {
			return t
		}
	case json.Number:
		n, err := t.Int64()
		if err == nil && n > 0 {
			return int(n)
		}
	}
	return 0
}

func setMax(dst *int, v int) {
	if v > *dst {
		*dst = v
	}
}

func normalizeUsage(u *dslconfig.Usage) {
	if u.InputTokens == 0 && u.PromptTokens > 0 {
		u.InputTokens = u.PromptTokens
	}
	if u.OutputTokens == 0 && u.CompletionTokens > 0 {
		u.OutputTokens = u.CompletionTokens
	}
	if u.PromptTokens == 0 && u.InputTokens > 0 {
		u.PromptTokens = u.InputTokens
	}
	if u.CompletionTokens == 0 && u.OutputTokens > 0 {
		u.CompletionTokens = u.OutputTokens
	}
	if u.TotalTokens == 0 && (u.InputTokens > 0 || u.OutputTokens > 0) {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
}

func hasCompleteUsage(u *dslconfig.Usage) bool {
	return u != nil && u.InputTokens > 0 && u.OutputTokens > 0 && u.TotalTokens > 0
}

func percentDelta(estimated, actual int) float64 {
	if actual == 0 {
		return 0
	}
	return float64(estimated-actual) * 100 / float64(actual)
}
