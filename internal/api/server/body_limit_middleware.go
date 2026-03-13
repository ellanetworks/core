// Copyright 2026 Ella Networks

package server

import (
	"net/http"
)

// DefaultMaxBodySize is the maximum request body size for most API endpoints (1 MB).
const DefaultMaxBodySize = 1 << 20

// MaxBodySizeMiddleware limits the size of incoming request bodies using
// http.MaxBytesReader. Most endpoints are limited to DefaultMaxBodySize.
// The restore endpoint is exempt because database backups have no
// predictable upper bound and the endpoint already requires admin auth.
func MaxBodySizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/restore" {
			r.Body = http.MaxBytesReader(w, r.Body, DefaultMaxBodySize)
		}

		next.ServeHTTP(w, r)
	})
}
