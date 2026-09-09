// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/raft"
	"go.uber.org/zap"
)

// pkiIssuerService is the global issuer handle used by /cluster/pki/*
// handlers on the cluster HTTP port. Set by SetPKIIssuer after the
// issuer service is instantiated in runtime.go.
var pkiIssuerService atomic.Pointer[pkiissuer.Service]

// SetPKIIssuer installs the issuer service used by the cluster-port
// PKI handlers. Safe to call before or after StartClusterHTTP.
func SetPKIIssuer(svc *pkiissuer.Service) {
	pkiIssuerService.Store(svc)
}

func loadPKIIssuer() *pkiissuer.Service {
	return pkiIssuerService.Load()
}

// maxClusterJoinBodyBytes caps the self-registration POST body. The real
// payload (AddClusterMemberRequest) is a handful of short fields; 4 KiB
// leaves generous headroom without enabling abuse through slow readers.
const maxClusterJoinBodyBytes = 4096

// newClusterMux builds the HTTP mux served on the cluster port.
// Routes here are protected by mTLS (no JWT auth). The cluster port
// exposes only what peers actually need: status probes, self-
// registration at join time, the typed propose-forward endpoint that
// in-process write callers use to commit through the current leader,
// and a small set of leader-only or node-targeted RPCs (autopilot
// state, side-effect hooks). Destructive cluster-membership operations
// (remove, promote) live on the public API under
// /api/v1/cluster/members/*, gated by JWT + PermManageCluster.
func newClusterMux(dbInstance *db.Database) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /cluster/status", ClusterStatus(dbInstance).ServeHTTP)
	mux.Handle("POST /cluster/members", selfRegistrationGuard(AddClusterMember(dbInstance)))
	mux.Handle("GET "+InternalAutopilotPath, removedNodeFence(dbInstance, ClusterAutopilotState(dbInstance)))
	mux.Handle("POST "+raft.ProposeForwardPath, removedNodeFence(dbInstance, ClusterPropose(dbInstance)))

	// PKI register endpoint on the cluster HTTP ALPN. The issuer
	// service becomes available after the first leader election;
	// pkiEndpoint resolves it at request time and 503s before then.
	mux.Handle("POST /cluster/pki/register", pkiEndpoint(func(svc *pkiissuer.Service) http.Handler {
		return ClusterPKIRegister(svc)
	}))

	return mux
}

// pkiEndpoint resolves the current pkiissuer.Service at request time
// and dispatches. Returns 503 until the service is installed and ready.
func pkiEndpoint(build func(*pkiissuer.Service) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc := loadPKIIssuer()
		if svc == nil || !svc.Ready(r.Context()) {
			writeError(r.Context(), w, http.StatusServiceUnavailable,
				"pki issuer not yet ready", nil, logger.APILog)

			return
		}

		build(svc).ServeHTTP(w, r)
	})
}

// removedNodeFence rejects proxied writes from peers whose nodeID is no
// longer present in cluster_members. Membership is the authoritative ACL:
// a node removed via RemoveClusterMember must not continue pushing writes
// through the proxy path, even if its mTLS cert is still valid (cert
// revocation lag is a real operational window). Returns 410 Gone so the
// client can surface the condition distinctly from 401/403/503.
func removedNodeFence(dbInstance *db.Database, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerID, ok := peerNodeIDFromContext(r.Context())
		if !ok {
			writeError(r.Context(), w, http.StatusForbidden, "peer identity unavailable", nil, logger.APILog)
			return
		}

		_, err := dbInstance.GetClusterMember(r.Context(), peerID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				logger.APILog.Warn("proxy: rejected write from removed cluster member",
					zap.Int("peerNodeId", peerID),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path))
				writeError(r.Context(), w, http.StatusGone,
					fmt.Sprintf("node-id %d is not a current cluster member", peerID), nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError,
				"failed to verify cluster membership", err, logger.APILog)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// selfRegistrationGuard restricts POST /cluster/members on the cluster
// port to self-registration: the body's nodeId must match the node-id
// encoded in the peer certificate's CN. This blocks a compromised peer
// cert from being used to register a node-id it was not issued for.
// Operator-initiated adds use the public API, which does not pass
// through this guard.
func selfRegistrationGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerID, ok := peerNodeIDFromContext(r.Context())
		if !ok {
			writeError(r.Context(), w, http.StatusForbidden, "peer identity unavailable", nil, logger.APILog)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxClusterJoinBodyBytes))
		_ = r.Body.Close()

		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "failed to read request body", err, logger.APILog)
			return
		}

		var probe struct {
			NodeID int `json:"nodeId"`
		}

		if err := json.Unmarshal(body, &probe); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid request body", err, logger.APILog)
			return
		}

		if probe.NodeID != peerID {
			writeError(r.Context(), w, http.StatusForbidden,
				fmt.Sprintf("nodeId %d does not match peer certificate CN (node-id %d)", probe.NodeID, peerID),
				nil, logger.APILog)

			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))

		next.ServeHTTP(w, r)
	})
}

type clusterNodeStatus struct {
	Role          string `json:"role"`
	NodeID        int    `json:"nodeId"`
	ClusterID     string `json:"clusterId,omitempty"`
	SchemaVersion int    `json:"schemaVersion"`
	AppliedSchema int    `json:"appliedSchema,omitempty"`
	PendingSchema int    `json:"pendingSchema,omitempty"`
}

type clusterStatusResponse struct {
	Cluster clusterNodeStatus `json:"cluster"`
}

// ClusterStatus returns the node's Raft role, ID, cluster ID, and
// schema version. Used by peers during discovery and health checks.
func ClusterStatus(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := clusterNodeStatus{
			Role:          dbInstance.RaftState(),
			NodeID:        dbInstance.NodeID(),
			SchemaVersion: db.SchemaVersion(),
		}

		op, err := dbInstance.GetOperator(r.Context())
		if err == nil && op.ClusterID != "" {
			status.ClusterID = op.ClusterID
		}

		if applied, err := dbInstance.CurrentSchemaVersion(r.Context()); err == nil {
			status.AppliedSchema = applied
			if db.SchemaVersion() > applied {
				status.PendingSchema = db.SchemaVersion()
			}
		}

		writeResponse(r.Context(), w, clusterStatusResponse{Cluster: status}, http.StatusOK, logger.APILog)
	})
}
