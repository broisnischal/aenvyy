// Package sqlitestore implements server.Store on top of SQLite using the
// pure-Go modernc.org/sqlite driver (no CGO), so the server still compiles to a
// single static binary. It opens the database in WAL mode for crash-safe,
// concurrent-reader durability and stores only ciphertext envelopes + metadata.
package sqlitestore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nees/envvar/internal/server"
)

// Store is a SQLite-backed server.Store.
type Store struct {
	db *sql.DB
}

// compile-time assertion that Store satisfies the interface.
var _ server.Store = (*Store)(nil)

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. WAL + a busy timeout + enforced foreign keys are set via the DSN;
// _txlock=immediate avoids write-write deadlocks under concurrent writers.
func Open(path string) (*Store, error) {
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_txlock=immediate"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite serializes access per connection; a small pool is plenty
	// and avoids "database is locked" surprises with WAL writers.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS projects (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS secret_bundles (
  project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  environment TEXT NOT NULL,
  ciphertext  BLOB NOT NULL,
  etag        TEXT NOT NULL,
  updated_at  INTEGER NOT NULL,
  PRIMARY KEY (project_id, environment)
);

CREATE TABLE IF NOT EXISTS audit_events (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  actor      TEXT NOT NULL DEFAULT '',
  action     TEXT NOT NULL,
  secret_ref TEXT NOT NULL DEFAULT '',
  ts         INTEGER NOT NULL
);
`

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// CreateProject inserts a project with a slug id derived from name. On a slug
// collision it retries with a short random suffix.
func (s *Store) CreateProject(ctx context.Context, name string) (server.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return server.Project{}, fmt.Errorf("project name is required")
	}
	base := slugify(name)
	now := time.Now().Unix()

	for attempt := 0; attempt < 5; attempt++ {
		id := base
		if attempt > 0 {
			id = base + "-" + randHex(3)
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO projects (id, name, created_at) VALUES (?, ?, ?)`,
			id, name, now)
		if err == nil {
			s.audit(ctx, "", "project.create", id)
			return server.Project{ID: id, Name: name, CreatedAt: now}, nil
		}
		if isUniqueViolation(err) {
			continue // slug taken, try a suffixed id
		}
		return server.Project{}, fmt.Errorf("create project: %w", err)
	}
	return server.Project{}, fmt.Errorf("create project: could not allocate unique id for %q", name)
}

// ListProjects returns all projects, newest first.
func (s *Store) ListProjects(ctx context.Context) ([]server.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM projects ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	out := []server.Project{}
	for rows.Next() {
		var p server.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetBundle returns the ciphertext bundle for (projectID, env).
func (s *Store) GetBundle(ctx context.Context, projectID, env string) (server.Bundle, error) {
	var b server.Bundle
	err := s.db.QueryRowContext(ctx,
		`SELECT ciphertext, etag, updated_at FROM secret_bundles
		 WHERE project_id = ? AND environment = ?`,
		projectID, env).Scan(&b.Ciphertext, &b.ETag, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return server.Bundle{}, server.ErrNotFound
	}
	if err != nil {
		return server.Bundle{}, fmt.Errorf("get bundle: %w", err)
	}
	return b, nil
}

// PutBundle creates or replaces the ciphertext bundle for (projectID, env).
func (s *Store) PutBundle(ctx context.Context, projectID, env string, ciphertext []byte) (server.Bundle, error) {
	// Require the project to exist so we return a clean 404 rather than a raw
	// foreign-key error.
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE id = ?`, projectID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return server.Bundle{}, server.ErrNotFound
	}
	if err != nil {
		return server.Bundle{}, fmt.Errorf("check project: %w", err)
	}

	etag := etagOf(ciphertext)
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO secret_bundles (project_id, environment, ciphertext, etag, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, environment)
		 DO UPDATE SET ciphertext = excluded.ciphertext,
		               etag       = excluded.etag,
		               updated_at = excluded.updated_at`,
		projectID, env, ciphertext, etag, now)
	if err != nil {
		return server.Bundle{}, fmt.Errorf("put bundle: %w", err)
	}
	s.audit(ctx, "", "secret.put", projectID+"/"+env)
	return server.Bundle{Ciphertext: ciphertext, ETag: etag, UpdatedAt: now}, nil
}

// audit records an event best-effort; a logging failure must not fail the write.
func (s *Store) audit(ctx context.Context, actor, action, ref string) {
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO audit_events (actor, action, secret_ref, ts) VALUES (?, ?, ?, ?)`,
		actor, action, ref, time.Now().Unix())
}

// etagOf returns a strong ETag (quoted sha256 hex) over the bundle bytes.
func etagOf(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// slugify lowercases name and collapses runs of non-alphanumerics into single
// hyphens, suitable for a URL path segment. Falls back to "project" if empty.
func slugify(name string) string {
	var sb strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			sb.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && sb.Len() > 0 {
				sb.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	slug := strings.Trim(sb.String(), "-")
	if slug == "" {
		return "project"
	}
	return slug
}

// randHex returns n random bytes as hex (2n chars).
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// isUniqueViolation reports whether err is a SQLite UNIQUE/PRIMARY KEY conflict.
// modernc/sqlite surfaces these in the error string with code 2067/1555.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "(2067)") || strings.Contains(msg, "(1555)")
}
