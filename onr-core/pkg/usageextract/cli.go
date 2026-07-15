// Package usageextract implements the onr-usage-extract command.
package usageextract

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type cliOptions struct {
	usage         string
	usageFile     string
	usageModeDSL  string
	usageModeFile string
	usageMode     string
	request       string
	requestFile   string
	derived       string
	derivedFile   string
	stream        bool
}

// RunCLI extracts usage facts from a response JSON object or SSE stream.
// Usage input can come from --usage, --usage-file, or stdin.
func RunCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := parseCLIOptions(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		}
		return 2
	}

	usageBody, err := readRequiredInput(opts.usage, opts.usageFile, "--usage", "--usage-file", stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	modeContent, modePath, err := readModeInput(opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	requestBody, err := readOptionalInput(opts.request, opts.requestFile, "--request-file")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	derivedBody, err := readOptionalInput(opts.derived, opts.derivedFile, "--derived-file")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	if len(requestBody) > 0 {
		if _, err := parseJSONObject(requestBody, "request"); err != nil {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	var derived map[string]any
	if len(derivedBody) > 0 {
		derived, err = parseJSONObject(derivedBody, "derived")
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	cfg, err := dslconfig.ParseUsageMode(modePath, modeContent, opts.usageMode)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	meta := &dslmeta.Meta{
		IsStream:     opts.stream,
		RequestBody:  requestBody,
		DerivedUsage: derived,
	}

	facts, err := extractFacts(meta, &cfg, usageBody)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	encoded, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: encode usage facts: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(encoded))
	return 0
}

func parseCLIOptions(args []string, stderr io.Writer) (cliOptions, error) {
	var opts cliOptions
	fs := flag.NewFlagSet("onr-usage-extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.usage, "usage", "", "response JSON or SSE text")
	fs.StringVar(&opts.usageFile, "usage-file", "", "path to response JSON or SSE file")
	fs.StringVar(&opts.usageModeDSL, "usage-mode-dsl", "", "usage_mode DSL content")
	fs.StringVar(&opts.usageModeFile, "usage-mode-file", "", "path to usage_mode DSL file")
	fs.StringVar(&opts.usageMode, "usage-mode", "", "usage_mode name to select")
	fs.BoolVar(&opts.stream, "stream", false, "treat usage input as text/event-stream")
	fs.StringVar(&opts.request, "request", "", "optional request JSON for source=request facts")
	fs.StringVar(&opts.requestFile, "request-file", "", "path to optional request JSON file")
	fs.StringVar(&opts.derived, "derived", "", "optional derived usage JSON for source=derived facts")
	fs.StringVar(&opts.derivedFile, "derived-file", "", "path to optional derived usage JSON file")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	if opts.usage != "" && opts.usageFile != "" {
		return opts, errors.New("--usage and --usage-file cannot be used together")
	}
	if opts.usageModeDSL != "" && opts.usageModeFile != "" {
		return opts, errors.New("--usage-mode-dsl and --usage-mode-file cannot be used together")
	}
	if opts.usageModeDSL == "" && opts.usageModeFile == "" {
		return opts, errors.New("one of --usage-mode-dsl or --usage-mode-file is required")
	}
	if opts.request != "" && opts.requestFile != "" {
		return opts, errors.New("--request and --request-file cannot be used together")
	}
	if opts.derived != "" && opts.derivedFile != "" {
		return opts, errors.New("--derived and --derived-file cannot be used together")
	}
	return opts, nil
}

func readModeInput(opts cliOptions) (content string, path string, err error) {
	if opts.usageModeDSL != "" {
		return opts.usageModeDSL, "<usage-mode-dsl>", nil
	}
	b, err := os.ReadFile(opts.usageModeFile) // #nosec G304 -- CLI file path is explicit user input.
	if err != nil {
		return "", "", fmt.Errorf("read usage mode file: %w", err)
	}
	return string(b), opts.usageModeFile, nil
}

func readRequiredInput(value, path, valueFlag, fileFlag string, stdin io.Reader) ([]byte, error) {
	if value != "" {
		return []byte(value), nil
	}
	if path != "" {
		b, err := os.ReadFile(path) // #nosec G304 -- CLI file path is explicit user input.
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fileFlag, err)
		}
		return b, nil
	}
	b, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, fmt.Errorf("%s, %s, or stdin is required", valueFlag, fileFlag)
	}
	return b, nil
}

func readOptionalInput(value, path, fileFlag string) ([]byte, error) {
	if value != "" {
		return []byte(value), nil
	}
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path) // #nosec G304 -- CLI file path is explicit user input.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fileFlag, err)
	}
	return b, nil
}

func parseJSONObject(body []byte, label string) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("parse %s JSON: %w", label, err)
	}
	if root == nil {
		return nil, fmt.Errorf("%s JSON must be an object", label)
	}
	return root, nil
}

func extractFacts(meta *dslmeta.Meta, cfg *dslconfig.UsageExtractConfig, body []byte) ([]dslconfig.UsageFact, error) {
	if meta.IsStream {
		agg := dslconfig.NewStreamMetricsAggregator(meta, cfg, nil)
		agg.OnSSETail(body)
		usage, _, _, ok := agg.Result()
		if !ok || usage == nil || len(usage.DebugFacts) == 0 {
			return []dslconfig.UsageFact{}, nil
		}
		return usage.DebugFacts, nil
	}
	usage, _, err := dslconfig.ExtractUsage(meta, cfg, body)
	if err != nil {
		return nil, err
	}
	if usage == nil || len(usage.DebugFacts) == 0 {
		return []dslconfig.UsageFact{}, nil
	}
	return usage.DebugFacts, nil
}
