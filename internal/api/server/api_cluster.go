package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	ClusterMemberAddAction          = "cluster_member_add"
	ClusterMemberRemoveAction       = "cluster_member_remove"
	ClusterMemberSelfAnnounceAction = "cluster_member_self_announce"
)

type ClusterMemberResponse struct {
	NodeID           int    `json:"nodeId"`
	RaftAddress      string `json:"raftAddress"`
	APIAddress       string `json:"apiAddress"`
	BinaryVersion    string `json:"binaryVersion"`
	Suffrage         string `json:"suffrage"`
	MaxSchemaVersion int    `json:"maxSchemaVersion"`
	IsLeader         bool   `json:"isLeader"`
	DrainState       string `json:"drainState"`
	DrainUpdatedAt   string `json:"drainUpdatedAt,omitempty"`
}

func toClusterMemberResponse(m db.ClusterMember, leaderAddr string) ClusterMemberResponse {
	state := m.DrainState
	if state == "" {
		state = db.DrainStateActive
	}

	updated := ""
	if m.DrainUpdatedAt > 0 {
		updated = time.Unix(m.DrainUpdatedAt, 0).UTC().Format(time.RFC3339)
	}

	return ClusterMemberResponse{
		NodeID:           m.NodeID,
		RaftAddress:      m.RaftAddress,
		APIAddress:       m.APIAddress,
		BinaryVersion:    m.BinaryVersion,
		Suffrage:         m.Suffrage,
		MaxSchemaVersion: m.MaxSchemaVersion,
		IsLeader:         leaderAddr != "" && m.RaftAddress == leaderAddr,
		DrainState:       state,
		DrainUpdatedAt:   updated,
	}
}

type AddClusterMemberRequest struct {
	NodeID           int    `json:"nodeId"`
	RaftAddress      string `json:"raftAddress"`
	APIAddress       string `json:"apiAddress"`
	ClusterID        string `json:"clusterId,omitempty"`
	SchemaVersion    int    `json:"schemaVersion,omitempty"`
	MaxSchemaVersion int    `json:"maxSchemaVersion,omitempty"`
	Suffrage         string `json:"suffrage,omitempty"`
}

func GetClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeIDStr := r.PathValue("id")

		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", err, logger.APILog)
			return
		}

		member, err := dbInstance.GetClusterMember(r.Context(), nodeID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to look up cluster member", err, logger.APILog)

			return
		}

		leaderAddr := dbInstance.LeaderAddress()

		writeResponse(r.Context(), w, toClusterMemberResponse(*member, leaderAddr), http.StatusOK, logger.APILog)
	})
}

func ListClusterMembers(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		members, err := dbInstance.ListClusterMembers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list cluster members", err, logger.APILog)
			return
		}

		leaderAddr := dbInstance.LeaderAddress()

		result := make([]ClusterMemberResponse, 0, len(members))
		for _, m := range members {
			result = append(result, toClusterMemberResponse(m, leaderAddr))
		}

		writeResponse(r.Context(), w, result, http.StatusOK, logger.APILog)
	})
}

func AddClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req AddClusterMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if req.NodeID <= 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "nodeId must be a positive integer", nil, logger.APILog)
			return
		}

		if req.RaftAddress == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "raftAddress is required", nil, logger.APILog)
			return
		}

		if req.APIAddress == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "apiAddress is required", nil, logger.APILog)
			return
		}

		// Zero means "not provided" by convention; negative is always a
		// client bug and must not pass through silently.
		if req.SchemaVersion < 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "schemaVersion must be non-negative", nil, logger.APILog)
			return
		}

		if req.MaxSchemaVersion < 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "maxSchemaVersion must be non-negative", nil, logger.APILog)
			return
		}

		// Default suffrage to "voter" for backward compatibility.
		suffrage := req.Suffrage
		if suffrage == "" {
			suffrage = "voter"
		}

		if suffrage != "voter" && suffrage != "nonvoter" {
			writeError(r.Context(), w, http.StatusBadRequest,
				"suffrage must be \"voter\" or \"nonvoter\"", nil, logger.APILog)

			return
		}

		if req.ClusterID != "" {
			op, err := dbInstance.GetOperator(r.Context())
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to read operator", err, logger.APILog)
				return
			}

			if op.ClusterID != "" && req.ClusterID != op.ClusterID {
				writeError(r.Context(), w, http.StatusConflict, "Cluster ID mismatch: node belongs to a different cluster", nil, logger.APILog)
				return
			}
		}

		// Schema handshake (leader side): reject joiners whose schema is
		// older than the leader's — they would miss migrations already
		// applied. Newer joiners are accepted; the leader will eventually
		// propose its own migrations through Raft. The follower-side
		// counterpart lives in discovery.go:discoveryTick.
		if req.SchemaVersion > 0 {
			local := db.SchemaVersion()
			if req.SchemaVersion < local {
				writeError(r.Context(), w, http.StatusConflict,
					fmt.Sprintf("Schema version mismatch: node has %d, cluster has %d", req.SchemaVersion, local),
					nil, logger.APILog)

				return
			}
		}

		if suffrage == "nonvoter" {
			if err := dbInstance.AddNonvoter(req.NodeID, req.RaftAddress); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to add nonvoter to Raft cluster", err, logger.APILog)
				return
			}
		} else {
			if err := dbInstance.AddVoter(req.NodeID, req.RaftAddress); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to add voter to Raft cluster", err, logger.APILog)
				return
			}
		}

		member := &db.ClusterMember{
			NodeID:           req.NodeID,
			RaftAddress:      req.RaftAddress,
			APIAddress:       req.APIAddress,
			BinaryVersion:    "", // populated by the joining node's self-announce
			Suffrage:         suffrage,
			MaxSchemaVersion: req.MaxSchemaVersion,
		}

		if err := dbInstance.UpsertClusterMember(r.Context(), member); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to register cluster member", err, logger.APILog)
			return
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberAddAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Added cluster member node %d at %s (suffrage: %s)", req.NodeID, req.RaftAddress, suffrage),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member added"}, http.StatusCreated, logger.APILog)
	})
}

func RemoveClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeIDStr := r.PathValue("id")

		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", err, logger.APILog)
			return
		}

		member, err := dbInstance.GetClusterMember(r.Context(), nodeID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to look up cluster member", err, logger.APILog)

			return
		}

		// Refuse to remove the current leader. The operator must drain and
		// transfer leadership first; otherwise we disrupt writes and risk
		// proxied callers losing the session mid-remove.
		if leaderAddr := dbInstance.LeaderAddress(); leaderAddr != "" && member.RaftAddress == leaderAddr {
			writeError(r.Context(), w, http.StatusConflict,
				"Cannot remove the current leader; drain this node first so leadership transfers, then retry",
				nil, logger.APILog)

			return
		}

		// Drain precondition: refuse removal unless the node has been
		// drained or the caller explicitly opts into force-remove.
		// force=true skips the drain check but not the leader check.
		force := r.URL.Query().Get("force") == "true"
		if !force && member.DrainState != db.DrainStateDrained {
			writeError(r.Context(), w, http.StatusConflict,
				fmt.Sprintf("Node is not drained (state=%s); drain it first via POST /api/v1/cluster/members/%d/drain, or pass ?force=true to skip",
					member.DrainState, nodeID),
				nil, logger.APILog)

			return
		}

		if err := dbInstance.RemoveServer(nodeID); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to remove server from Raft cluster", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteClusterMember(r.Context(), nodeID); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to remove cluster member record", err, logger.APILog)
			return
		}

		// Purge the removed node's dynamic IP leases so the addresses
		// they occupy are returned to the cluster-wide pool. Static
		// leases are preserved by the underlying DELETE (it filters
		// by type='dynamic'). Failure here is logged but non-fatal:
		// the membership change has already succeeded and an operator
		// can re-run cleanup later via a direct DB operation if needed.
		if err := dbInstance.DeleteDynamicLeasesByNode(r.Context(), nodeID); err != nil {
			logger.APILog.Warn("Failed to purge dynamic IP leases for removed cluster member; leases will linger until manually cleaned",
				zap.Int("nodeId", nodeID), zap.Error(err))
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberRemoveAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Removed cluster member node %d", nodeID),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member removed"}, http.StatusOK, logger.APILog)
	})
}

const ClusterMemberPromoteAction = "cluster_member_promote"

func PromoteClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nodeIDStr := r.PathValue("id")

		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", err, logger.APILog)
			return
		}

		member, err := dbInstance.GetClusterMember(r.Context(), nodeID)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", err, logger.APILog)
			return
		}

		if member.Suffrage == "voter" {
			writeError(r.Context(), w, http.StatusConflict, "Node is already a voter", nil, logger.APILog)
			return
		}

		if err := dbInstance.AddVoter(nodeID, member.RaftAddress); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to promote node to voter", err, logger.APILog)
			return
		}

		member.Suffrage = "voter"

		if err := dbInstance.UpsertClusterMember(r.Context(), member); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update cluster member record", err, logger.APILog)
			return
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberPromoteAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Promoted cluster member node %d to voter", nodeID),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member promoted to voter"}, http.StatusOK, logger.APILog)
	})
}
