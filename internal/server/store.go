package server

import (
	"context"
	"errors"
)

// ErrNotFound is returned by a Store when a requested project or bundle does
// not exist. Handlers map it to HTTP 404.
var ErrNotFound = errors.New("not found")

// Project is a server-plane project record.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"` // unix seconds
}

// Bundle is the stored ciphertext for one (project, environment). The server is
// zero-knowledge: Ciphertext is opaque to it (the enc:v1 env-file envelope). The
// ETag is a strong validator over the bytes, used for conditional GET / 304.
type Bundle struct {
	Ciphertext []byte
	ETag       string
	UpdatedAt  int64 // unix seconds
}

// Store is the server's persistence layer. It holds only ciphertext envelopes
// plus metadata — never plaintext. The SQLite implementation lives in
// internal/sqlitestore; the interface keeps handlers backend-agnostic so a
// Postgres backend can be added later without touching them.
type Store interface {
	// CreateProject creates a project with a unique id derived from name.
	CreateProject(ctx context.Context, name string) (Project, error)
	// ListProjects returns all projects, newest first.
	ListProjects(ctx context.Context) ([]Project, error)
	// GetBundle returns the ciphertext bundle for (projectID, env), or
	// ErrNotFound if the project or bundle does not exist.
	GetBundle(ctx context.Context, projectID, env string) (Bundle, error)
	// PutBundle stores (creating or replacing) the ciphertext bundle for
	// (projectID, env). It returns ErrNotFound if the project does not exist.
	PutBundle(ctx context.Context, projectID, env string, ciphertext []byte) (Bundle, error)
	// Close releases the underlying database handle.
	Close() error
}
