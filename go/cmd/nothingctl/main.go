package main

import (
	"fmt"
	"os"

	"github.com/Limplom/nothingctl/internal/cmd"
)

// Version is set at build time via -ldflags
var Version = "dev"

func main() {
	cmd.SetVersion(Version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
