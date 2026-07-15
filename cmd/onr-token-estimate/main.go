package main

import (
	"os"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
)

func main() {
	os.Exit(usageestimate.RunCLI(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
