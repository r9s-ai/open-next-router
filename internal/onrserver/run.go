package onrserver

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/r9s-ai/open-next-router/internal/config"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/internal/proxy"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
)

func Run(cfgPath string) error {
	startedAt := time.Now().Unix()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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
		HTTP:            httpClient,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		Registry:        reg,
		UsageEst:        &cfg.UsageEstimation,
		ProxyByProvider: cfg.UpstreamProxies.ByProvider,
	}

	st := &state{
		keys:        keys,
		modelRouter: mr,
	}
	st.SetStartedAtUnix(startedAt)

	engine := NewRouter(cfg, st, reg, pclient)

	log.Printf("open-next-router listening on %s", cfg.Server.Listen)
	if err := engine.Run(cfg.Server.Listen); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}
