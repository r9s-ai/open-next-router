package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/modelsquery"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/oauthclient"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Query upstream models via providers DSL",
	}
	cmd.AddCommand(newModelsGetCmd())
	return cmd
}

type modelsGetOptions struct {
	cfgPath      string
	keysPath     string
	providersDir string

	provider     string
	providersCSV string
	allProviders bool
	failFast     bool

	api         string
	stream      bool
	upstreamKey string
	baseURLOv   string
	debug       bool
}

func newModelsGetCmd() *cobra.Command {
	opts := modelsGetOptions{
		cfgPath: "onr.yaml",
		api:     "chat.completions",
	}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Fetch model ids for one or more providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsGetWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&opts.providersDir, "providers-dir", "", "providers dir path")
	fs.StringVarP(&opts.provider, "provider", "p", "", "provider name")
	fs.StringVar(&opts.providersCSV, "providers", "", "providers list for batch mode, comma separated")
	fs.BoolVar(&opts.allProviders, "all", false, "query all providers in DSL registry")
	fs.BoolVar(&opts.failFast, "fail-fast", false, "stop on first error in batch mode")
	fs.StringVar(&opts.api, "api", "chat.completions", "api name for DSL auth/match selection")
	fs.BoolVar(&opts.stream, "stream", false, "stream flag for DSL auth/match selection")
	fs.StringVar(&opts.upstreamKey, "upstream-key", "", "upstream api key")
	fs.StringVar(&opts.upstreamKey, "uk", "", "upstream api key")
	fs.StringVar(&opts.baseURLOv, "base-url", "", "override base url")
	fs.BoolVar(&opts.debug, "debug", false, "print upstream raw response body")
	return cmd
}

func runModelsGetWithOptions(opts modelsGetOptions) error {
	if err := validateModelsGetFlags(opts); err != nil {
		return err
	}
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(opts.cfgPath))
	keysPath, _ := store.ResolveDataPaths(cfg, opts.keysPath, "")
	providersDir := strings.TrimSpace(opts.providersDir)
	if providersDir == "" {
		if cfg != nil && strings.TrimSpace(cfg.Providers.Dir) != "" {
			providersDir = strings.TrimSpace(cfg.Providers.Dir)
		} else {
			providersDir = defaultProvidersDir
		}
	}

	reg := dslconfig.NewRegistry()
	if _, err := reg.ReloadFromDir(providersDir); err != nil {
		return fmt.Errorf("load providers dir %s failed: %w", providersDir, err)
	}
	targets, err := resolveTargetProviders(reg, opts.provider, opts.providersCSV, opts.allProviders)
	if err != nil {
		return err
	}

	var ks *keystore.Store
	needKeyLookup := strings.TrimSpace(opts.upstreamKey) == "" || strings.TrimSpace(opts.baseURLOv) == ""
	if needKeyLookup {
		loaded, loadErr := keystore.Load(strings.TrimSpace(keysPath))
		if loadErr != nil {
			if strings.TrimSpace(opts.upstreamKey) == "" {
				return fmt.Errorf("load keys.yaml failed: %w", loadErr)
			}
		} else {
			ks = loaded
		}
	}

	var debugOut io.Writer
	if opts.debug {
		debugOut = os.Stdout
	}
	oauth := oauthclient.New(nil, false, "")

	success := 0
	fail := 0
	for _, p := range targets {
		pf, ok := reg.GetProvider(p)
		if !ok {
			fail++
			fmt.Printf("provider=%s error=%q\n", p, "provider not found in registry")
			if opts.failFast {
				return fmt.Errorf("provider %q not found in %s", p, providersDir)
			}
			continue
		}

		key, baseURL := resolveProviderKeyAndBaseURL(ks, p, strings.TrimSpace(opts.upstreamKey), strings.TrimSpace(opts.baseURLOv))
		meta := dslmeta.Meta{
			API:      strings.TrimSpace(opts.api),
			IsStream: opts.stream,
			APIKey:   key,
		}

		if err := prepareOAuthForModels(oauth, p, pf, &meta); err != nil {
			fail++
			fmt.Printf("provider=%s error=%q\n", p, err.Error())
			if opts.failFast {
				return err
			}
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, qerr := modelsquery.Query(ctx, modelsquery.Params{
			Provider: p,
			File:     pf,
			Meta:     &meta,
			BaseURL:  baseURL,
			APIKey:   key,
			DebugOut: debugOut,
		})
		cancel()
		if qerr != nil {
			fail++
			fmt.Printf("provider=%s error=%q\n", p, qerr.Error())
			if opts.failFast {
				return qerr
			}
			continue
		}

		success++
		printModelIDs(result, len(targets) > 1)
	}
	if len(targets) > 1 {
		fmt.Printf("summary total=%d success=%d failed=%d\n", len(targets), success, fail)
	}
	if fail > 0 {
		return fmt.Errorf("batch query completed with %d failure(s)", fail)
	}
	return nil
}

