package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nees/envvar/internal/crypto"
	"github.com/nees/envvar/internal/envfile"
	"github.com/nees/envvar/internal/keys"
)

func newGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guard",
		Short: "Pre-commit check: block plaintext secrets or private keys from a commit",
		Long: "Scans the git staging area and fails if it would commit the private-key\n" +
			"file or any env file containing an unencrypted secret value. Installed as a\n" +
			"pre-commit hook by `envvar init`; also runnable directly.",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			files, err := stagedFiles()
			if err != nil {
				return err
			}
			var problems []string
			for _, name := range files {
				base := filepath.Base(name)
				if base == keys.KeysFile {
					problems = append(problems,
						fmt.Sprintf("%s — private key file must never be committed", name))
					continue
				}
				if !isSecretEnvFile(base) {
					continue
				}
				content, err := stagedContent(name)
				if err != nil {
					continue // file staged for deletion, etc.
				}
				problems = append(problems, scanEnvForLeaks(name, content)...)
			}

			if len(problems) > 0 {
				out := cmd.ErrOrStderr()
				fmt.Fprintln(out, "envvar: refusing commit — unencrypted secrets or keys detected:")
				for _, p := range problems {
					fmt.Fprintf(out, "  • %s\n", p)
				}
				fmt.Fprintln(out, "fix: run `envvar encrypt` (or unstage the file), then commit again.")
				return fmt.Errorf("%d issue(s) found", len(problems))
			}
			return nil
		},
	}
}

// isSecretEnvFile reports whether a filename is an env file that should hold
// only ciphertext. Example/sample/template files are exempt (placeholders), and
// .local overrides are gitignored anyway.
func isSecretEnvFile(base string) bool {
	if base != ".env" && !strings.HasPrefix(base, ".env.") {
		return false
	}
	switch {
	case base == keys.KeysFile,
		base == ".env.example", base == ".env.sample", base == ".env.template",
		strings.HasSuffix(base, ".local"):
		return false
	}
	return true
}

// scanEnvForLeaks returns human-readable problems for an env file's staged
// content: any non-header pair whose value is not an envvar envelope, plus any
// private-key (sk_) material. Pure function — unit-testable without git.
func scanEnvForLeaks(name string, content []byte) []string {
	var problems []string
	f := envfile.Parse(content)
	for _, kv := range f.Pairs() {
		key, val := kv[0], kv[1]
		if envfile.IsHeaderKey(key) {
			continue
		}
		if strings.Contains(val, "sk_") {
			problems = append(problems, fmt.Sprintf("%s: %s looks like a private key", name, key))
			continue
		}
		if val != "" && !crypto.IsEncrypted(val) {
			problems = append(problems, fmt.Sprintf("%s: %s is an unencrypted value", name, key))
		}
	}
	return problems
}

// stagedFiles lists paths staged for commit (added/copied/modified/renamed).
func stagedFiles() ([]string, error) {
	out, err := runGit("diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) > 0 {
			files = append(files, string(p))
		}
	}
	return files, nil
}

// stagedContent returns the staged (index) content of a path.
func stagedContent(name string) ([]byte, error) {
	return runGit("show", ":"+name)
}

func runGit(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
