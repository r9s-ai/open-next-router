package cli

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
	"github.com/r9s-ai/open-next-router/internal/keystore"
)

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
