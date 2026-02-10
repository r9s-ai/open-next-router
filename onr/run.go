package onr

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/r9s-ai/open-next-router/internal/version"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/config"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/models"
	"github.com/r9s-ai/open-next-router/onr/internal/onrserver"
	"gopkg.in/yaml.v3"
)

func Main(args []string) int {
	var cfgPath string
	var signalCmd string
	var testConfig bool
	var showVersion bool

	fs := flag.NewFlagSet("onr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "path to config yaml")
	fs.StringVar(&cfgPath, "c", "onr.yaml", "path to config yaml (alias of --config)")
	fs.StringVar(&signalCmd, "s", "", "send signal to a running onr (supported: reload)")
	fs.BoolVar(&testConfig, "t", false, "test config and exit (no network)")
	fs.BoolVar(&showVersion, "V", false, "show version information")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if showVersion {
		fmt.Fprintln(os.Stdout, version.Get())
		return 0
	}

	if strings.TrimSpace(signalCmd) != "" {
		switch strings.ToLower(strings.TrimSpace(signalCmd)) {
		case "reload":
			if err := sendReloadSignal(cfgPath); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err.Error())
				return 1
			}
			return 0
		default:
			_, _ = fmt.Fprintln(os.Stderr, "unsupported -s value: "+strings.TrimSpace(signalCmd)+" (supported: reload)")
			return 2
		}
	}

	if testConfig {
		if fs.NArg() == 1 && strings.TrimSpace(fs.Arg(0)) != "" {
			cfgPath = strings.TrimSpace(fs.Arg(0))
		}
		if err := runConfigTest(cfgPath); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error: "+err.Error())
			return 1
		}
		fmt.Fprintln(os.Stdout, "configuration ok")
		return 0
	}

	if err := onrserver.Run(cfgPath); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func runConfigTest(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	fmt.Fprintln(os.Stdout, "ok: config")

	res, err := dslconfig.ValidateProvidersDir(cfg.Providers.Dir)
	if err != nil {
		return fmt.Errorf("providers: %w", err)
	}
	fmt.Fprintf(os.Stdout, "ok: providers loaded=%d\n", len(res.LoadedProviders))

	if _, err := keystore.Load(cfg.Keys.File); err != nil {
		return fmt.Errorf("keys: %w", err)
	}
	fmt.Fprintln(os.Stdout, "ok: keys")

	if _, err := models.Load(cfg.Models.File); err != nil {
		return fmt.Errorf("models: %w", err)
	}
	fmt.Fprintln(os.Stdout, "ok: models")
	return nil
}

func sendReloadSignal(cfgPath string) error {
	pidFile, err := pidFileFromConfig(cfgPath)
	if err != nil {
		return err
	}
	// #nosec G304 -- pid file path comes from trusted config/env.
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("read pid file %q: %w", pidFile, err)
	}
	pidStr := strings.TrimSpace(string(b))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return fmt.Errorf("invalid pid in %q: %q", pidFile, pidStr)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process pid=%d: %w", pid, err)
	}
	if err := p.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP pid=%d: %w", pid, err)
	}
	return nil
}

func pidFileFromConfig(cfgPath string) (string, error) {
	// Default must match onr-core/pkg/config defaults.
	const def = "/var/run/onr.pid"
	path := strings.TrimSpace(cfgPath)
	if path == "" {
		return def, nil
	}
	// #nosec G304 -- config path comes from trusted flag.
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read config %q: %w", path, err)
	}
	var partial struct {
		Server struct {
			PidFile string `yaml:"pid_file"`
		} `yaml:"server"`
	}
	if err := yaml.Unmarshal(b, &partial); err != nil {
		return "", fmt.Errorf("parse config %q: %w", path, err)
	}
	if v := strings.TrimSpace(partial.Server.PidFile); v != "" {
		return v, nil
	}
	return def, nil
}
