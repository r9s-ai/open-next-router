package onrserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/models"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

type providersReloadResult struct {
	LoadResult       dslconfig.LoadResult
	ChangedProviders []string
}

func Run(cfgPath string) error {
	startedAt := time.Now().Unix()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	sysLogger, err := logx.NewSystemLogger(cfg.Logging.Level)
	if err != nil {
		return fmt.Errorf("init system log: %w", err)
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
	loadRes, err := reg.ReloadFromDir(cfg.Providers.Dir)
	if err != nil {
		return fmt.Errorf("load providers dir %q: %w", cfg.Providers.Dir, err)
	}
	logSkippedProviders(sysLogger, cfg.Providers.Dir, loadRes.SkippedFiles, loadRes.SkippedReasons, false)

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
		SystemLogger:             sysLogger,
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

	reloadMu := &sync.Mutex{}
	installReloadSignalHandler(cfg, st, reg, pclient, reloadMu, sysLogger)
	autoReloadClose, err := installProvidersAutoReload(cfg, reg, reloadMu, sysLogger)
	if err != nil {
		return fmt.Errorf("init providers auto reload: %w", err)
	}
	if autoReloadClose != nil {
		defer func() { _ = autoReloadClose.Close() }()
	}

	accessFormat, err := logx.ResolveAccessLogFormat(cfg.Logging.AccessLogFormat, cfg.Logging.AccessLogFormatPreset)
	if err != nil {
		return fmt.Errorf("resolve access log format: %w", err)
	}
	accessFormatter, err := logx.CompileAccessLogFormat(accessFormat)
	if err != nil {
		return fmt.Errorf("compile access_log_format: %w", err)
	}
	engine := NewRouter(cfg, st, reg, pclient, accessLogger, accessColor, "X-Onr-Request-Id", accessFormatter)

	logStartupSummary(sysLogger, cfg, cfgPath)
	sysLogger.Info(logx.SystemCategoryServer, "open-next-router listening", map[string]any{
		"listen_url": resolveListenURL(cfg.Server.Listen),
	})
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
	if cfg.Logging.AccessLogRotate.Enabled && path == "" {
		return nil, nil, false, errors.New("logging.access_log_path is required when logging.access_log_rotate.enabled=true")
	}
	if path == "" {
		// default: stdout (same as current behavior)
		return log.New(os.Stdout, "", 0), nil, true, nil
	}
	if cfg.Logging.AccessLogRotate.Enabled {
		w, err := logx.NewAccessRotateWriter(logx.AccessLogRotateOptions{
			Path:       path,
			MaxSizeMB:  cfg.Logging.AccessLogRotate.MaxSizeMB,
			MaxBackups: cfg.Logging.AccessLogRotate.MaxBackups,
			MaxAgeDays: cfg.Logging.AccessLogRotate.MaxAgeDays,
			Compress:   cfg.Logging.AccessLogRotate.Compress,
		})
		if err != nil {
			return nil, nil, false, err
		}
		return log.New(w, "", 0), w, false, nil
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
	return log.New(f, "", 0), f, false, nil
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

func installReloadSignalHandler(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client, mu *sync.Mutex, logger *logx.SystemLogger) {
	if cfg == nil || st == nil || reg == nil || mu == nil {
		return
	}
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			mu.Lock()
			providersRes, err := reloadRuntime(cfg, st, reg, pclient, logger)
			mu.Unlock()
			if err != nil {
				logReloadFailed(logger, "signal", err)
				continue
			}
			logReloadOK(logger, "signal", cfg, providersRes)
		}
	}()
}

func reloadProvidersRuntime(cfg *config.Config, reg *dslconfig.Registry, logger *logx.SystemLogger) (providersReloadResult, error) {
	if cfg == nil || reg == nil {
		return providersReloadResult{}, errors.New("reload providers: nil cfg/registry")
	}
	before := snapshotProviderFingerprints(reg)
	loadRes, err := reg.ReloadFromDir(cfg.Providers.Dir)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload providers dir %q: %w", cfg.Providers.Dir, err)
	}
	logSkippedProviders(logger, cfg.Providers.Dir, loadRes.SkippedFiles, loadRes.SkippedReasons, true)
	after := snapshotProviderFingerprints(reg)
	return providersReloadResult{
		LoadResult:       loadRes,
		ChangedProviders: diffChangedProviderNames(before, after),
	}, nil
}

func reloadRuntime(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client, logger *logx.SystemLogger) (providersReloadResult, error) {
	if cfg == nil || st == nil || reg == nil || pclient == nil {
		return providersReloadResult{}, errors.New("reload: nil cfg/state/registry/pclient")
	}
	providersRes, err := reloadProvidersRuntime(cfg, reg, logger)
	if err != nil {
		return providersReloadResult{}, err
	}
	ks, err := keystore.Load(cfg.Keys.File)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload keys file %q: %w", cfg.Keys.File, err)
	}
	mr, err := models.Load(cfg.Models.File)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload models file %q: %w", cfg.Models.File, err)
	}
	pricingResolver, err := pricing.LoadResolver(cfg.Pricing.File, cfg.Pricing.OverridesFile)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload pricing files failed: %w", err)
	}
	st.SetKeys(ks)
	st.SetModelRouter(mr)
	pclient.SetPricingResolver(pricingResolver)
	pclient.SetPricingEnabled(cfg.Pricing.Enabled)
	return providersRes, nil
}

