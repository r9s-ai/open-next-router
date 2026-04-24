package onrserver

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/pkg/config"
)

// installProvidersAutoReload requires non-nil config, registry, and mutex from Run.
func installProvidersAutoReload(cfg *config.Config, reg *dslconfig.Registry, mu *sync.Mutex, logger *logx.SystemLogger) (io.Closer, error) {
	if !cfg.Providers.AutoReload.Enabled {
		return nil, nil
	}

	dir := strings.TrimSpace(config.ResolveProviderDSLWatchDir(cfg))
	if dir == "" {
		return nil, nil
	}
	debounce := time.Duration(cfg.Providers.AutoReload.DebounceMs) * time.Millisecond

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addWatchRecursive(watcher, dir); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	triggerCh := make(chan struct{}, 1)

	go func() {
		defer close(doneCh)
		var (
			timer  *time.Timer
			timerC <-chan time.Time
		)
		resetTimer := func() {
			if timer == nil {
				timer = time.NewTimer(debounce)
				timerC = timer.C
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
			timerC = timer.C
		}
		runReload := func() {
			mu.Lock()
			reloadRes, err := reloadProvidersRuntime(cfg, reg, logger)
			mu.Unlock()
			if err != nil {
				logReloadFailed(logger, "providers_auto", err)
				return
			}
			logReloadOK(logger, "providers_auto", cfg, reloadRes)
		}

		for {
			select {
			case <-stopCh:
				if timer != nil {
					timer.Stop()
				}
				return
			case <-timerC:
				timerC = nil
				runReload()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error(logx.SystemCategoryProviders, "providers auto-reload watcher error", map[string]any{
					"source": "providers_auto",
					"error":  err.Error(),
				})
			case evt, ok := <-watcher.Events:
				if !ok {
					return
				}
				if evt.Op&fsnotify.Create != 0 {
					if fi, statErr := os.Stat(evt.Name); statErr == nil && fi.IsDir() {
						if addErr := addWatchRecursive(watcher, evt.Name); addErr != nil {
							logger.Warn(logx.SystemCategoryProviders, "providers auto-reload add watch failed", map[string]any{
								"source": "providers_auto",
								"path":   evt.Name,
								"error":  addErr.Error(),
							})
						}
					}
				}
				if shouldTriggerProviderReload(evt) {
					select {
					case triggerCh <- struct{}{}:
					default:
					}
				}
			case <-triggerCh:
				resetTimer()
			}
		}
	}()

	logger.Info(logx.SystemCategoryProviders, "providers auto-reload enabled", map[string]any{
		"source":              "providers_auto",
		"providers_watch_dir": dir,
		"debounce_ms":         cfg.Providers.AutoReload.DebounceMs,
	})
	return closerFunc(func() error {
		close(stopCh)
		_ = watcher.Close()
		<-doneCh
		return nil
	}), nil
}

func shouldTriggerProviderReload(evt fsnotify.Event) bool {
	if strings.TrimSpace(evt.Name) == "" {
		return false
	}
	if evt.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
		return false
	}
	base := filepath.Base(evt.Name)
	return !strings.HasPrefix(base, ".")
}

func addWatchRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return watcher.Add(path)
	})
}
