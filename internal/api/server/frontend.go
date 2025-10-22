package server

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

func newFrontendFileServer(embedFS fs.FS) (http.Handler, error) {
	uiFS, err := fs.Sub(embedFS, "out")
	if err != nil {
		return nil, err
	}

	fsHandler := http.FileServer(http.FS(uiFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve Next assets & any dotted path verbatim.
		if strings.HasPrefix(r.URL.Path, "/_next/") || (strings.Contains(r.URL.Path, ".")) {
			serveFileServer(w, r, fsHandler, r.URL.Path)
			return
		}

		// React Flight (RSC) requests → serve exported *.txt payloads.
		if isFlightRequest(r) {
			for _, candidate := range flightCandidates(r.URL.Path) {
				if fileExists(uiFS, candidate) {
					serveBytes(w, uiFS, candidate)
					return
				}
			}
			http.NotFound(w, r)
			return
		}

		// Root → always serve /index.html directly (avoid FileServer redirect loop).
		if r.URL.Path == "/" {
			if fileExists(uiFS, "/index.html") {
				serveBytes(w, uiFS, "/index.html")
				return
			}
			http.NotFound(w, r)
			return
		}

		// Other routes: try concrete shapes, no redirects.
		for _, candidate := range htmlCandidates(r.URL.Path) {
			if fileExists(uiFS, candidate) {
				serveFileServer(w, r, fsHandler, candidate)
				return
			}
		}

		http.NotFound(w, r)
	}), nil
}

func isFlightRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/x-component") || r.URL.Query().Has("__flight__")
}

func flightCandidates(p string) []string {
	// Try both export layouts for safety.
	if p == "/" {
		return []string{"/index.txt"}
	}
	base := strings.TrimSuffix(p, "/")
	return []string{base + ".txt", base + "/index.txt"}
}

func htmlCandidates(p string) []string {
	base := strings.TrimSuffix(p, "/")
	return []string{base + ".html", base + "/index.html"}
}

func fileExists(fsys fs.FS, p string) bool {
	_, err := fs.Stat(fsys, strings.TrimPrefix(path.Clean(p), "/"))
	return err == nil
}

func serveFileServer(w http.ResponseWriter, r *http.Request, fsHandler http.Handler, p string) {
	r2 := *r
	if r.URL != nil {
		u := *r.URL
		r2.URL = &u
	}
	r2.URL.Path = p
	fsHandler.ServeHTTP(w, &r2)
}

func serveBytes(w http.ResponseWriter, fsys fs.FS, p string) {
	f, err := fsys.Open(strings.TrimPrefix(p, "/"))
	if err != nil {
		http.NotFound(w, nil)
		return
	}

	defer f.Close()

	ct := mime.TypeByExtension(filepath.Ext(p))
	if ct == "" {
		switch {
		case strings.HasSuffix(p, ".html"):
			ct = "text/html; charset=utf-8"
		case strings.HasSuffix(p, ".txt"):
			ct = "text/plain; charset=utf-8" // Flight payload
		default:
			ct = "application/octet-stream"
		}
	}

	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}