func logSkippedProviders(logger *logx.SystemLogger, providersDir string, skipped []string, skippedReasons map[string]string, reloading bool) {
	if len(skipped) == 0 {
		return
	}
	phase := "load"
	if reloading {
		phase = "reload"
	}
	sortedSkipped := append([]string(nil), skipped...)
	sort.Strings(sortedSkipped)
	details := make([]string, 0, len(sortedSkipped))
	for _, file := range sortedSkipped {
		reason := strings.TrimSpace(skippedReasons[file])
		if reason == "" {
			reason = "unknown"
		}
		details = append(details, fmt.Sprintf("%s: %s", file, reason))
	}
	logger.Warn(logx.SystemCategoryProviders, "providers skipped invalid files", map[string]any{
		"phase":                 phase,
		"providers_dir":         providersDir,
		"skipped_invalid_files": strings.Join(sortedSkipped, ","),
		"skip_reasons":          strings.Join(details, " | "),
	})
}

func logStartupSummary(logger *logx.SystemLogger, cfg *config.Config, cfgPath string) {
	if cfg == nil {
		return
	}
	logger.Info(logx.SystemCategoryStartup, "startup config loaded", map[string]any{
		"config_path":   strings.TrimSpace(cfgPath),
		"providers_dir": cfg.Providers.Dir,
		"keys_file":     cfg.Keys.File,
		"models_file":   cfg.Models.File,
	})
	logger.Info(logx.SystemCategoryStartup, "startup runtime flags", map[string]any{
		"traffic_dump_enabled":              cfg.TrafficDump.Enabled,
		"traffic_dump_dir":                  cfg.TrafficDump.Dir,
		"traffic_dump_max_bytes":            cfg.TrafficDump.MaxBytes,
		"access_log_enabled":                cfg.Logging.AccessLog,
		"access_log_target":                 accessLogTarget(cfg),
		"providers_auto_reload_enabled":     cfg.Providers.AutoReload.Enabled,
		"providers_auto_reload_debounce_ms": cfg.Providers.AutoReload.DebounceMs,
	})
}

func logReloadFailed(logger *logx.SystemLogger, source string, err error) {
	if err == nil {
		return
	}
	logger.Error(logx.SystemCategoryReload, "reload failed", map[string]any{
		"source": source,
		"error":  err.Error(),
	})
}

func logReloadOK(logger *logx.SystemLogger, source string, cfg *config.Config, providersRes providersReloadResult) {
	if cfg == nil {
		return
	}
	logger.Info(logx.SystemCategoryReload, "reload ok", map[string]any{
		"source":                 source,
		"providers_dir":          cfg.Providers.Dir,
		"changed_providers":      providerNamesForLog(providersRes.ChangedProviders),
		"keys_file":              cfg.Keys.File,
		"models_file":            cfg.Models.File,
		"pricing_file":           cfg.Pricing.File,
		"pricing_overrides_file": cfg.Pricing.OverridesFile,
	})
}

func accessLogTarget(cfg *config.Config) string {
	if cfg == nil || !cfg.Logging.AccessLog {
		return "disabled"
	}
	if strings.TrimSpace(cfg.Logging.AccessLogPath) == "" {
		return "stdout"
	}
	return strings.TrimSpace(cfg.Logging.AccessLogPath)
}

func resolveListenURL(listen string) string {
	v := strings.TrimSpace(listen)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v
	}
	if strings.HasPrefix(v, ":") {
		return "http://0.0.0.0" + v
	}
	host, port, err := net.SplitHostPort(v)
	if err == nil {
		h := strings.TrimSpace(host)
		switch h {
		case "", "0.0.0.0", "::", "[::]":
			h = "0.0.0.0"
		}
		if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") && !strings.HasSuffix(h, "]") {
			h = "[" + h + "]"
		}
		return "http://" + h + ":" + strings.TrimSpace(port)
	}
	return "http://" + v
}

func providerNamesForLog(names []string) string {
	if len(names) == 0 {
		return "<none>"
	}
	return strings.Join(names, ",")
}

func snapshotProviderFingerprints(reg *dslconfig.Registry) map[string]string {
	if reg == nil {
		return map[string]string{}
	}
	names := reg.ListProviderNames()
	out := make(map[string]string, len(names))
	for _, name := range names {
		pf, ok := reg.GetProvider(name)
		if !ok {
			continue
		}
		out[name] = providerFingerprint(pf)
	}
	return out
}

func providerFingerprint(pf dslconfig.ProviderFile) string {
	return strings.TrimSpace(pf.Path) + "\x00" + pf.Content
}

func diffChangedProviderNames(before map[string]string, after map[string]string) []string {
	changed := make([]string, 0)
	for name, prev := range before {
		next, ok := after[name]
		if !ok || next != prev {
			changed = append(changed, name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}
