// Package cli wires the envvar command-line interface.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/version"
)

// envFlag is shared by commands that operate on a single environment.
var envFlag string

// NewRootCmd builds the root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "envvar",
		Short:         "envvar — git-native, post-quantum, agent-ready secrets",
		Long:          "envvar manages environment variables as an encrypted, git-committable file and (soon) a self-hostable server.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.AddCommand(
		newKeygenCmd(),
		newInitCmd(),
		newSetCmd(),
		newEncryptCmd(),
		newGetCmd(),
		newRunCmd(),
		newServerCmd(),
		newStubCmd("sync", "Push secrets to platforms (GitHub, Vercel, Azure, ...)"),
		newStubCmd("pull", "Import secrets from platforms into the encrypted file"),
		newStubCmd("rekey", "Re-wrap all secrets under a new algorithm/key/recipients"),
		newStubCmd("mcp", "Run the agent MCP server (use-but-never-see grants)"),
	)
	return root
}

func newStubCmd(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short + " (not implemented yet)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("`envvar %s` is planned but not implemented yet.\n", name)
			return nil
		},
	}
}
