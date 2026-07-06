package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

var errUnexpectedPositionalArgs = errors.New("unexpected positional arguments")

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "version" {
		_, _ = fmt.Fprintln(stdout, detectVersion())
		return 0
	}
	opts := defaultPackOptions()
	fs := newPackFlagSet("onr-pack", stderr)
	addSourceFlags(fs, &opts)
	addOutputFlags(fs, &opts)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.versionOnly, "version", false, "print version and exit")
	fs.BoolVar(&opts.checkOnly, "check-only", false, "validate provider DSL only; do not write bundled output")
	addCheckFlags(fs, &opts)
	fs.Usage = func() {
		printRootUsage(stderr)
		fs.PrintDefaults()
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printRootUsage(stdout)
		fs.SetOutput(stdout)
		fs.PrintDefaults()
		return 0
	}
	if err := parseFlagSet(fs, args); err != nil {
		return errCodeForFlagParse(err)
	}
	if opts.versionOnly {
		_, _ = fmt.Fprintln(stdout, detectVersion())
		return 0
	}
	if err := rejectPositionalArgs(fs, stderr); err != nil {
		return errCodeForFlagParse(err)
	}
	return executePack(opts, stdout, stderr)
}

func executePack(opts packOptions, stdout, stderr io.Writer) int {
	sourcePath := resolveProviderSource(opts)

	res, err := dslconfig.ValidateProvidersPath(sourcePath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: validate providers failed: "+err.Error())
		return 1
	}
	for _, w := range res.Warnings {
		_, _ = fmt.Fprintln(stdout, "warn: "+w.String())
	}
	if err := runExtraChecks(sourcePath, opts.checks); err != nil {
		_, _ = fmt.Fprintln(stderr, "error: extra checks failed")
		_, _ = fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if opts.checkOnly {
		_, _ = fmt.Fprintf(stdout, "validate providers: source=%s providers=%d\n", sourcePath, len(res.LoadedProviders))
		return 0
	}

	outputPath := strings.TrimSpace(opts.outPath)
	if outputPath == "" {
		_, _ = fmt.Fprintln(stderr, "error: output path is empty")
		return 1
	}
	if samePath(sourcePath, outputPath) {
		_, _ = fmt.Fprintln(stderr, "error: output path must differ from provider source path")
		return 1
	}

	content, err := dslconfig.BundleProvidersPath(sourcePath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: bundle providers failed: "+err.Error())
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		_, _ = fmt.Fprintln(stderr, "error: create output dir failed: "+err.Error())
		return 1
	}
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		_, _ = fmt.Fprintln(stderr, "error: write bundle failed: "+err.Error())
		return 1
	}
	if err := validateBundledOutput(content); err != nil {
		_ = os.Remove(outputPath)
		_, _ = fmt.Fprintln(stderr, "error: validate bundled file failed: "+err.Error())
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "bundled providers: source=%s output=%s providers=%d\n", sourcePath, outputPath, len(res.LoadedProviders))
	return 0
}

type packOptions struct {
	providersPath string
	outPath       string
	checkOnly     bool
	versionOnly   bool
	checks        checkList
}

func defaultPackOptions() packOptions {
	return packOptions{
		outPath: "providers.conf",
	}
}

func newPackFlagSet(name string, output io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	return fs
}

func addSourceFlags(fs *flag.FlagSet, opts *packOptions) {
	fs.StringVar(&opts.providersPath, "providers", opts.providersPath, "provider DSL source path (dir or merged file)")
}

func addOutputFlags(fs *flag.FlagSet, opts *packOptions) {
	fs.StringVar(&opts.outPath, "out", opts.outPath, "output bundled providers file")
	fs.StringVar(&opts.outPath, "o", opts.outPath, "output bundled providers file (alias of --out)")
}

func addCheckFlags(fs *flag.FlagSet, opts *packOptions) {
	fs.Var(&opts.checks, "check", "run extra named check after DSL validation; repeat or comma-separate values (known: required-usage, all)")
}

func parseFlagSet(fs *flag.FlagSet, args []string) error {
	return fs.Parse(args)
}

func rejectPositionalArgs(fs *flag.FlagSet, stderr io.Writer) error {
	if fs.NArg() == 0 {
		return nil
	}
	_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
	return errUnexpectedPositionalArgs
}

func errCodeForFlagParse(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}

func printRootUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  onr-pack [--providers PATH] [--out FILE] [--check-only] [--check CHECK]
  onr-pack version

Examples:
  onr-pack --providers ./config/onr.conf --out ./dist/providers.conf
  onr-pack --providers ./config/onr.conf --check-only
  onr-pack --providers ./config/onr.conf --check-only --check required-usage
  onr-pack version

Flags:
`)
}

func detectVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}

	for _, setting := range bi.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			if len(setting.Value) > 12 {
				return "devel+" + setting.Value[:12]
			}
			return "devel+" + setting.Value
		}
	}

	if bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "unknown"
}

func resolveProviderSource(opts packOptions) string {
	if v := strings.TrimSpace(opts.providersPath); v != "" {
		return v
	}
	path, _ := config.ResolveProviderDSLSource(nil)
	return path
}

func samePath(a string, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func validateBundledOutput(content string) error {
	dir, err := os.MkdirTemp("", "onr-pack-validate-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "providers.conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	_, err = dslconfig.ValidateProvidersFile(path)
	return err
}
