package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/envfile"
	"github.com/nees/envvar/internal/keys"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run -- command [args...]",
		Short: "Decrypt secrets in memory and run a command with them injected",
		Long:  "Resolves the environment's private key, decrypts the secrets file, and executes the given command with the decrypted values injected as environment variables. Plaintext is never written to disk.",
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
			id, err := keys.Resolve(envFlag)
			if err != nil {
				return err
			}
			secrets, err := f.Decrypted(id)
			if err != nil {
				return err
			}
			secrets, err = envfile.Compose(secrets)
			if err != nil {
				return err
			}

			env := os.Environ()
			for k, v := range secrets {
				env = append(env, k+"="+v)
			}

			bin, err := exec.LookPath(args[0])
			if err != nil {
				return fmt.Errorf("command not found: %s", args[0])
			}
			child := exec.Command(bin, args[1:]...)
			child.Env = env
			child.Stdin = os.Stdin
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()
			if err := child.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&envFlag, "env", "e", "default", "environment")
	return cmd
}
