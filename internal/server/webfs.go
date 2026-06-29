package server

import (
	"embed"
	"io/fs"
	"net/http"
)

// dist holds the built frontend. The web app (web/) builds to web/dist, which
// the Makefile copies into internal/server/dist so it embeds into the single
// Go binary. A committed placeholder keeps the build green before a web build.
//
//go:embed all:dist
var dist embed.FS

// shellFile is the SPA entry produced by TanStack Start's SPA-mode build.
const shellFile = "_shell.html"

// webHandler serves the embedded SPA, falling back to the shell for client-side
// routes (history API) and unknown asset paths.
func webHandler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := trimLeadingSlash(r.URL.Path)
		if clean == "" || serveShell(sub, clean) {
			serveFile(sub, w, r, shellFile)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// serveShell reports whether the request should fall back to the SPA shell
// (path has no matching embedded asset).
func serveShell(sub fs.FS, clean string) bool {
	_, err := fs.Stat(sub, clean)
	return err != nil
}

func serveFile(sub fs.FS, w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(sub, name)
	if err != nil {
		// No SPA build embedded (fresh checkout without `make web`). Serve a
		// friendly placeholder rather than 404 so the API is still usable.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(placeholderHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

const placeholderHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
	`<title>envvar</title></head><body style="font-family:system-ui;max-width:40rem;margin:4rem auto;padding:0 1rem">` +
	`<h1>envvar</h1><p>API is running. Build the web UI with <code>make web</code> to embed it.</p>` +
	`<p><a href="/healthz">/healthz</a></p></body></html>`

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}
