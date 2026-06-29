package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/keys"
	"github.com/nees/envvar/internal/store"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set KEY=VALUE [KEY=VALUE ...]",
		Short: "Encrypt one or more values into the environment file",
		Args:  cobra.MinimumNArgs(1),
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
			labels, recips, err := recipientsFromConfig(cfg, f)
			if err != nil {
				return err
			}
			f.SetRecipients(labels, recipMap(labels, recips))

			for _, a := range args {
				k, v, ok := strings.Cut(a, "=")
				if !ok {
					return fmt.Errorf("expected KEY=VALUE, got %q", a)
				}
				if err := f.SetSecret(strings.TrimSpace(k), v, recips); err != nil {
					return err
				}
			}
			if err := store.WriteFileAtomic(path, f.Bytes(), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "encrypted %d value(s) into %s\n", len(args), path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	return cmd
}

func newEncryptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt all plaintext values in the environment file",
		Args:  cobra.NoArgs,
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
			labels, recips, err := recipientsFromConfig(cfg, f)
			if err != nil {
				return err
			}
			// Verify round-trip with the local identity when available.
			verify, _ := keys.Resolve(envFlag)
			f.SetRecipients(labels, recipMap(labels, recips))
			if err := f.EncryptInPlace(recips, verify); err != nil {
				return err
			}
			if err := store.WriteFileAtomic(path, f.Bytes(), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "encrypted plaintext values in %s\n", path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	return cmd
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [KEY]",
		Short: "Decrypt and print a value (or all values) — handle with care",
		Args:  cobra.MaximumNArgs(1),
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
			id, err := keys.Resolve(envFlag)
			if err != nil {
				return err
			}
			vals, err := f.Decrypted(id)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(args) == 1 {
				v, ok := vals[args[0]]
				if !ok {
					return fmt.Errorf("no such key %q", args[0])
				}
				fmt.Fprintln(out, v)
				return nil
			}
			ks := make([]string, 0, len(vals))
			for k := range vals {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			for _, k := range ks {
				fmt.Fprintf(out, "%s=%s\n", k, vals[k])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	return cmd
}
