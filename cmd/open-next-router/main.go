package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/edgefn/open-next-router/internal/onrserver"
	"github.com/edgefn/open-next-router/internal/version"
)

func main() {
	var cfgPath string
	var showVersion bool
	flag.StringVar(&cfgPath, "config", "open-next-router.yaml", "path to config yaml")
	flag.BoolVar(&showVersion, "version", false, "show version information")
	flag.Parse()

	// Show version and exit
	if showVersion {
		fmt.Println(version.Get())
		return
	}

	if err := onrserver.Run(cfgPath); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
