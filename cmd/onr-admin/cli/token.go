package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/spf13/cobra"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Token helper",
	}
	cmd.AddCommand(newTokenCreateCmd())
	return cmd
}

func newTokenCreateCmd() *cobra.Command {
	opts := tokenCreateOptions{
		cfgPath: "onr.yaml",
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Generate Token Key (onr:v1?)",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := buildToken(opts)
			if err != nil {
				return err
			}
			fmt.Println("onr:v1?" + token)
			return nil
		},
	}
	addTokenCreateFlags(cmd, &opts)
	cmd.AddCommand(newTokenCreatePhaseCmd())
	return cmd
}

func newTokenCreatePhaseCmd() *cobra.Command {
	opts := tokenCreateOptions{
		cfgPath: "onr.yaml",
	}
	cmd := &cobra.Command{
		Use:   "phase <query|uri>",
		Short: "Print token output by phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			phase := strings.ToLower(strings.TrimSpace(args[0]))
			token, err := buildToken(opts)
			if err != nil {
				return err
			}
			switch phase {
			case "query":
				fmt.Println(token)
				return nil
			case "uri":
				fmt.Println("onr:v1?" + token)
				return nil
			default:
				return fmt.Errorf("unknown phase %q (expect: query|uri)", phase)
			}
		},
	}
	addTokenCreateFlags(cmd, &opts)
	return cmd
}

type tokenCreateOptions struct {
	cfgPath       string
	keysPath      string
	accessKey     string
	accessKeyName string
	provider      string
	modelOverride string
	upstreamKey   string
	plainKey      bool
}

func addTokenCreateFlags(cmd *cobra.Command, opts *tokenCreateOptions) {
	fs := cmd.Flags()
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&opts.accessKey, "access-key", "", "direct access key value")
	fs.StringVar(&opts.accessKeyName, "access-key-name", "", "read access key value by name from keys.yaml")
	fs.BoolVar(&opts.plainKey, "k-plain", false, "embed access key as k=<plain> (default k64)")
	fs.StringVarP(&opts.provider, "provider", "p", "", "provider")
	fs.StringVarP(&opts.modelOverride, "model", "m", "", "model override")
	fs.StringVar(&opts.upstreamKey, "upstream-key", "", "BYOK upstream key")
	// Backward compatibility for legacy flag name.
	fs.StringVar(&opts.upstreamKey, "uk", "", "BYOK upstream key")
}

func buildToken(opts tokenCreateOptions) (string, error) {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(opts.cfgPath))
	keysPath, _ := store.ResolveDataPaths(cfg, opts.keysPath, "")

	accessKey := strings.TrimSpace(opts.accessKey)
	if accessKey == "" {
		if strings.TrimSpace(opts.accessKeyName) != "" {
			v, err := accessKeyByName(keysPath, opts.accessKeyName)
			if err != nil {
				return "", err
			}
			accessKey = v
		} else {
			accessKey = store.ResolveMasterKey(cfg)
		}
	}
	accessKey = strings.TrimSpace(accessKey)
	if accessKey == "" {
		return "", errors.New("missing access key: provide --access-key or --access-key-name")
	}

	vals := tokenQueryValues(
		accessKey,
		strings.ToLower(strings.TrimSpace(opts.provider)),
		strings.TrimSpace(opts.modelOverride),
		strings.TrimSpace(opts.upstreamKey),
		opts.plainKey,
	)
	return vals, nil
}

func tokenQueryValues(accessKey, provider, modelOverride, upstreamKey string, plain bool) string {
	pairs := make([]string, 0, 4)
	if plain {
		pairs = append(pairs, "k="+urlEscape(accessKey))
	} else {
		k64 := base64.RawURLEncoding.EncodeToString([]byte(accessKey))
		pairs = append(pairs, "k64="+urlEscape(k64))
	}
	if provider != "" {
		pairs = append(pairs, "p="+urlEscape(provider))
	}
	if modelOverride != "" {
		pairs = append(pairs, "m="+urlEscape(modelOverride))
	}
	if upstreamKey != "" {
		pairs = append(pairs, "uk="+urlEscape(upstreamKey))
	}
	return strings.Join(pairs, "&")
}

func urlEscape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		const hex = "0123456789ABCDEF"
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}

func accessKeyByName(keysPath, name string) (string, error) {
	ks, err := keystore.Load(strings.TrimSpace(keysPath))
	if err != nil {
		return "", fmt.Errorf("load keys for --access-key-name failed: %w", err)
	}
	want := strings.TrimSpace(name)
	for _, ak := range ks.AccessKeys() {
		if strings.TrimSpace(ak.Name) == want {
			v := strings.TrimSpace(ak.Value)
			if v != "" {
				return v, nil
			}
			break
		}
	}
	return "", fmt.Errorf("access key name %q not found", want)
}
