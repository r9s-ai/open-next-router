package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-admin/internal/store"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/spf13/cobra"
)

func newCryptoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crypto",
		Short: "Crypto and key helpers",
	}
	cmd.AddCommand(
		newCryptoEncryptCmd(),
		newCryptoEncryptKeysCmd(),
		newCryptoGenMasterKeyCmd(),
	)
	return cmd
}

func newCryptoEncryptCmd() *cobra.Command {
	var text string
	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt plaintext to ENC[v1:aesgcm:...]",
		RunE: func(cmd *cobra.Command, args []string) error {
			plain, err := resolveEncryptPlaintext(strings.TrimSpace(text), cmd.InOrStdin(), isTerminalReader(cmd.InOrStdin()))
			if err != nil {
				return err
			}
			if plain == "" {
				return errors.New("missing input: provide --text, enter a line, or pipe stdin")
			}
			out, err := keystore.Encrypt(plain)
			if err != nil {
				return fmt.Errorf("encrypt: %w", err)
			}
			fmt.Println(out)
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "plain text to encrypt (if empty, read from stdin)")
	return cmd
}

func resolveEncryptPlaintext(text string, in io.Reader, inTerminal bool) (string, error) {
	plain := strings.TrimSpace(text)
	if plain != "" {
		return plain, nil
	}

	if inTerminal {
		r := bufio.NewReader(in)
		line, err := r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimSpace(line), nil
	}

	b, err := io.ReadAll(in)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func isTerminalReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func newCryptoGenMasterKeyCmd() *cobra.Command {
	var format string
	var exportLine bool
	cmd := &cobra.Command{
		Use:   "gen-master-key",
		Short: "Generate a random ONR_MASTER_KEY",
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}
	fs := cmd.Flags()
	fs.StringVar(&format, "format", "base64", "output format: base64|base64url")
	fs.BoolVar(&exportLine, "export", false, "print as shell export line")
	return cmd
}

func newCryptoEncryptKeysCmd() *cobra.Command {
	opts := cryptoEncryptKeysOptions{cfgPath: "onr.yaml", backup: true}
	cmd := &cobra.Command{
		Use:   "encrypt-keys",
		Short: "Encrypt plaintext values in keys.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(opts.cfgPath))
			keysPath, _ := store.ResolveDataPaths(cfg, opts.keysPath, "")
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
			if opts.dryRun {
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
			if err := store.WriteAtomic(keysPath, b, opts.backup); err != nil {
				return err
			}
			fmt.Printf("encrypt-keys: encrypted %d value(s) in %s\n", n, keysPath)
			return nil
		},
	}
	fs := cmd.Flags()
	fs.StringVar(&opts.cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.keysPath, "keys", "", "keys.yaml path")
	fs.BoolVar(&opts.backup, "backup", true, "backup keys.yaml before saving")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print result without writing file")
	return cmd
}

type cryptoEncryptKeysOptions struct {
	cfgPath  string
	keysPath string
	backup   bool
	dryRun   bool
}
