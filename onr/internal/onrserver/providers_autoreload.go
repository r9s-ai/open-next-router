package onrserver

import (
	"crypto/sha256"
	"fmt"
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
	if !needsSourceAutoReload(cfg) {
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

	if err := addJSWatch(watcher, cfg); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	triggerCh := make(chan struct{}, 1)

	initialJSSignature := jsTreeSignature(cfg.JS.Root)
	go func() {
		defer close(doneCh)
		var poll *time.Ticker
		var pollC <-chan time.Time
		signature := ""
		if cfg.JS.Reload == "poll" {
			poll = time.NewTicker(time.Second)
			pollC = poll.C
			defer poll.Stop()
			signature = initialJSSignature
		}
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
			case <-pollC:
				current := jsTreeSignature(cfg.JS.Root)
				if current != signature {
					signature = current
					runReload()
				}
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
				if shouldTriggerProviderReload(evt) && shouldWatchSource(cfg, evt.Name) {
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

func isWithinJSRoot(root, path string) bool {
	if root == "" {
		return false
	}
	a, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	b, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(a, b)
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}
func jsTreeSignature(root string) string {
	if root == "" {
		return ""
	}
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s:%d:%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func needsSourceAutoReload(cfg *config.Config) bool {
	return cfg.Providers.AutoReload.Enabled || cfg.JS.Reload == "watch" || cfg.JS.Reload == "poll"
}
func addJSWatch(watcher *fsnotify.Watcher, cfg *config.Config) error {
	if cfg.JS.Reload == "watch" && cfg.JS.Root != "" {
		return addWatchRecursive(watcher, cfg.JS.Root)
	}
	return nil
}
func shouldWatchSource(cfg *config.Config, path string) bool {
	within := isWithinJSRoot(cfg.JS.Root, path)
	return cfg.JS.Reload == "watch" && within || cfg.Providers.AutoReload.Enabled && (!within || filepath.Ext(path) == ".conf")
}
