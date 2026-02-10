package main

import (
	"os"

	"github.com/r9s-ai/open-next-router/onr"
)

func main() {
	os.Exit(onr.Main(os.Args[1:]))
}
