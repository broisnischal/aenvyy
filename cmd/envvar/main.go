// Command envvar is the CLI and self-hostable server for envvar — a git-native,
// post-quantum, agent-ready environment-variable manager.
package main

import (
	"fmt"
	"os"

	"github.com/nees/envvar/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
