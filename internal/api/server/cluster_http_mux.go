// Copyright 2026 Ella Networks

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// ClusterSideEffectDeps carries the AMF and BGP services the drain/resume
// side-effect endpoints need. The cluster HTTP mux starts before these
// services exist (discovery needs the cluster port up while Raft forms), so
// the deps are populated by `SetClusterSideEffectDeps` after the public API
// `Upgrade` call. Until then, side-effect endpoints return 503.
type ClusterSideEffectDeps struct {
	AMF *amf.AMF
	BGP *bgp.BGPService
}

var clusterSideEffectDeps atomic.Pointer[ClusterSideEffectDeps]

// SetClusterSideEffectDeps installs the AMF and BGP references used by the
// /cluster/internal/*-side-effects endpoints. Intended to be called once,
// from the same place that wires the full operator API (api.Server.Upgrade).
func SetClusterSideEffectDeps(deps ClusterSideEffectDeps) {
	clusterSideEffectDeps.Store(&deps)
}

func loadClusterSideEffectDeps() (*amf.AMF, *bgp.BGPService, bool) {
	deps := clusterSideEffectDeps.Load()
	if deps == nil {
		return nil, nil, false
	}

	return deps.AMF, deps.BGP, true
}

// maxClusterJoinBodyBytes caps the self-registration POST body. The real
// payload (AddClusterMemberRequest) is a handful of short fields; 4 KiB
// leaves generous headroom without enabling abuse through slow readers.
const maxClusterJoinBodyBytes = 4096

// newClusterMux builds the HTTP mux served on the cluster port.
// Routes here are protected by mTLS (no JWT auth). The cluster port
// exposes only what peers actually need: status probes, self-registration
// at join time, and the /cluster/proxy/ mount that followers use to
// forward authenticated writes to the leader. Destructive cluster-
// membership operations (remove, promote) live on the public API under
// /api/v1/cluster/members/*, gated by JWT + PermManageCluster.
func newClusterMux(dbInstance *db.Database, operatorHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /cluster/status", ClusterStatus(dbInstance).ServeHTTP)
	mux.Handle("POST /cluster/members", selfRegistrationGuard(AddClusterMember(dbInstance)))
	mux.Handle("POST /cluster/members/self", selfRegistrationGuard(SelfAnnounceClusterMember(dbInstance)))
	mux.Handle("POST /cluster/internal/drain-side-effects", removedNodeFence(dbInstance, DrainLocalSideEffects()))
	mux.Handle("POST /cluster/internal/resume-side-effects", removedNodeFence(dbInstance, ResumeLocalSideEffects()))
	mux.Handle("POST /cluster/internal/drain-self", removedNodeFence(dbInstance, DrainSelfOnLeader(dbInstance)))

	if operatorHandler != nil {
		mux.Handle("/cluster/proxy/", removedNodeFence(dbInstance, http.StripPrefix("/cluster/proxy", operatorHandler)))
	}

	return mux
}

// DrainSideEffectsResponse reports which node-local drain side-effects ran.
type DrainSideEffectsResponse struct {
	RANsNotified int  `json:"ransNotified"`
	BGPStopped   bool `json:"bgpStopped"`
}

// DrainLocalSideEffects runs the node-local drain side-effects on the
// receiving node (AMF Status Indication + BGP stop). The leader calls this
// endpoint over mTLS when the drain target is a different node; when the
// target is the leader itself, the leader runs these steps inline without
// the RPC. Returns 503 if dependencies are not yet installed, which happens
// only during the brief window between cluster HTTP startup and the public
// API `Upgrade` call.
func DrainLocalSideEffects() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amfInstance, bgpService, ok := loadClusterSideEffectDeps()
		if !ok {
			writeError(r.Context(), w, http.StatusServiceUnavailable,
				"cluster side-effect dependencies not yet installed", nil, logger.APILog)

			return
		}

		ransNotified := notifyRANsUnavailable(r.Context(), amfInstance, defaultDrainStepTimeout)

		bgpStopped := false

		if bgpService != nil {
			if err := bgpService.Stop(); err != nil {
				logger.APILog.Warn("BGP stop during drain failed", zap.Error(err))
			} else {
				bgpStopped = true
			}
		}

		writeResponse(r.Context(), w, DrainSideEffectsResponse{
			RANsNotified: ransNotified,
			BGPStopped:   bgpStopped,
		}, http.StatusOK, logger.APILog)
	})
}

// ResumeSideEffectsResponse reports which node-local resume side-effects ran.
type ResumeSideEffectsResponse struct {
	BGPStarted bool `json:"bgpStarted"`
}

