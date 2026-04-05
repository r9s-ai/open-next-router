package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

func run(args []string, stdout, stderr io.Writer) int {
	opts := packOptions{}
	fs := flag.NewFlagSet("onr-pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "path to config yaml")
	fs.StringVar(&opts.cfgPath, "c", "onr.yaml", "path to config yaml (alias of --config)")
	fs.StringVar(&opts.providersPath, "providers", "", "provider DSL source path (dir or merged file)")
	fs.StringVar(&opts.outPath, "out", "providers.conf", "output merged providers file")
	fs.StringVar(&opts.outPath, "o", "providers.conf", "output merged providers file (alias of --out)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, "error: unexpected positional arguments")
		return 2
	}

	sourcePath, err := resolveProviderSource(opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: "+err.Error())
		return 1
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

	res, err := dslconfig.ValidateProvidersPath(sourcePath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "error: validate providers failed: "+err.Error())
		return 1
	}
	for _, w := range res.Warnings {
		_, _ = fmt.Fprintln(stdout, "warn: "+w.String())
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
	cfgPath       string
	providersPath string
	outPath       string
}

func resolveProviderSource(opts packOptions) (string, error) {
	if v := strings.TrimSpace(opts.providersPath); v != "" {
		return v, nil
	}
	cfgPath := strings.TrimSpace(opts.cfgPath)
	if cfgPath != "" {
		cfg, err := config.Load(cfgPath)
		if err == nil {
			path, _ := config.ResolveProviderDSLSource(cfg)
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("config: %w", err)
		}
	}
	path, _ := config.ResolveProviderDSLSource(nil)
	return path, nil
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
