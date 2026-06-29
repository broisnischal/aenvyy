package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/nees/envvar/internal/config"
	"github.com/nees/envvar/internal/crypto"
	"github.com/nees/envvar/internal/envfile"
)

// envFilePath resolves which file holds the given environment's secrets.
func envFilePath(cfg *config.Config, env string) string {
	if cfg != nil {
		if p, ok := cfg.Environments[env]; ok && p != "" {
			return p
		}
	}
	if env == "" || env == "default" {
		return ".env"
	}
	return ".env." + env
}

// loadEnvFile reads and parses an env file, returning an empty one if missing.
func loadEnvFile(path string) (*envfile.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return envfile.Parse(nil), nil
		}
		return nil, err
	}
	return envfile.Parse(data), nil
}

// recipientsFromConfig returns the configured recipients (ordered by label) plus
// the ordered labels, falling back to the file's own header when config is nil.
func recipientsFromConfig(cfg *config.Config, f *envfile.File) (labels []string, recips []*crypto.Recipient, err error) {
	m := map[string]*crypto.Recipient{}
	if cfg != nil {
		for label, pk := range cfg.Recipients {
			r, perr := crypto.ParseRecipient(pk)
			if perr != nil {
				return nil, nil, fmt.Errorf("config recipient %q: %w", label, perr)
			}
			m[label] = r
		}
	}
	if len(m) == 0 && f != nil {
		fm, ferr := f.Recipients()
		if ferr != nil {
			return nil, nil, ferr
		}
		m = fm
	}
	if len(m) == 0 {
		return nil, nil, fmt.Errorf("no recipients configured: run `envvar init`")
	}
	for label := range m {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		recips = append(recips, m[label])
	}
	return labels, recips, nil
}

// recipMap zips ordered labels and recipients back into a map.
func recipMap(labels []string, recips []*crypto.Recipient) map[string]*crypto.Recipient {
	m := make(map[string]*crypto.Recipient, len(labels))
	for i, l := range labels {
		m[l] = recips[i]
	}
	return m
}

// maybeConfig loads envvar.toml if present, else returns nil without error.
func maybeConfig() (*config.Config, error) {
	if !config.Exists("") {
		return nil, nil
	}
	return config.Load("")
}
