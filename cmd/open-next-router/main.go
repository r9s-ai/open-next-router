package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/edgefn/open-next-router/internal/onrserver"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "open-next-router.yaml", "path to config yaml")
	flag.Parse()

	if err := onrserver.Run(cfgPath); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
