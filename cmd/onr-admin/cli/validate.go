package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
)

func runValidate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: onr-admin validate <all|keys|models|providers> [flags]")
	}
	target := strings.ToLower(strings.TrimSpace(args[0]))

	var cfgPath string
	var keysPath string
	var modelsPath string
	var providersDir string
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&modelsPath, "models", "", "models.yaml path")
	fs.StringVar(&providersDir, "providers-dir", "", "providers dir path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, modelsPath = store.ResolveDataPaths(cfg, keysPath, modelsPath)
	if strings.TrimSpace(providersDir) == "" {
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