// ResumeLocalSideEffects restarts the local BGP speaker on the receiving
// node. The leader calls this endpoint over mTLS when the resume target is a
// different node; when the target is the leader itself, the leader runs this
// step inline.
func ResumeLocalSideEffects() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, bgpService, ok := loadClusterSideEffectDeps()
		if !ok {
			writeError(r.Context(), w, http.StatusServiceUnavailable,
				"cluster side-effect dependencies not yet installed", nil, logger.APILog)

			return
		}

		bgpStarted := false

		if bgpService != nil && !bgpService.IsRunning() {
			if err := bgpService.Restart(r.Context()); err != nil {
				logger.APILog.Warn("resume: failed to restart BGP speaker", zap.Error(err))
			} else {
				bgpStarted = true
			}
		}

		writeResponse(r.Context(), w, ResumeSideEffectsResponse{BGPStarted: bgpStarted}, http.StatusOK, logger.APILog)
	})
}

// DrainSelfOnLeader accepts a shutdown-drain request from a peer. Runs only
// on the leader; the calling peer's nodeID is derived from its mTLS client
// certificate, so the caller can only mark itself drained. Used by the
// shutdown path: a node about to exit tells the leader to flip its
// drain_state to "drained" so operators see a clean "active → drained"
// transition instead of the 10s "active → removed-by-autopilot" gap.
func DrainSelfOnLeader(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dbInstance.IsLeader() {
			writeError(r.Context(), w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil, logger.APILog)

			return
		}

		peerID, ok := peerNodeIDFromContext(r.Context())
		if !ok {
			writeError(r.Context(), w, http.StatusForbidden, "peer identity unavailable", nil, logger.APILog)
			return
		}

		if err := dbInstance.SetDrainState(r.Context(), peerID, db.DrainStateDrained); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "cluster member not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError,
				"failed to set drain state", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "drained"}, http.StatusOK, logger.APILog)
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

// SelfAnnounceRequest is the body a node sends to POST /cluster/members/self
// on the leader's cluster port. Every field is self-reported capability data.
type SelfAnnounceRequest struct {
	NodeID           int    `json:"nodeId"`
	RaftAddress      string `json:"raftAddress"`
	APIAddress       string `json:"apiAddress"`
	BinaryVersion    string `json:"binaryVersion"`
	MaxSchemaVersion int    `json:"maxSchemaVersion"`
	Suffrage         string `json:"suffrage,omitempty"`
}

// SelfAnnounceClusterMember handles a node refreshing its own
// cluster_members row. Only the leader can service the request; the
// selfRegistrationGuard wrapper has already validated that the body's
// nodeId matches the peer certificate CN, so the request is authentic.
// Followers return 421 Misdirected Request so the caller can retry
// against the current leader.
func SelfAnnounceClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dbInstance.IsLeader() {
			writeError(r.Context(), w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil, logger.APILog)

			return
		}

		var req SelfAnnounceRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxClusterJoinBodyBytes)).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid request body", err, logger.APILog)
			return
		}

		if req.NodeID <= 0 || req.RaftAddress == "" || req.APIAddress == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "nodeId, raftAddress, apiAddress are required", nil, logger.APILog)
			return
		}

		suffrage := req.Suffrage
		if suffrage == "" {
			suffrage = "voter"
		}

		if suffrage != "voter" && suffrage != "nonvoter" {
			writeError(r.Context(), w, http.StatusBadRequest, `suffrage must be "voter" or "nonvoter"`, nil, logger.APILog)
			return
		}

		// Preserve the suffrage already recorded by the leader: a node
		// self-announcing a conflicting suffrage must not be able to
		// promote itself.
		if existing, err := dbInstance.GetClusterMember(r.Context(), req.NodeID); err == nil && existing != nil && existing.Suffrage != "" {
			suffrage = existing.Suffrage
		}

		member := &db.ClusterMember{
			NodeID:           req.NodeID,
			RaftAddress:      req.RaftAddress,
			APIAddress:       req.APIAddress,
			BinaryVersion:    req.BinaryVersion,
			Suffrage:         suffrage,
			MaxSchemaVersion: req.MaxSchemaVersion,
		}

		if err := dbInstance.UpsertClusterMember(r.Context(), member); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "failed to record cluster member", err, logger.APILog)
			return
		}

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberSelfAnnounceAction,
			getActorFromContext(r),
			getClientIP(r),
			fmt.Sprintf("Self-announced cluster member node %d at %s", req.NodeID, req.RaftAddress),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "self-announce accepted"}, http.StatusOK, logger.APILog)
	})
}
