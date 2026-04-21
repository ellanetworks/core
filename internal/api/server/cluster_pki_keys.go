// Copyright 2026 Ella Networks

package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// KeyTransferResponse is the wire format returned by GET and accepted by
// POST on /cluster/pki/keys. Both keys are transferred together so a
// partial transfer can never leave a voter with a mismatched pair.
type KeyTransferResponse struct {
	RootKeyPEM         string `json:"rootKeyPEM"`
	IntermediateKeyPEM string `json:"intermediateKeyPEM"`
}

// maxKeyTransferBody caps the accepted upload size. A PEM-encoded ECDSA
// P-256 PKCS#8 key is ~230 bytes; 8 KiB is generous.
const maxKeyTransferBody = 8 * 1024

// ClusterPKIKeysGet handles GET /cluster/pki/keys. mTLS-authenticated
// voter members receive the root + intermediate PKCS#8 PEM bytes so
// they can sign leaves after promotion to leader. Non-voters and
// non-members are rejected.
//
// Security model: the private keys are already on disk in cleartext
// on every voter; handing them to another voter over mTLS does not
// widen the blast radius. Non-voters are rejected because they have
// no path to leadership and therefore no need for the keys.
func ClusterPKIKeysGet(dbInstance *db.Database, svc *pkiissuer.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerNodeID, ok := peerNodeIDFromContext(r.Context())
		if !ok {
			writeError(r.Context(), w, http.StatusUnauthorized, "mTLS required", nil, logger.APILog)
			return
		}

		member, err := dbInstance.GetClusterMember(r.Context(), peerNodeID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusForbidden, "peer is not a cluster member", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "verify membership", err, logger.APILog)

			return
		}

		if member.Suffrage != "voter" {
			writeError(r.Context(), w, http.StatusForbidden, "key transfer only permitted to voter members", nil, logger.APILog)
			return
		}

		rootKeyPEM, intKeyPEM, err := svc.ExportKeys()
		if err != nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "no keys available for transfer", err, logger.APILog)
			return
		}

		logger.APILog.Info("pki key transfer served",
			zap.Int("peerNodeID", peerNodeID))

		resp := KeyTransferResponse{
			RootKeyPEM:         string(rootKeyPEM),
			IntermediateKeyPEM: string(intKeyPEM),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// ClusterPKIKeysPut is currently unused — key transfers are pull-based
// from the receiver (see ClusterPKIKeysGet). The handler is kept as a
// placeholder in case a future push path is needed; for now it responds
// with 405.
func ClusterPKIKeysPut() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, maxKeyTransferBody))
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "key transfer is pull-only", nil, logger.APILog)
	})
}
