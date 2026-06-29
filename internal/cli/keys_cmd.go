package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/crypto"
)

func newKeygenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "keygen",
		Short: "Generate a new hybrid (X25519+ML-KEM-768) keypair",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, err := crypto.GenerateIdentity()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "# private key (keep secret — set as DOTENV_PRIVATE_KEY):")
			fmt.Fprintln(out, id.String())
			fmt.Fprintln(out, "# public recipient (safe to share/commit):")
			fmt.Fprintln(out, id.Recipient().String())
			return nil
		},
	}
}
