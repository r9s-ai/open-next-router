package main

import (
	"fmt"
	"os"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
