package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/config"
	"github.com/nees/envvar/internal/crypto"
	"github.com/nees/envvar/internal/keys"
	"github.com/nees/envvar/internal/store"
)

func newRekeyCmd() *cobra.Command {
	var addRecipients []string
	cmd := &cobra.Command{
		Use:   "rekey",
		Short: "Re-wrap all secrets under fresh keys / updated recipients",
		Long: "Decrypts every value with your private key and re-encrypts it with new\n" +
			"per-value data keys for the project's recipients. Use this to rotate keys\n" +
			"or, with --add-recipient, to grant a new public key access to every\n" +
			"existing secret (the recipient is also persisted to envvar.toml).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := maybeConfig()
			if err != nil {
				return err
			}
			if cfg == nil {
				return fmt.Errorf("no %s in this directory: run `envvar init` first", config.FileName)
			}

			added := 0
			for _, spec := range addRecipients {
				label, pk, ok := strings.Cut(spec, "=")
				if !ok {
					return fmt.Errorf("--add-recipient expects label=pk_..., got %q", spec)
				}
				label, pk = strings.TrimSpace(label), strings.TrimSpace(pk)
				if _, err := crypto.ParseRecipient(pk); err != nil {
					return fmt.Errorf("recipient %q: %w", label, err)
				}
				if cfg.Recipients == nil {
					cfg.Recipients = map[string]string{}
				}
				cfg.Recipients[label] = pk
				added++
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
			id, err := keys.Resolve(envFlag)
			if err != nil {
				return err
			}

			f.SetRecipients(labels, recipMap(labels, recips))
			if err := f.RekeyInPlace(id, recips, id); err != nil {
				return err
			}
			if err := store.WriteFileAtomic(path, f.Bytes(), 0o644); err != nil {
				return err
			}
			if added > 0 {
				if err := config.Save("", cfg); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rekeyed %s to %d recipient(s)%s\n",
				path, len(recips), addedSuffix(added))
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	cmd.Flags().StringArrayVar(&addRecipients, "add-recipient", nil,
		"grant a recipient access to all secrets: label=pk_... (repeatable)")
	return cmd
}

func addedSuffix(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" (+%d new)", n)
}
