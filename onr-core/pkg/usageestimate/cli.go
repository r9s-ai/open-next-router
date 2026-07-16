package usageestimate

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type cliOptions struct {
	model     string
	api       string
	direction string
	body      string
	bodyFile  string
}

// RunCLI estimates token count for a request or response body.
// Body input can come from --body, --body-file, or stdin.
func RunCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := parseCLIOptions(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		}
		return 2
	}

	bodyBytes, source, err := readCLIBody(opts, stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	body := parseCLIBodyValue(bodyBytes)
	tokens, err := EstimateToken(opts.model, opts.api, body, parseEstimateDirection(opts.direction))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "tokens=%d\n", tokens)
	_, _ = fmt.Fprintf(stdout, "model=%s\n", opts.model)
	_, _ = fmt.Fprintf(stdout, "api=%s\n", opts.api)
	_, _ = fmt.Fprintf(stdout, "direction=%s\n", parseEstimateDirection(opts.direction))
	_, _ = fmt.Fprintf(stdout, "source=%s\n", source)
	return 0
}

func parseCLIOptions(args []string, stderr io.Writer) (cliOptions, error) {
	var opts cliOptions
	fs := flag.NewFlagSet("onr-token-estimate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.model, "model", "", "model name")
	fs.StringVar(&opts.model, "m", "", "model name")
	fs.StringVar(&opts.api, "api", apiMessages, "API name, for example claude.messages")
	fs.StringVar(&opts.direction, "direction", string(EstimateInput), "estimate direction: input or output")
	fs.StringVar(&opts.body, "body", "", "request or response body JSON/text")
	fs.StringVar(&opts.bodyFile, "body-file", "", "path to request or response body file")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if strings.TrimSpace(opts.model) == "" {
		return opts, errors.New("model is required")
	}
	if opts.body != "" && opts.bodyFile != "" {
		return opts, errors.New("--body and --body-file cannot be used together")
	}
	return opts, nil
}

func readCLIBody(opts cliOptions, stdin io.Reader) ([]byte, string, error) {
	if opts.body != "" {
		return []byte(opts.body), "body", nil
	}
	if opts.bodyFile != "" {
		b, err := os.ReadFile(opts.bodyFile)
		if err != nil {
			return nil, "", fmt.Errorf("read body file: %w", err)
		}
		return b, opts.bodyFile, nil
	}
	b, err := io.ReadAll(stdin)
	if err != nil {
		return nil, "", fmt.Errorf("read stdin: %w", err)
	}
	return b, "stdin", nil
}

func parseCLIBodyValue(body []byte) any {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(body, &v); err == nil {
		return v
	}
	return string(body)
}

func parseEstimateDirection(direction string) EstimateDirection {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "output", "response", "completion", string(EstimateOutput):
		return EstimateOutput
	default:
		return EstimateInput
	}
}
