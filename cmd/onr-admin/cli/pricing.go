package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newPricingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pricing",
		Short: "Sync model pricing into price.yaml",
	}
	cmd.AddCommand(newPricingSyncCmd(), newPricingProvidersCmd())
	return cmd
}

type pricingSyncOptions struct {
	cfgPath      string
	outPath      string
	url          string
	backup       bool
	provider     string
	providersCSV string
	modelsCSV    string
}

type pricingProvidersOptions struct {
	url    string
	search string
}

func newPricingSyncCmd() *cobra.Command {
	opts := pricingSyncOptions{
		cfgPath: "onr.yaml",
		outPath: "./price.yaml",
		url:     pricing.DefaultCatalogURL,
		backup:  true,
	}
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch from models.dev and write price.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPricingSyncWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.outPath, "out", "./price.yaml", "output price yaml path")
	fs.StringVar(&opts.url, "url", pricing.DefaultCatalogURL, "catalog url")
	fs.BoolVar(&opts.backup, "backup", true, "backup old price file before overwrite")
	fs.StringVarP(&opts.provider, "provider", "p", "", "provider name")
	fs.StringVar(&opts.providersCSV, "providers", "", "providers list, comma separated")
	fs.StringVar(&opts.modelsCSV, "models", "", "models list, comma separated")
	return cmd
}

func runPricingSyncWithOptions(opts pricingSyncOptions) error {
	providers, err := resolvePricingSyncProviders(opts)
	if err != nil {
		return err
	}
	models := parseCSVList(opts.modelsCSV)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fetch, err := pricing.FetchCatalog(ctx, nil, strings.TrimSpace(opts.url))
	if err != nil {
		return err
	}

	allPrices := make([]pricing.ModelPrice, 0, 64)
	usedProviders := make([]string, 0, len(providers))
	skippedProviders := 0
	for _, provider := range providers {
		if _, ok := fetch.Catalog.ResolveProviderID(provider); !ok {
			skippedProviders++
			fmt.Fprintf(os.Stdout, "skip provider=%s reason=not_found_in_models_dev\n", provider)
			continue
		}
		prices, resolvedProvider, xerr := fetch.Catalog.ExtractPrices(provider, models)
		if xerr != nil {
			return xerr
		}
		usedProviders = append(usedProviders, resolvedProvider)
		allPrices = append(allPrices, prices...)
	}
	if len(allPrices) == 0 {
		return fmt.Errorf("no provider matched models.dev (skipped=%d)", skippedProviders)
	}
	sort.Slice(allPrices, func(i, j int) bool {
		if allPrices[i].Provider != allPrices[j].Provider {
			return allPrices[i].Provider < allPrices[j].Provider
		}
		return allPrices[i].Model < allPrices[j].Model
	})

	priceFile := pricing.BuildPriceFile(fetch, usedProviders, allPrices)
	data, err := yaml.Marshal(&priceFile)
	if err != nil {
		return fmt.Errorf("encode price yaml failed: %w", err)
	}
	if err := store.WriteAtomic(strings.TrimSpace(opts.outPath), data, opts.backup); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "pricing synced: providers=%d models=%d out=%s fetched_at=%s\n",
		len(priceFile.Source.Providers),
		len(priceFile.Entries),
		strings.TrimSpace(opts.outPath),
		priceFile.Source.FetchedAt,
	)
	return nil
}

func resolvePricingSyncProviders(opts pricingSyncOptions) ([]string, error) {
	if providers, err := parseProviderTargets(opts.provider, opts.providersCSV); err == nil && len(providers) > 0 {
		return providers, nil
	}
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(opts.cfgPath))
	providersDir := "./config/providers"
	if cfg != nil && strings.TrimSpace(cfg.Providers.Dir) != "" {
		providersDir = strings.TrimSpace(cfg.Providers.Dir)
	}
	reg := dslconfig.NewRegistry()
	if _, err := reg.ReloadFromDir(providersDir); err != nil {
		return nil, fmt.Errorf("load providers dir %s failed: %w", providersDir, err)
	}
	providers := reg.ListProviderNames()
	if len(providers) == 0 {
		return nil, errors.New("missing provider: use --provider/-p, --providers, or configure providers in onr.yaml")
	}
	return providers, nil
}

func newPricingProvidersCmd() *cobra.Command {
	opts := pricingProvidersOptions{
		url: pricing.DefaultCatalogURL,
	}
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List providers from models.dev",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPricingProvidersWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVar(&opts.url, "url", pricing.DefaultCatalogURL, "catalog url")
	fs.StringVar(&opts.search, "search", "", "keyword filter for provider id/name")
	return cmd
}

func runPricingProvidersWithOptions(opts pricingProvidersOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fetch, err := pricing.FetchCatalog(ctx, nil, strings.TrimSpace(opts.url))
	if err != nil {
		return err
	}
	rows := buildProviderRows(fetch.Catalog, strings.TrimSpace(opts.search))
	if len(rows) == 0 {
		fmt.Fprintln(os.Stdout, "no providers matched")
		return nil
	}
	for _, row := range rows {
		fmt.Fprintln(os.Stdout, row)
	}
	fmt.Fprintf(os.Stdout, "total=%d fetched_at=%s\n", len(rows), fetch.FetchedAt.Format(time.RFC3339))
	return nil
}

func buildProviderRows(catalog pricing.Catalog, search string) []string {
	keys := make([]string, 0, len(catalog.Providers))
	for providerID := range catalog.Providers {
		keys = append(keys, providerID)
	}
	sort.Strings(keys)

	needle := strings.ToLower(strings.TrimSpace(search))
	rows := make([]string, 0, len(keys))
	for _, providerID := range keys {
		p := catalog.Providers[providerID]
		name := strings.TrimSpace(p.Name)
		if needle != "" {
			idText := strings.ToLower(strings.TrimSpace(providerID))
			nameText := strings.ToLower(name)
			if !strings.Contains(idText, needle) && !strings.Contains(nameText, needle) {
				continue
			}
		}
		if name == "" {
			name = "-"
		}
		rows = append(rows, fmt.Sprintf("provider=%s name=%q models=%d", providerID, name, len(p.Models)))
	}
	return rows
}

func parseProviderTargets(provider, providersCSV string) ([]string, error) {
	p := strings.TrimSpace(provider)
	if p != "" {
		return []string{strings.ToLower(p)}, nil
	}
	parts := parseCSVList(providersCSV)
	if len(parts) == 0 {
		return nil, errors.New("missing provider: use --provider/-p or --providers")
	}
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, item := range parts {
		name := strings.ToLower(strings.TrimSpace(item))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func parseCSVList(in string) []string {
	raw := strings.TrimSpace(in)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, item := range parts {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}
