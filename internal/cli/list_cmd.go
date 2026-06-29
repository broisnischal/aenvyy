package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/crypto"
	"github.com/nees/envvar/internal/envfile"
	"github.com/nees/envvar/internal/store"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List keys and their encryption status (no values revealed)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := maybeConfig()
			if err != nil {
				return err
			}
			path := envFilePath(cfg, envFlag)
			f, err := loadEnvFile(path)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			n := 0
			for _, kv := range f.Pairs() {
				if envfile.IsHeaderKey(kv[0]) {
					continue
				}
				status := "plaintext"
				if crypto.IsEncrypted(kv[1]) {
					status = "encrypted"
				}
				fmt.Fprintf(out, "%-32s %s\n", kv[0], status)
				n++
			}
			if n == 0 {
				fmt.Fprintf(out, "no keys in %s\n", path)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	return cmd
}

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm KEY [KEY ...]",
		Aliases: []string{"unset"},
		Short:   "Remove one or more keys from the environment file",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := maybeConfig()
			if err != nil {
				return err
			}
			path := envFilePath(cfg, envFlag)
			f, err := loadEnvFile(path)
			if err != nil {
				return err
			}
			removed := 0
			for _, k := range args {
				if f.Unset(k) {
					removed++
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: no such key %q\n", k)
				}
			}
			if removed == 0 {
				return fmt.Errorf("nothing removed")
			}
			if err := store.WriteFileAtomic(path, f.Bytes(), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d key(s) from %s\n", removed, path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	return cmd
}
