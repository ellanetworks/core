// Copyright 2026 Ella Networks

package server

import (
	"crypto/subtle"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const headerClusterToken = "X-Ella-Cluster-Token" // #nosec: G101 -- HTTP header name, not a credential

// ClusterTokenOrAuth accepts either a valid cluster join token (via the
// X-Ella-Cluster-Token header) or standard JWT/API-token authentication
// with admin authorization. If the join token matches, the request bypasses
// the normal auth chain.
func ClusterTokenOrAuth(joinToken string, jwtSecret *JWTSecret, dbInstance *db.Database, next http.Handler) http.Handler {
	authFallback := Authenticate(jwtSecret, dbInstance,
		Authorize(PermManageCluster, next))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(headerClusterToken)
		if token != "" && joinToken != "" {
			if subtle.ConstantTimeCompare([]byte(token), []byte(joinToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}

			writeError(r.Context(), w, http.StatusForbidden, "Invalid cluster join token", nil, logger.APILog)

			return
		}

		authFallback.ServeHTTP(w, r)
	})
}