func resolveProviderKeyAndBaseURL(ks *keystore.Store, provider, keyIn, baseURLIn string) (string, string) {
	key := strings.TrimSpace(keyIn)
	baseURL := strings.TrimSpace(baseURLIn)
	if ks == nil {
		return key, baseURL
	}
	if key != "" && baseURL != "" {
		return key, baseURL
	}
	next, found := ks.NextKey(provider)
	if !found {
		return key, baseURL
	}
	if key == "" {
		key = strings.TrimSpace(next.Value)
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(next.BaseURLOverride)
	}
	return key, baseURL
}

func prepareOAuthForModels(client *oauthclient.Client, provider string, pf dslconfig.ProviderFile, meta *dslmeta.Meta) error {
	if client == nil || meta == nil {
		return nil
	}
	phase, ok := pf.Headers.Effective(meta)
	if !ok {
		return nil
	}
	resolved, ok := phase.OAuth.Resolve(meta)
	if !ok {
		return nil
	}
	cacheKey := buildOAuthCacheKey(provider, resolved.CacheIdentity(), meta.APIKey)
	tok, err := client.GetToken(context.Background(), oauthclient.AcquireInput{
		CacheKey:          cacheKey,
		TokenURL:          resolved.TokenURL,
		Method:            resolved.Method,
		ContentType:       resolved.ContentType,
		Form:              resolved.Form,
		BasicAuthUsername: resolved.BasicAuthUsername,
		BasicAuthPassword: resolved.BasicAuthPassword,
		TokenPath:         resolved.TokenPath,
		ExpiresInPath:     resolved.ExpiresInPath,
		TokenTypePath:     resolved.TokenTypePath,
		Timeout:           time.Duration(resolved.TimeoutMs) * time.Millisecond,
		RefreshSkew:       time.Duration(resolved.RefreshSkewSec) * time.Second,
		FallbackTTL:       time.Duration(resolved.FallbackTTLSec) * time.Second,
	})
	if err != nil {
		return err
	}
	meta.OAuthAccessToken = strings.TrimSpace(tok.AccessToken)
	return nil
}

func buildOAuthCacheKey(provider, identity, apiKey string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	id := strings.TrimSpace(identity)
	hash := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return p + "|" + id + "|" + hex.EncodeToString(hash[:])
}

func printModelIDs(result modelsquery.Result, withProviderPrefix bool) {
	ids := append([]string(nil), result.IDs...)
	sort.Strings(ids)
	if withProviderPrefix {
		for _, id := range ids {
			fmt.Printf("provider=%s id=%s\n", result.Provider, id)
		}
		if len(ids) == 0 {
			fmt.Printf("provider=%s id=<none>\n", result.Provider)
		}
		return
	}
	for _, id := range ids {
		fmt.Println(id)
	}
}

func validateModelsGetFlags(opts modelsGetOptions) error {
	count := 0
	if strings.TrimSpace(opts.provider) != "" {
		count++
	}
	if strings.TrimSpace(opts.providersCSV) != "" {
		count++
	}
	if opts.allProviders {
		count++
	}
	if count == 0 {
		return errors.New("missing provider: use --provider/-p, --providers or --all")
	}
	if count > 1 {
		return errors.New("provider flags are mutually exclusive: use only one of --provider/-p, --providers, --all")
	}
	return nil
}
