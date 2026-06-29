package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/config"
	"github.com/nees/envvar/internal/crypto"
	"github.com/nees/envvar/internal/keys"
	"github.com/nees/envvar/internal/store"
)

func newInitCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up keys, config, and .gitignore for a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if config.Exists("") {
				return fmt.Errorf("%s already exists; refusing to overwrite", config.FileName)
			}

			// 1. Generate a personal identity and store the private half locally.
			id, err := crypto.GenerateIdentity()
			if err != nil {
				return err
			}
			if err := keys.Save(envFlag, id); err != nil {
				return err
			}

			// 2. Write envvar.toml with the personal recipient.
			if name == "" {
				wd, _ := os.Getwd()
				name = baseName(wd)
			}
			cfg := &config.Config{
				Project:      config.Project{Name: name},
				Crypto:       config.Crypto{KEM: crypto.AlgHybridX25519MLKEM768, AEAD: "aes-256-gcm"},
				Recipients:   map[string]string{"personal": id.Recipient().String()},
				Environments: map[string]string{"default": ".env", "production": ".env.production"},
			}
			if err := config.Save("", cfg); err != nil {
				return err
			}

			// 3. Ensure .gitignore protects the private key and plaintext .env.
			if err := ensureGitignore(); err != nil {
				return err
			}

			// 4. Install the pre-commit guard (best-effort; only inside a git repo).
			installed, herr := installPreCommitHook()
			if herr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  note: could not install pre-commit hook: %v\n", herr)
			}

			fmt.Fprintf(out, "Initialized envvar project %q\n", name)
			fmt.Fprintf(out, "  wrote %s (config) and %s (private key, gitignored)\n", config.FileName, keys.KeysFile)
			if installed {
				fmt.Fprintln(out, "  installed .git/hooks/pre-commit guard (blocks plaintext leaks)")
			}
			fmt.Fprintln(out, "  next: `envvar set KEY=value` then `envvar run -- your-cmd`")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (defaults to directory name)")
	return cmd
}

// gitignoreEntries are added by init to prevent secret/key leaks.
//
// Note: the encrypted .env / .env.<env> files are intentionally NOT ignored —
// committing them is the whole point. We ignore only the private keys and
// conventional local plaintext overrides. A pre-commit guard (planned) enforces
// that committed env files contain no plaintext secret values.
var gitignoreEntries = []string{keys.KeysFile, ".env.local", ".env.*.local"}

func ensureGitignore() error {
	const path = ".gitignore"
	existing, _ := os.ReadFile(path)
	have := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		have[strings.TrimSpace(line)] = true
	}
	var add []string
	for _, e := range gitignoreEntries {
		if !have[e] {
			add = append(add, e)
		}
	}
	if len(add) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		sb.WriteByte('\n')
	}
	sb.WriteString("\n# envvar — never commit private keys or plaintext env\n")
	sb.WriteString(strings.Join(add, "\n"))
	sb.WriteByte('\n')
	return store.WriteFileAtomic(path, []byte(sb.String()), 0o644)
}

// preCommitSnippet is the shell that invokes the guard; it no-ops cleanly when
// the envvar binary isn't on PATH so the hook never blocks unrelated work.
const preCommitSnippet = `if command -v envvar >/dev/null 2>&1; then
  envvar guard || exit 1
fi
`

const preCommitHook = "#!/bin/sh\n# Installed by envvar — block committing plaintext secrets or private keys.\n" + preCommitSnippet

// installPreCommitHook writes (or augments) .git/hooks/pre-commit to run the
// guard. Returns false (no error) when not in a standard git repo. If a hook
// already exists, the snippet is appended unless it is already present.
func installPreCommitHook() (bool, error) {
	info, err := os.Stat(".git")
	if err != nil || !info.IsDir() {
		return false, nil // not a git repo, or a worktree/submodule .git file — skip
	}
	hooksDir := filepath.Join(".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return false, err
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")

	existing, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, store.WriteFileAtomic(hookPath, []byte(preCommitHook), 0o755)
		}
		return false, err
	}
	if strings.Contains(string(existing), "envvar guard") {
		return false, nil // already installed
	}
	merged := string(existing)
	if !strings.HasSuffix(merged, "\n") {
		merged += "\n"
	}
	merged += "\n# envvar guard\n" + preCommitSnippet
	return true, store.WriteFileAtomic(hookPath, []byte(merged), 0o755)
}

func baseName(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	if path == "" {
		return "app"
	}
	return path
}
