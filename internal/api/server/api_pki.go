// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Admin-facing PKI endpoints mounted at /api/v1/cluster/pki/*. All
// require PermManageCluster and every mutation is audit-logged.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
)

const (
	PKIMintJoinTokenAction = "pki_mint_join_token" // #nosec G101 -- audit action name
)

// A leader that has just been promoted seeds the join-HMAC key and
// registers its own cluster certificate as part of leader init;
// minting before those commits land would fail or embed a stale pin,
// so the handler waits it out rather than failing a request that is
// about to become serviceable.
var (
	mintReadyWait = 15 * time.Second
	mintReadyPoll = 100 * time.Millisecond
)

// pkiAdminEndpoint resolves the pkiissuer.Service at request time and
// dispatches to build. Returns 503 until the issuer service has been
// installed by runtime.
func pkiAdminEndpoint(build func(*pkiissuer.Service) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc := loadPKIIssuer()
		if svc == nil {
			w.Header().Set("Retry-After", "1")
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

// MintJoinTokenResponse carries the minted token. The leader's
// pinned cert fingerprint is embedded in the token; the joining
// node extracts it unverified to pin the bootstrap TLS handshake,
// and the token's HMAC protects against tampering.
type MintJoinTokenResponse struct {
	Token             string `json:"token"`
	ExpiresAtUnixSecs int64  `json:"expiresAt"`
}

// PKIMintJoinToken handles POST /api/v1/cluster/pki/join-tokens.
// Minting is leader-only and is not forwarded: a request that lands
// on a follower fails.
func PKIMintJoinToken(svc *pkiissuer.Service) http.Handler {
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

		token, err := mintWhenReady(r.Context(), svc, req.NodeID, ttl)
		if err != nil {
			if errors.Is(err, pkiissuer.ErrNotReady) {
				w.Header().Set("Retry-After", "1")
				writeError(r.Context(), w, http.StatusServiceUnavailable, "mint token", err, logger.APILog)

				return
			}

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

// mintWhenReady retries MintJoinToken while the issuer reports
// ErrNotReady, up to mintReadyWait.
func mintWhenReady(ctx context.Context, svc *pkiissuer.Service, nodeID int, ttl time.Duration) (string, error) {
	deadline := time.Now().Add(mintReadyWait)

	for {
		token, err := svc.MintJoinToken(ctx, nodeID, ttl)
		if !errors.Is(err, pkiissuer.ErrNotReady) || time.Now().After(deadline) {
			return token, err
		}

		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(mintReadyPoll):
		}
	}
}
