package main

import (
	"os"

	"github.com/kplane-dev/kplane/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
