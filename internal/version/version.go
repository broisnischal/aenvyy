// Package version holds build-time version metadata.
package version

// Version is the CLI/server version, overridable at build time via
// -ldflags "-X github.com/nees/envvar/internal/version.Version=...".
var Version = "0.0.0-dev"
