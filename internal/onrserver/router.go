package onrserver

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/edgefn/open-next-router/internal/auth"
	"github.com/edgefn/open-next-router/internal/config"
	"github.com/edgefn/open-next-router/internal/keystore"
	"github.com/edgefn/open-next-router/internal/models"
	"github.com/edgefn/open-next-router/internal/proxy"
	"github.com/edgefn/open-next-router/internal/requestid"
	"github.com/edgefn/open-next-router/pkg/dslconfig"
	"github.com/edgefn/open-next-router/pkg/trafficdump"
)

func NewRouter(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client) *gin.Engine {
	r := gin.New()
	r.Use(requestIDMiddleware())
	r.Use(requestLogger())
	r.Use(gin.Recovery())
	if cfg.TrafficDump.Enabled {
		r.Use(trafficDumpMiddleware(cfg))
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	secured := r.Group("/")
	secured.Use(auth.Middleware(cfg.Auth.APIKey))

	secured.GET("/admin/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"providers": reg.ListProviderNames(),
		})
	})

	secured.POST("/admin/reload", func(c *gin.Context) {
		res, err := reg.ReloadFromDir(cfg.Providers.Dir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ks, err := keystore.Load(cfg.Keys.File)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "providers": res})
			return
		}
		st.SetKeys(ks)
		mr, err := models.Load(cfg.Models.File)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "providers": res})
			return
		}
		st.SetModelRouter(mr)
		log.Printf("reload ok")
		c.JSON(http.StatusOK, gin.H{"providers": res})
	})

	v1 := secured.Group("/v1")
	v1.POST("/chat/completions", makeHandler(cfg, st, pclient, "chat.completions"))
	v1.POST("/responses", makeHandler(cfg, st, pclient, "responses"))
	v1.POST("/embeddings", makeHandler(cfg, st, pclient, "embeddings"))
	v1.POST("/messages", makeHandler(cfg, st, pclient, "claude.messages"))
	v1.GET("/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, st.ModelRouter().ToOpenAIListAt(st.StartedAtUnix()))
	})

	v1beta := secured.Group("/v1beta")
	// Gemini-style model listing.
	v1beta.GET("/models", func(c *gin.Context) {
		type geminiModel struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		}
		ids := st.ModelRouter().Models()
		out := make([]geminiModel, 0, len(ids))
		for _, id := range ids {
			out = append(out, geminiModel{Name: id, DisplayName: id})
		}
		c.JSON(http.StatusOK, gin.H{
			"models":        out,
			"nextPageToken": nil,
		})
	})
	// Gemini native API paths: /v1beta/models/{model}:generateContent
	// (Stage 1) Only generateContent / streamGenerateContent are supported.
	v1beta.POST("/models/*path", makeGeminiHandler(cfg, st, pclient))

	return r
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestid.HeaderKey))
		if id == "" {
			id = requestid.Gen()
		}
		c.Header(requestid.HeaderKey, id)
		c.Set(requestid.HeaderKey, id)
		c.Next()
	}
}

func trafficDumpMiddleware(cfg *config.Config) gin.HandlerFunc {
	tdcfg := trafficdump.Config{
		Enabled:     cfg.TrafficDump.Enabled,
		Dir:         cfg.TrafficDump.Dir,
		FilePath:    cfg.TrafficDump.FilePath,
		MaxBytes:    cfg.TrafficDump.MaxBytes,
		MaskSecrets: cfg.TrafficDump.MaskSecrets,
	}
	return func(c *gin.Context) {
		rec, err := trafficdump.Start(c, tdcfg)
		if err != nil {
			c.Next()
			return
		}
		c.Next()
		rec.Close()
	}
}
