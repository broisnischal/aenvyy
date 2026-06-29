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
	"log/slog"
	"net/http"
	"time"

	"github.com/nees/envvar/internal/version"
)

// Server holds API dependencies. The store is a placeholder until the SQLite
// backend lands.
type Server struct {
	log *slog.Logger
}

// New constructs a Server.
func New(log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{log: log}
}

// Handler returns the root http.Handler with all routes mounted.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/projects", s.notImplemented("list projects"))
	mux.HandleFunc("POST /v1/projects", s.notImplemented("create project"))
	mux.HandleFunc("GET /v1/projects/{id}/environments/{env}/secrets", s.notImplemented("fetch ciphertext bundle"))
	mux.HandleFunc("PUT /v1/projects/{id}/environments/{env}/secrets", s.notImplemented("store ciphertext bundle"))

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

func (s *Server) notImplemented(what string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": "not implemented yet",
			"route": what,
		})
	}
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
