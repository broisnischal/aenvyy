// Package config reads and writes the project's envvar.toml.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/nees/envvar/internal/store"
)

// FileName is the canonical project config filename.
const FileName = "envvar.toml"

// Config is the on-disk project configuration.
type Config struct {
	Project      Project            `toml:"project"`
	Crypto       Crypto             `toml:"crypto"`
	Recipients   map[string]string  `toml:"recipients"`   // label -> pk_...
	Environments map[string]string  `toml:"environments"` // name -> file path
	Sync         map[string]SyncTgt `toml:"sync"`         // platform -> target
}

type Project struct {
	Name string `toml:"name"`
}

type Crypto struct {
	KEM  string `toml:"kem"`
	AEAD string `toml:"aead"`
}

type SyncTgt struct {
	Repo    string `toml:"repo,omitempty"`
	Project string `toml:"project,omitempty"`
}

// Load reads envvar.toml from the current directory (or path).
func Load(path string) (*Config, error) {
	if path == "" {
		path = FileName
	}
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return &c, nil
}

// Save writes the config atomically.
func Save(path string, c *Config) error {
	if path == "" {
		path = FileName
	}
	var buf []byte
	b := &bytesBuffer{}
	if err := toml.NewEncoder(b).Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	buf = b.Bytes()
	return store.WriteFileAtomic(path, buf, 0o644)
}

// Exists reports whether a config file is present.
func Exists(path string) bool {
	if path == "" {
		path = FileName
	}
	_, err := os.Stat(path)
	return err == nil
}

// bytesBuffer is a tiny io.Writer wrapper so we can encode then write atomically.
type bytesBuffer struct{ b []byte }

func (w *bytesBuffer) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
func (w *bytesBuffer) Bytes() []byte               { return w.b }
