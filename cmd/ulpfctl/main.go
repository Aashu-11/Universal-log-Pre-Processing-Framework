// Command ulpfctl is the ULPF operator CLI: vault integrity operations,
// parser testing/linting, partition sync and pipeline stats.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "ulpfctl",
		Short:   "ULPF operator CLI",
		Version: Version,
	}
	root.AddCommand(newVaultCmd())
	root.AddCommand(newParserCmd())
	root.AddCommand(newPartitionsCmd())
	root.AddCommand(newGenCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
