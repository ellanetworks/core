package server

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func newFrontendFileServer(embedFS fs.FS) (http.Handler, error) {
	uiFS, err := fs.Sub(embedFS, "dist")
	if err != nil {
		return nil, err
	}

	fsHandler := http.FileServer(http.FS(uiFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve static assets (JS, CSS, images, etc.) directly.
		if strings.HasPrefix(r.URL.Path, "/assets/") || strings.Contains(r.URL.Path, ".") {
			serveFileServer(w, r, fsHandler, r.URL.Path)
			return
		}

		// SPA fallback: serve index.html for all other routes.
		if fileExists(uiFS, "/index.html") {
			serveBytes(w, uiFS, "/index.html")
			return
		}

		http.NotFound(w, r)
	}), nil
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

	defer func() {
		err := f.Close()
		if err != nil {
			logger.EllaLog.Error("could not close file", zap.String("path", p), zap.Error(err))
		}
	}()

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
