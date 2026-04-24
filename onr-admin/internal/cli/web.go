package cli

import (
	"strings"

	adminweb "github.com/r9s-ai/open-next-router/onr-admin/internal/web"
	"github.com/spf13/cobra"
)

type webOptions struct {
	cfgPath      string
	providersDir string
	listen       string
}

// newWebCmd returns a non-nil web command.
func newWebCmd() *cobra.Command {
	opts := webOptions{
		cfgPath: "onr.yaml",
	}
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start web UI for provider config validate/save",
		Long: strings.TrimSpace(`
Start web UI for provider config validate/save.

Environment variables:
  ONR_ADMIN_WEB_LISTEN
    HTTP listen address used when --listen is not set.

  ONR_ADMIN_WEB_CURL_API_BASE_URL
    Default API base URL prefilled in the web UI curl/test request area.
    Default: http://127.0.0.1:3300
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWebWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.cfgPath, "config", "c", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.providersDir, "providers-dir", "", "providers dir path")
	fs.StringVar(&opts.listen, "listen", "", "http listen address (overrides ONR_ADMIN_WEB_LISTEN)")
	return cmd
}

func runWebWithOptions(opts webOptions) error {
	return adminweb.Run(adminweb.Options{
		ConfigPath:   strings.TrimSpace(opts.cfgPath),
		ProvidersDir: strings.TrimSpace(opts.providersDir),
		Listen:       strings.TrimSpace(opts.listen),
	})
}
