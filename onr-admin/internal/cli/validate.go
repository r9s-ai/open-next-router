package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/models"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate keys/models/providers",
	}
	cmd.AddCommand(
		newValidateTargetCmd("all"),
		newValidateTargetCmd("keys"),
		newValidateTargetCmd("models"),
		newValidateTargetCmd("providers"),
	)
	return cmd
}

type validateOptions struct {
	cfgPath       string
	keysPath      string
	modelsPath    string
	providersDir  string
	showUsagePlan bool
	stdout        io.Writer
}

func newValidateTargetCmd(target string) *cobra.Command {
	opts := validateOptions{cfgPath: "onr.yaml", stdout: os.Stdout}
	cmd := &cobra.Command{
		Use:   target,
		Short: "Validate " + target,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidateTarget(target, opts)
		},
	}
	addValidateFlags(cmd, &opts)
	return cmd
}

func addValidateFlags(cmd *cobra.Command, opts *validateOptions) {
	fs := cmd.Flags()
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&opts.modelsPath, "models", "", "models.yaml path")
	fs.StringVar(&opts.providersDir, "providers-dir", "", "providers dir path")
	fs.BoolVar(&opts.showUsagePlan, "show-usage-plan", false, "print compiled usage execution plans after provider validation")
}

func runValidateTarget(target string, opts validateOptions) error {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(opts.cfgPath))
	keysPath, modelsPath := store.ResolveDataPaths(cfg, opts.keysPath, opts.modelsPath)
	providersDir := resolveProviderSourcePath(cfg, opts.providersDir)

	switch target {
	case "keys":
		return validateKeys(keysPath, opts.stdout)
	case "models":
		return validateModels(modelsPath, opts.stdout)
	case "providers":
		return validateProviders(providersDir, opts.showUsagePlan, opts.stdout)
	case "all":
		if err := validateKeys(keysPath, opts.stdout); err != nil {
			return err
		}
		if err := validateModels(modelsPath, opts.stdout); err != nil {
			return err
		}
		if err := validateProviders(providersDir, opts.showUsagePlan, opts.stdout); err != nil {
			return err
		}
		fmt.Fprintln(opts.stdout, "validate all: OK")
		return nil
	default:
		return fmt.Errorf("unknown validate target %q", target)
	}
}

func validateKeys(path string, stdout io.Writer) error {
	doc, err := store.LoadOrInitKeysDoc(path)
	if err != nil {
		return fmt.Errorf("load keys yaml: %w", err)
	}
	if err := store.ValidateKeysDoc(doc); err != nil {
		return fmt.Errorf("keys yaml structure invalid: %w", err)
	}
	if _, err := keystore.Load(path); err != nil {
		return fmt.Errorf("keystore load failed: %w", err)
	}
	fmt.Fprintln(stdout, "validate keys: OK")
	return nil
}

func validateModels(path string, stdout io.Writer) error {
	doc, err := store.LoadOrInitModelsDoc(path)
	if err != nil {
		return fmt.Errorf("load models yaml: %w", err)
	}
	if err := store.ValidateModelsDoc(doc); err != nil {
		return fmt.Errorf("models yaml structure invalid: %w", err)
	}
	if _, err := models.Load(path); err != nil {
		return fmt.Errorf("models load failed: %w", err)
	}
	fmt.Fprintln(stdout, "validate models: OK")
	return nil
}

func validateProviders(path string, showUsagePlan bool, stdout io.Writer) error {
	res, err := dslconfig.ValidateProvidersPath(path)
	if err != nil {
		return fmt.Errorf("validate providers dir %s failed: %w", path, err)
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stdout, "warn: %s\n", w.String())
	}
	fmt.Fprintln(stdout, "validate providers: OK")
	if showUsagePlan {
		providers, err := loadValidatedProviderFiles(path)
		if err != nil {
			return err
		}
		payload, err := buildUsagePlanJSON(providers)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(payload))
	}
	return nil
}

func loadValidatedProviderFiles(dir string) ([]dslconfig.ProviderFile, error) {
	reg, res, err := loadRegistryFromProviderSource(dir)
	if err != nil {
		return nil, fmt.Errorf("load providers %s failed: %w", dir, err)
	}
	out := make([]dslconfig.ProviderFile, 0, len(res.LoadedProviders))
	for _, name := range reg.ListProviderNames() {
		pf, ok := reg.GetProvider(name)
		if !ok {
			continue
		}
		out = append(out, pf)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func buildUsagePlanJSON(providers []dslconfig.ProviderFile) ([]byte, error) {
	type providerUsagePlanOutput struct {
		Provider string                               `json:"provider"`
		Path     string                               `json:"path"`
		Usage    dslconfig.ProviderUsageExecutionPlan `json:"usage"`
	}
	items := make([]providerUsagePlanOutput, 0, len(providers))
	for _, pf := range providers {
		items = append(items, providerUsagePlanOutput{
			Provider: pf.Name,
			Path:     pf.Path,
			Usage:    pf.Usage.CompiledPlans(),
		})
	}
	return json.MarshalIndent(items, "", "  ")
}
