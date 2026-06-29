// Package server implements the envvar control-plane HTTP API (server plane).
//
// Design: the server is zero-knowledge by default — it stores and serves only
// ciphertext envelopes; clients (web UI via WASM, SDKs) encrypt and decrypt.
// Endpoints are versioned under /v1. The web UI is served from embedded assets
// (see webfs.go); secrets are read far more than written, so the read path is
// built to be cache-friendly (ETag/304) for fast network fetches.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/nees/envvar/internal/version"
)

// maxBundleBytes caps a single PUT body. Bundles are env-file ciphertext —
// kilobytes in practice; the limit is a safety valve against runaway uploads.
const maxBundleBytes = 8 << 20 // 8 MiB

// Server holds API dependencies.
type Server struct {
	log   *slog.Logger
	store Store
}

// New constructs a Server backed by store. store may be nil (the /v1 data
// routes then return 503), so the binary still boots to serve the UI/health.
func New(log *slog.Logger, store Store) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{log: log, store: store}
}

// Handler returns the root http.Handler with all routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/projects", s.handleListProjects)
	mux.HandleFunc("POST /v1/projects", s.handleCreateProject)
	mux.HandleFunc("GET /v1/projects/{id}/environments/{env}/secrets", s.handleGetSecrets)
	mux.HandleFunc("PUT /v1/projects/{id}/environments/{env}/secrets", s.handlePutSecrets)

	// Web UI (embedded SPA) as a fallback for all non-API routes.
	mux.Handle("/", webHandler())

	return s.withCommon(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version.Version,
	})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.storeUnavailable(w)
		return
	}
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		s.serverError(w, "list projects", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.storeUnavailable(w)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	p, err := s.store.CreateProject(r.Context(), body.Name)
	if err != nil {
		s.serverError(w, "create project", err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleGetSecrets(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.storeUnavailable(w)
		return
	}
	id, env := r.PathValue("id"), r.PathValue("env")
	b, err := s.store.GetBundle(r.Context(), id, env)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "no secrets for that project/environment")
		return
	}
	if err != nil {
		s.serverError(w, "get bundle", err)
		return
	}

	w.Header().Set("ETag", b.ETag)
	w.Header().Set("Cache-Control", "no-cache") // must revalidate, but 304 is cheap
	// Conditional GET: unchanged bundle costs ~0 over the wire.
	if match := r.Header.Get("If-None-Match"); match != "" && match == b.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b.Ciphertext)
}

func (s *Server) handlePutSecrets(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.storeUnavailable(w)
		return
	}
	id, env := r.PathValue("id"), r.PathValue("env")
	ciphertext, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBundleBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "bundle too large")
		return
	}
	b, err := s.store.PutBundle(r.Context(), id, env, ciphertext)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "unknown project")
		return
	}
	if err != nil {
		s.serverError(w, "put bundle", err)
		return
	}
	w.Header().Set("ETag", b.ETag)
	writeJSON(w, http.StatusOK, map[string]any{
		"etag":       b.ETag,
		"updated_at": b.UpdatedAt,
	})
}

func (s *Server) storeUnavailable(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "store not configured")
}

func (s *Server) serverError(w http.ResponseWriter, what string, err error) {
	s.log.Error(what, "err", err)
	writeError(w, http.StatusInternalServerError, what+" failed")
}

// withCommon adds logging and basic headers.
func (s *Server) withCommon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
