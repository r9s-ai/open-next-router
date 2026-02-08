package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/pkg/balancequery"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
)

func runBalance(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: onr-admin balance get [flags]")
	}
	switch args[0] {
	case "get":
		return runBalanceGet(args[1:])
	default:
		return fmt.Errorf("unknown balance subcommand %q", args[0])
	}
}

func runBalanceGet(args []string) error {
	var cfgPath string
	var keysPath string
	var providersDir string
	var provider string
	var providersCSV string
	var allProviders bool
	var failFast bool
	var api string
	var stream bool
	var upstreamKey string
	var baseURLOv string
	var debug bool

	fs := flag.NewFlagSet("balance get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&providersDir, "providers-dir", "", "providers dir path")
	fs.StringVar(&provider, "p", "", "provider name")
	fs.StringVar(&provider, "provider", "", "provider name")
	fs.StringVar(&providersCSV, "providers", "", "providers list for batch mode, comma separated")
	fs.BoolVar(&allProviders, "all", false, "query all providers in DSL registry")
	fs.BoolVar(&failFast, "fail-fast", false, "stop on first error in batch mode")
	fs.StringVar(&api, "api", "chat.completions", "api name for DSL match selection")
	fs.BoolVar(&stream, "stream", false, "stream flag for DSL match selection")
	fs.StringVar(&upstreamKey, "uk", "", "upstream api key")
	fs.StringVar(&upstreamKey, "upstream-key", "", "upstream api key")
	fs.StringVar(&baseURLOv, "base-url", "", "override base url")
	fs.BoolVar(&debug, "debug", false, "print upstream raw response body")
	if err := fs.Parse(args); err != nil {
		return err
	}

	api = strings.TrimSpace(api)
	if api == "" {
		return errors.New("missing api: use --api")
	}

	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, _ = store.ResolveDataPaths(cfg, keysPath, "")
	if strings.TrimSpace(providersDir) == "" {
		if cfg != nil && strings.TrimSpace(cfg.Providers.Dir) != "" {
			providersDir = strings.TrimSpace(cfg.Providers.Dir)
		} else {
			providersDir = "./config/providers"
		}
	}

	reg := dslconfig.NewRegistry()
	if _, err := reg.ReloadFromDir(providersDir); err != nil {
		return fmt.Errorf("load providers dir %s failed: %w", providersDir, err)
	}

	targets, err := resolveTargetProviders(reg, provider, providersCSV, allProviders)
	if err != nil {
		return err
	}

	ks, err := keystore.Load(strings.TrimSpace(keysPath))
	if err != nil {
		return fmt.Errorf("load keys.yaml failed: %w", err)
	}

	success := 0
	fail := 0
	var debugOut io.Writer
	if debug {
		debugOut = os.Stdout
	}
	for _, p := range targets {
		pf, ok := reg.GetProvider(p)
		if !ok {
			fail++
			fmt.Printf("provider=%s error=%q\n", p, "provider not found in registry")
			if failFast {
				return fmt.Errorf("provider %q not found in %s", p, providersDir)
			}
			continue
		}

		key := strings.TrimSpace(upstreamKey)
		baseURL := strings.TrimSpace(baseURLOv)
		if key == "" || baseURL == "" {
			next, found := ks.NextKey(p)
			if !found {
				fail++
				fmt.Printf("provider=%s error=%q\n", p, "no upstream key in keys.yaml")
				if failFast {
					return fmt.Errorf("provider %q has no key in %s (or env override)", p, keysPath)
				}
				continue
			}
			if key == "" {
				key = strings.TrimSpace(next.Value)
			}
			if baseURL == "" {
				baseURL = strings.TrimSpace(next.BaseURLOverride)
			}
		}
		if key == "" {
			fail++
			fmt.Printf("provider=%s error=%q\n", p, "upstream key is empty")
			if failFast {
				return errors.New("upstream key is empty")
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, qerr := balancequery.Query(ctx, balancequery.Params{
			Provider: p,
			File:     pf,
			Meta: dslmeta.Meta{
				API:      api,
				IsStream: stream,
			},
			BaseURL:  baseURL,
			APIKey:   key,
			DebugOut: debugOut,
		})
		cancel()
		if qerr != nil {
			fail++
			fmt.Printf("provider=%s error=%q\n", p, qerr.Error())
			if failFast {
				return qerr
			}
			continue
		}
		success++
		usedText := "N/A"
		if result.Used != nil {
			usedText = fmt.Sprintf("%.6f", *result.Used)
		}
		fmt.Printf("provider=%s mode=%s unit=%s balance=%.6f used=%s\n", result.Provider, result.Mode, result.Unit, result.Balance, usedText)
	}
	if len(targets) > 1 {
		fmt.Printf("summary total=%d success=%d failed=%d\n", len(targets), success, fail)
	}
	if fail > 0 {
		return fmt.Errorf("batch query completed with %d failure(s)", fail)
	}
	return nil
}

func resolveTargetProviders(reg *dslconfig.Registry, provider, providersCSV string, allProviders bool) ([]string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p != "" {
		return []string{p}, nil
	}
	if strings.TrimSpace(providersCSV) != "" {
		parts := strings.FieldsFunc(strings.TrimSpace(providersCSV), func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
		})
		out := make([]string, 0, len(parts))
		seen := map[string]struct{}{}
		for _, it := range parts {
			name := strings.ToLower(strings.TrimSpace(it))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
		if len(out) == 0 {
			return nil, errors.New("empty providers list")
		}
		sort.Strings(out)
		return out, nil
	}
	if allProviders {
		names := reg.ListProviderNames()
		if len(names) == 0 {
			return nil, errors.New("no providers found in registry")
		}
		return names, nil
	}
	return nil, errors.New("missing provider: use --provider/-p, --providers or --all")
}
