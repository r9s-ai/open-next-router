package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/internal/keystore"
)

func main() {
	var text string
	flag.StringVar(&text, "text", "", "plain text to encrypt (if empty, read from stdin)")
	flag.Parse()

	plain := strings.TrimSpace(text)
	if plain == "" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}
		plain = strings.TrimSpace(string(b))
	}
	if plain == "" {
		fmt.Fprintln(os.Stderr, "missing input: provide --text or pipe stdin")
		os.Exit(2)
	}

	out, err := keystore.Encrypt(plain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encrypt: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}
