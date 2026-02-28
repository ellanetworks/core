package server

import (
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

// fleetReadOnlyExemptPrefixes lists URL path prefixes that are allowed
// even when the Core is managed by Fleet. These cover authentication,
// user management, fleet operations, backup, and observability endpoints.
var fleetReadOnlyExemptPrefixes = []string{
	"/api/v1/auth/",
	"/api/v1/init",
	"/api/v1/status",
	"/api/v1/metrics",
	"/api/v1/users",
	"/api/v1/fleet/",
	"/api/v1/backup",
	"/api/v1/logs/",
	"/api/v1/pprof/",
}

// FleetReadOnlyMiddleware rejects mutating API requests (POST, PUT, DELETE)
// when the Core is registered to a Fleet. Read-only (GET, HEAD, OPTIONS)
// requests and exempt paths always pass through.
func FleetReadOnlyMiddleware(dbInstance *db.Database, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all read-only methods unconditionally
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Allow non-API requests (e.g. frontend assets)
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow exempt paths
		for _, prefix := range fleetReadOnlyExemptPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check if fleet-managed
		managed, err := dbInstance.IsFleetManaged(r.Context())
		if err != nil {
			logger.APILog.Warn("couldn't check fleet status for read-only guard")
			// On error, allow the request through to avoid blocking
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
