// Package keys resolves and persists private identities for the git plane.
//
// Resolution order for an environment's private key:
//  1. process env var (e.g. DOTENV_PRIVATE_KEY_PRODUCTION) — used in CI/Docker
//  2. the local .env.keys file (gitignored, 0600) — used in local dev
package keys

import (
	"fmt"
	"os"
	"strings"

	"github.com/nees/envvar/internal/crypto"
	"github.com/nees/envvar/internal/envfile"
	"github.com/nees/envvar/internal/store"
)

// KeysFile is the local private-key store filename.
const KeysFile = ".env.keys"

// EnvVarName returns the private-key variable name for an environment.
// The default environment ("" or "default") uses DOTENV_PRIVATE_KEY.
func EnvVarName(env string) string {
	if env == "" || env == "default" {
		return "DOTENV_PRIVATE_KEY"
	}
	return "DOTENV_PRIVATE_KEY_" + strings.ToUpper(env)
}

// Resolve loads the identity able to decrypt env's secrets.
//
// Recipients are project-wide, so an environment-specific key is optional: we
// try the env-specific name first (env var, then file) and fall back to the
// default personal key (DOTENV_PRIVATE_KEY). This lets CI override per
// environment while local dev uses a single key.
func Resolve(env string) (*crypto.Identity, error) {
	names := []string{EnvVarName(env)}
	if def := EnvVarName(""); def != names[0] {
		names = append(names, def)
	}

	// 1. Process environment (CI / Docker).
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return crypto.ParseIdentity(v)
		}
	}

	// 2. Local .env.keys file.
	data, err := os.ReadFile(KeysFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no key for env %q: set %s or run `envvar init`", env, names[0])
		}
		return nil, err
	}
	f := envfile.Parse(data)
	for _, name := range names {
		if v, ok := f.Get(name); ok && v != "" {
			return crypto.ParseIdentity(v)
		}
	}
	return nil, fmt.Errorf("no key for env %q (looked for %v) in %s", env, names, KeysFile)
}

// Save writes/updates an identity for env into .env.keys (0600, atomic).
func Save(env string, id *crypto.Identity) error {
	var f *envfile.File
	if data, err := os.ReadFile(KeysFile); err == nil {
		f = envfile.Parse(data)
	} else if os.IsNotExist(err) {
		f = envfile.Parse([]byte("#/ envvar private keys — DO NOT COMMIT\n"))
	} else {
		return err
	}
	f.Set(EnvVarName(env), id.String())
	return store.WriteFileAtomic(KeysFile, f.Bytes(), 0o600)
}
