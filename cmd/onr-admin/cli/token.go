package cli

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
)

func runToken(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: onr-admin token create [flags]")
	}
	switch args[0] {
	case "create":
		return runTokenCreate(args[1:])
	default:
		return fmt.Errorf("unknown token subcommand %q", args[0])
	}
}

func runTokenCreate(args []string) error {
	if len(args) > 0 && args[0] == "phase" {
		return runTokenCreatePhase(args[1:])
	}
	token, err := buildTokenFromFlags(args)
	if err != nil {
		return err
	}
	fmt.Println("onr:v1?" + token)
	return nil
}

func runTokenCreatePhase(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: onr-admin token create phase <query|uri> [flags]")
	}
	phase := strings.ToLower(strings.TrimSpace(args[0]))
	token, err := buildTokenFromFlags(args[1:])
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
}

func buildTokenFromFlags(args []string) (string, error) {
	var cfgPath string
	var keysPath string
	var accessKey string
	var accessKeyName string
	var provider string
	var modelOverride string
	var upstreamKey string
	var plainKey bool

	fs := flag.NewFlagSet("token create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&accessKey, "access-key", "", "direct access key value")
	fs.StringVar(&accessKeyName, "access-key-name", "", "read access key value by name from keys.yaml")
	fs.BoolVar(&plainKey, "k-plain", false, "embed access key as k=<plain> (default k64)")
	fs.StringVar(&provider, "p", "", "provider")
	fs.StringVar(&provider, "provider", "", "provider")
	fs.StringVar(&modelOverride, "m", "", "model override")
	fs.StringVar(&modelOverride, "model", "", "model override")
	fs.StringVar(&upstreamKey, "uk", "", "BYOK upstream key")
	fs.StringVar(&upstreamKey, "upstream-key", "", "BYOK upstream key")
	if err := fs.Parse(args); err != nil {
		return "", err
	}

	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, _ = store.ResolveDataPaths(cfg, keysPath, "")
	if strings.TrimSpace(accessKey) == "" {
		if strings.TrimSpace(accessKeyName) != "" {
			v, err := accessKeyByName(keysPath, accessKeyName)
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
		strings.ToLower(strings.TrimSpace(provider)),
		strings.TrimSpace(modelOverride),
		strings.TrimSpace(upstreamKey),
		plainKey,
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
