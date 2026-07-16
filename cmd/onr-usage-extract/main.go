package main

import (
	"os"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageextract"
)

func main() {
	os.Exit(usageextract.RunCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
