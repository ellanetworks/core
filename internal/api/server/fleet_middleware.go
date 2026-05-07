// Copyright 2026 Ella Networks

package server

import (
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

// fleetReadOnlyExemptPrefixes are URL path prefixes permitted even when
// the Core is Fleet-managed. Covers auth, observability, backup, cluster
// admin, and the fleet endpoints themselves.
var fleetReadOnlyExemptPrefixes = []string{
	"/api/v1/auth/",
	"/api/v1/init",
	"/api/v1/status",
	"/api/v1/metrics",
	"/api/v1/openapi.yaml",
	"/api/v1/users",
	"/api/v1/fleet/",
	"/api/v1/backup",
	"/api/v1/restore",
	"/api/v1/support-bundle",
	"/api/v1/logs/",
	"/api/v1/pprof/",
	"/api/v1/cluster/",
}

// FleetReadOnlyMiddleware rejects mutating API requests (POST, PUT,
// DELETE, PATCH) while the Core is registered to a Fleet. Reads and
// exempt paths always pass through.
func FleetReadOnlyMiddleware(dbInstance *db.Database, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		for _, prefix := range fleetReadOnlyExemptPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		managed, err := dbInstance.IsFleetManaged(r.Context())
		if err != nil {
			logger.APILog.Warn("couldn't check fleet status for read-only guard")
			next.ServeHTTP(w, r)

			return
		}

		if managed {
			writeError(r.Context(), w, http.StatusForbidden, "This Core is managed by Fleet. Changes must be made through Fleet.", nil, logger.APILog)
			return
		}

		next.ServeHTTP(w, r)
	})
}
