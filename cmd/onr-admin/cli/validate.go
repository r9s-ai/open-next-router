package cli

import (
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "校验 keys/models/providers",
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
	cfgPath      string
	keysPath     string
	modelsPath   string
	providersDir string
}

func newValidateTargetCmd(target string) *cobra.Command {
	opts := validateOptions{cfgPath: "onr.yaml"}
	cmd := &cobra.Command{
		Use:   target,
		Short: "校验 " + target,
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
}

func runValidateTarget(target string, opts validateOptions) error {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(opts.cfgPath))
	keysPath, modelsPath := store.ResolveDataPaths(cfg, opts.keysPath, opts.modelsPath)
	providersDir := strings.TrimSpace(opts.providersDir)
	if providersDir == "" {
		if cfg != nil && strings.TrimSpace(cfg.Providers.Dir) != "" {
			providersDir = strings.TrimSpace(cfg.Providers.Dir)
		} else {
			providersDir = "./config/providers"
		}
	}

	switch target {
	case "keys":
		return validateKeys(keysPath)
	case "models":
		return validateModels(modelsPath)
	case "providers":
		return validateProviders(providersDir)
	case "all":
		if err := validateKeys(keysPath); err != nil {
			return err
		}
		if err := validateModels(modelsPath); err != nil {
			return err
		}
		if err := validateProviders(providersDir); err != nil {
			return err
		}
		fmt.Println("validate all: OK")
		return nil
	default:
		return fmt.Errorf("unknown validate target %q", target)
	}
}

func validateKeys(path string) error {
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
	fmt.Println("validate keys: OK")
	return nil
}

func validateModels(path string) error {
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
	fmt.Println("validate models: OK")
	return nil
}

func validateProviders(path string) error {
	if _, err := dslconfig.ValidateProvidersDir(path); err != nil {
		return fmt.Errorf("validate providers dir %s failed: %w", path, err)
	}
	fmt.Println("validate providers: OK")
	return nil
}
