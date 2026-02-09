package onrserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/r9s-ai/open-next-router/internal/config"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/internal/proxy"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
)

func Run(cfgPath string) error {
	startedAt := time.Now().Unix()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	accessLogger, accessClose, accessColor, err := openAccessLogger(cfg)
	if err != nil {
		return fmt.Errorf("init access log: %w", err)
	}
	if accessClose != nil {
		defer func() { _ = accessClose.Close() }()
	}

	pidCleanup, err := writePIDFile(cfg)
	if err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	if pidCleanup != nil {
		defer func() { _ = pidCleanup.Close() }()
	}

	reg := dslconfig.NewRegistry()
	if _, err := reg.ReloadFromDir(cfg.Providers.Dir); err != nil {
		return fmt.Errorf("load providers dir %q: %w", cfg.Providers.Dir, err)
	}

	keys, err := keystore.Load(cfg.Keys.File)
	if err != nil {
		return fmt.Errorf("load keys file %q: %w", cfg.Keys.File, err)
	}
	mr, err := models.Load(cfg.Models.File)
	if err != nil {
		return fmt.Errorf("load models file %q: %w", cfg.Models.File, err)
	}

	readTimeout := time.Duration(cfg.Server.ReadTimeoutMs) * time.Millisecond
	writeTimeout := time.Duration(cfg.Server.WriteTimeoutMs) * time.Millisecond

	httpClient := &http.Client{
		Timeout: writeTimeout,
	}

	pclient := &proxy.Client{
		HTTP:                     httpClient,
		ReadTimeout:              readTimeout,
		WriteTimeout:             writeTimeout,
		Registry:                 reg,
		UsageEst:                 &cfg.UsageEstimation,
		ProxyByProvider:          cfg.UpstreamProxies.ByProvider,
		OAuthTokenPersistEnabled: cfg.OAuth.TokenPersist.Enabled,
		OAuthTokenPersistDir:     cfg.OAuth.TokenPersist.Dir,
	}
	pricingResolver, err := pricing.LoadResolver(cfg.Pricing.File, cfg.Pricing.OverridesFile)
	if err != nil {
		return fmt.Errorf("load pricing files failed: %w", err)
	}
	pclient.SetPricingResolver(pricingResolver)
	pclient.SetPricingEnabled(cfg.Pricing.Enabled)

	st := &state{
		keys:        keys,
		modelRouter: mr,
	}
	st.SetStartedAtUnix(startedAt)

	installReloadSignalHandler(cfg, st, reg, pclient)

	engine := NewRouter(cfg, st, reg, pclient, accessLogger, accessColor)

	log.Printf("open-next-router listening on %s", cfg.Server.Listen)
	if err := engine.Run(cfg.Server.Listen); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func openAccessLogger(cfg *config.Config) (*log.Logger, io.Closer, bool, error) {
	if cfg == nil || !cfg.Logging.AccessLog {
		return nil, nil, false, nil
	}

	path := strings.TrimSpace(cfg.Logging.AccessLogPath)
	if path == "" {
		// default: stdout (same as current behavior)
		return log.New(os.Stdout, "", log.LstdFlags), nil, true, nil
	}

	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, nil, false, err
		}
	}
	// #nosec G304 -- access_log_path comes from trusted config/env.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, false, err
	}
	return log.New(f, "", log.LstdFlags), f, false, nil
}

type closerFunc func() error

func (c closerFunc) Close() error { return c() }

func writePIDFile(cfg *config.Config) (io.Closer, error) {
	if cfg == nil {
		return nil, nil
	}
	path := strings.TrimSpace(cfg.Server.PidFile)
	if path == "" {
		return nil, nil
	}
	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}

	tmp := path + ".tmp"
	pid := strconv.Itoa(os.Getpid()) + "\n"
	// #nosec G304 -- pid_file comes from trusted config/env.
	if err := os.WriteFile(tmp, []byte(pid), 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	return closerFunc(func() error { return os.Remove(path) }), nil
}

func installReloadSignalHandler(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client) {
	if cfg == nil || st == nil || reg == nil {
		return
	}
	var mu sync.Mutex
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			mu.Lock()
			err := reloadRuntime(cfg, st, reg, pclient)
			mu.Unlock()
			if err != nil {
				log.Printf("reload failed: %v", err)
				continue
			}
			log.Printf("reload ok")
		}
	}()
}

func reloadRuntime(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client) error {
	if cfg == nil || st == nil || reg == nil || pclient == nil {
		return errors.New("reload: nil cfg/state/registry/pclient")
	}
	if _, err := reg.ReloadFromDir(cfg.Providers.Dir); err != nil {
		return fmt.Errorf("reload providers dir %q: %w", cfg.Providers.Dir, err)
	}
	ks, err := keystore.Load(cfg.Keys.File)
	if err != nil {
		return fmt.Errorf("reload keys file %q: %w", cfg.Keys.File, err)
	}
	mr, err := models.Load(cfg.Models.File)
	if err != nil {
		return fmt.Errorf("reload models file %q: %w", cfg.Models.File, err)
	}
	pricingResolver, err := pricing.LoadResolver(cfg.Pricing.File, cfg.Pricing.OverridesFile)
	if err != nil {
		return fmt.Errorf("reload pricing files failed: %w", err)
	}
	st.SetKeys(ks)
	st.SetModelRouter(mr)
	pclient.SetPricingResolver(pricingResolver)
	pclient.SetPricingEnabled(cfg.Pricing.Enabled)
	return nil
}
