package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/ui"
)

// newFrontendFileServer uses the embedded ui output files as the base for a file server
func newFrontendFileServer() (http.Handler, error) {
	frontendFS, err := fs.Sub(ui.FrontendFS, "out")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(frontendFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasSuffix(path, "/") && !strings.Contains(path, ".") {
			path += ".html"
		}
		r.URL.Path = path
		fileServer.ServeHTTP(w, r)
	}), nil
}
