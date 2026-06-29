package sqlitestore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nees/envvar/internal/server"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreateAndListProjects(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	p, err := s.CreateProject(ctx, "My App")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID != "my-app" {
		t.Fatalf("slug id = %q, want my-app", p.ID)
	}

	// Same name → unique id with suffix, no collision error.
	p2, err := s.CreateProject(ctx, "My App")
	if err != nil {
		t.Fatalf("create dup: %v", err)
	}
	if p2.ID == p.ID {
		t.Fatalf("expected distinct id on collision, both = %q", p.ID)
	}

	list, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(list))
	}
}

func TestPutGetBundleAndETag(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	p, err := s.CreateProject(ctx, "app")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ct := []byte("DATABASE_URL=enc:v1:abc")
	put, err := s.PutBundle(ctx, p.ID, "production", ct)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if put.ETag == "" {
		t.Fatal("empty etag")
	}

	got, err := s.GetBundle(ctx, p.ID, "production")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Ciphertext) != string(ct) {
		t.Fatalf("ciphertext = %q, want %q", got.Ciphertext, ct)
	}
	if got.ETag != put.ETag {
		t.Fatalf("etag mismatch: get %q put %q", got.ETag, put.ETag)
	}

	// Overwrite changes the etag.
	put2, err := s.PutBundle(ctx, p.ID, "production", []byte("DATABASE_URL=enc:v1:xyz"))
	if err != nil {
		t.Fatalf("put2: %v", err)
	}
	if put2.ETag == put.ETag {
		t.Fatal("etag should change when ciphertext changes")
	}
}

func TestGetBundleNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.GetBundle(ctx, "nope", "production"); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPutBundleUnknownProject(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.PutBundle(ctx, "ghost", "production", []byte("x")); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
