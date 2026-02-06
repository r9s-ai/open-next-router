package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/cmd/onr-admin/tui"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
)

func runCLI(args []string) error {
	if len(args) == 0 {
		printRootHelp(os.Stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printRootHelp(os.Stdout)
		return nil
	case "tui":
		return runTUI(args[1:])
	case "token":
		return runToken(args[1:])
	case "crypto":
		return runCrypto(args[1:])
	case "validate":
		return runValidate(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return runTUI(args)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootHelp(w io.Writer) {
	fmt.Fprintln(w, "onr-admin - ONR 管理命令")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  onr-admin <command> <subcommand> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "命令:")
	fmt.Fprintln(w, "  token create            生成 Token Key (onr:v1?)")
	fmt.Fprintln(w, "  token create phase      分阶段输出 Token 生成结果")
	fmt.Fprintln(w, "  crypto encrypt          将明文加密为 ENC[v1:aesgcm:...]")
	fmt.Fprintln(w, "  crypto encrypt-keys     一键加密 keys.yaml 中明文 value")
	fmt.Fprintln(w, "  crypto gen-master-key   生成随机 ONR_MASTER_KEY")
	fmt.Fprintln(w, "  validate all            校验 keys/models/providers")
	fmt.Fprintln(w, "  validate keys           校验 keys.yaml")
	fmt.Fprintln(w, "  validate models         校验 models.yaml")
	fmt.Fprintln(w, "  validate providers      校验 providers DSL 目录")
	fmt.Fprintln(w, "  tui                     打开交互式管理界面")
}

func runTUI(args []string) error {
	var cfgPath string
	var keysPath string
	var modelsPath string
	var backup bool

	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&cfgPath, "c", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path (override config keys.file)")
	fs.StringVar(&modelsPath, "models", "", "models.yaml path (override config models.file)")
	fs.BoolVar(&backup, "backup", true, "backup yaml before saving")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return tui.Run(cfgPath, keysPath, modelsPath, backup, os.Stdin, os.Stdout)
}

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

func runCrypto(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: onr-admin crypto <encrypt|encrypt-keys|gen-master-key> [flags]")
	}
	switch args[0] {
	case "encrypt":
		return runCryptoEncrypt(args[1:])
	case "encrypt-keys":
		return runCryptoEncryptKeys(args[1:])
	case "gen-master-key":
		return runCryptoGenMasterKey(args[1:])
	default:
		return fmt.Errorf("unknown crypto subcommand %q", args[0])
	}
}

func runCryptoEncrypt(args []string) error {
	var text string
	fs := flag.NewFlagSet("crypto encrypt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&text, "text", "", "plain text to encrypt (if empty, read from stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plain := strings.TrimSpace(text)
	if plain == "" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		plain = strings.TrimSpace(string(b))
	}
	if plain == "" {
		return errors.New("missing input: provide --text or pipe stdin")
	}
	out, err := keystore.Encrypt(plain)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	fmt.Println(out)
	return nil
}

func runCryptoGenMasterKey(args []string) error {
	var format string
	var exportLine bool

	fs := flag.NewFlagSet("crypto gen-master-key", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&format, "format", "base64", "output format: base64|base64url")
	fs.BoolVar(&exportLine, "export", false, "print as shell export line")
	if err := fs.Parse(args); err != nil {
		return err
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generate random key: %w", err)
	}

	var out string
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "base64":
		out = base64.StdEncoding.EncodeToString(buf)
	case "base64url":
		out = base64.RawURLEncoding.EncodeToString(buf)
	default:
		return errors.New("invalid --format, expect base64 or base64url")
	}

	if exportLine {
		fmt.Printf("export ONR_MASTER_KEY='%s'\n", out)
		return nil
	}
	fmt.Println(out)
	return nil
}

func runCryptoEncryptKeys(args []string) error {
	var cfgPath string
	var keysPath string
	var backup bool
	var dryRun bool

	fs := flag.NewFlagSet("crypto encrypt-keys", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path")
	fs.BoolVar(&backup, "backup", true, "backup keys.yaml before saving")
	fs.BoolVar(&dryRun, "dry-run", false, "print result without writing file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, _ = store.ResolveDataPaths(cfg, keysPath, "")
	doc, err := store.LoadOrInitKeysDoc(keysPath)
	if err != nil {
		return fmt.Errorf("load keys: %w", err)
	}

	n, err := store.EncryptKeysDocValues(doc)
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Println("encrypt-keys: no plaintext value found")
		return nil
	}
	if dryRun {
		fmt.Printf("encrypt-keys: %d value(s) would be encrypted (dry-run)\n", n)
		return nil
	}

	if err := store.ValidateKeysDoc(doc); err != nil {
		return err
	}
	b, err := store.EncodeYAML(doc)
	if err != nil {
		return err
	}
	if err := store.WriteAtomic(keysPath, b, backup); err != nil {
		return err
	}
	fmt.Printf("encrypt-keys: encrypted %d value(s) in %s\n", n, keysPath)
	return nil
}

func runValidate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: onr-admin validate <all|keys|models|providers> [flags]")
	}
	target := strings.ToLower(strings.TrimSpace(args[0]))

	var cfgPath string
	var keysPath string
	var modelsPath string
	var providersDir string
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path")
	fs.StringVar(&modelsPath, "models", "", "models.yaml path")
	fs.StringVar(&providersDir, "providers-dir", "", "providers dir path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, modelsPath = store.ResolveDataPaths(cfg, keysPath, modelsPath)
	if strings.TrimSpace(providersDir) == "" {
		if cfg != nil && strings.TrimSpace(cfg.Providers.Dir) != "" {
			providersDir = strings.TrimSpace(cfg.Providers.Dir)
		} else {
			providersDir = "./config/providers"
		}
	}

	switch target {
	case "keys":
		return validateKeys(keysPath)
	case "models":
		return validateModels(modelsPath)
	case "providers":
		return validateProviders(providersDir)
	case "all":
		if err := validateKeys(keysPath); err != nil {
			return err
		}
		if err := validateModels(modelsPath); err != nil {
			return err
		}
		if err := validateProviders(providersDir); err != nil {
			return err
		}
		fmt.Println("validate all: OK")
		return nil
	default:
		return fmt.Errorf("unknown validate target %q", target)
	}
}

func validateKeys(path string) error {
	doc, err := store.LoadOrInitKeysDoc(path)
	if err != nil {
		return fmt.Errorf("load keys yaml: %w", err)
	}
	if err := store.ValidateKeysDoc(doc); err != nil {
		return fmt.Errorf("keys yaml structure invalid: %w", err)
	}
	if _, err := keystore.Load(path); err != nil {
		return fmt.Errorf("keystore load failed: %w", err)
	}
	fmt.Println("validate keys: OK")
	return nil
}

func validateModels(path string) error {
	doc, err := store.LoadOrInitModelsDoc(path)
	if err != nil {
		return fmt.Errorf("load models yaml: %w", err)
	}
	if err := store.ValidateModelsDoc(doc); err != nil {
		return fmt.Errorf("models yaml structure invalid: %w", err)
	}
	if _, err := models.Load(path); err != nil {
		return fmt.Errorf("models load failed: %w", err)
	}
	fmt.Println("validate models: OK")
	return nil
}

func validateProviders(path string) error {
	if _, err := dslconfig.ValidateProvidersDir(path); err != nil {
		return fmt.Errorf("validate providers dir %s failed: %w", path, err)
	}
	fmt.Println("validate providers: OK")
	return nil
}
