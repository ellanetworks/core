// Copyright 2026 Ella Networks

// Admin-facing PKI endpoints mounted at /api/v1/cluster/pki/*. All
// require PermManageCluster and every mutation is audit-logged.

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
)

// Audit action strings.
const (
	PKIMintJoinTokenAction = "pki_mint_join_token" // #nosec G101 -- audit action name, not a credential
)

// pkiAdminEndpoint resolves the pkiissuer.Service at request time and
// dispatches to build. Returns 503 until the issuer service has been
// installed by runtime.
func pkiAdminEndpoint(build func(*pkiissuer.Service) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc := loadPKIIssuer()
		if svc == nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable,
				"pki issuer not yet installed", nil, logger.APILog)

			return
		}

		build(svc).ServeHTTP(w, r)
	})
}

// MintJoinTokenRequest is the admin body for POST /api/v1/cluster/pki/join-tokens.
type MintJoinTokenRequest struct {
	NodeID int `json:"nodeID"`

	// TTLSeconds is optional; zero selects the default of 30 minutes.
	TTLSeconds int `json:"ttlSeconds,omitempty"`
}

// MintJoinTokenResponse carries the minted token. The CA fingerprint
// is embedded in the token itself — the joining node extracts it
// unverified to pin its TLS handshake, and the token's HMAC protects
// against tampering.
type MintJoinTokenResponse struct {
	Token             string `json:"token"`
	ExpiresAtUnixSecs int64  `json:"expiresAt"`
}

// PKIMintJoinToken handles POST /api/v1/cluster/pki/join-tokens.
func PKIMintJoinToken(dbInstance *db.Database, svc *pkiissuer.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req MintJoinTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "decode body", err, logger.APILog)
			return
		}

		if req.NodeID < pki.MinNodeID || req.NodeID > pki.MaxNodeID {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("nodeID must be in [%d, %d]", pki.MinNodeID, pki.MaxNodeID), nil, logger.APILog)

			return
		}

		ttl := time.Duration(req.TTLSeconds) * time.Second
		if ttl == 0 {
			ttl = 30 * time.Minute
		}

		token, err := svc.MintJoinToken(r.Context(), req.NodeID, ttl)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "mint token", err, logger.APILog)
			return
		}

		expiresAt := time.Now().Add(ttl).Unix()

		logger.LogAuditEvent(
			r.Context(),
			PKIMintJoinTokenAction,
			getActorFromContext(r),
			getClientIP(r),
			fmt.Sprintf("Minted join token for node %d (ttl=%s)", req.NodeID, ttl),
		)

		writeResponse(r.Context(), w, MintJoinTokenResponse{
			Token:             token,
			ExpiresAtUnixSecs: expiresAt,
		}, http.StatusCreated, logger.APILog)
	})
}
