package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	ClusterMemberAddAction    = "cluster_member_add"
	ClusterMemberRemoveAction = "cluster_member_remove"
)

type ClusterMemberResponse struct {
	NodeID        int    `json:"nodeId"`
	RaftAddress   string `json:"raftAddress"`
	APIAddress    string `json:"apiAddress"`
	BinaryVersion string `json:"binaryVersion"`
	Suffrage      string `json:"suffrage"`
}

type AddClusterMemberRequest struct {
	NodeID        int    `json:"nodeId"`
	RaftAddress   string `json:"raftAddress"`
	APIAddress    string `json:"apiAddress"`
	ClusterID     string `json:"clusterId,omitempty"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
	Suffrage      string `json:"suffrage,omitempty"`
}

func ListClusterMembers(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		members, err := dbInstance.ListClusterMembers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list cluster members", err, logger.APILog)
			return
		}

		result := make([]ClusterMemberResponse, 0, len(members))
		for _, m := range members {
			result = append(result, ClusterMemberResponse{
				NodeID:        m.NodeID,
				RaftAddress:   m.RaftAddress,
				APIAddress:    m.APIAddress,
				BinaryVersion: m.BinaryVersion,
				Suffrage:      m.Suffrage,
			})
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
			NodeID:        req.NodeID,
			RaftAddress:   req.RaftAddress,
			APIAddress:    req.APIAddress,
			BinaryVersion: "", // populated by the joining node during startup registration
			Suffrage:      suffrage,
		}

		if err := dbInstance.UpsertClusterMember(r.Context(), member); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to register cluster member", err, logger.APILog)
			return
		}

		email := getEmailFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberAddAction,
			email,
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

		if err := dbInstance.RemoveServer(nodeID); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to remove server from Raft cluster", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteClusterMember(r.Context(), nodeID); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to remove cluster member record", err, logger.APILog)
			return
		}

		email := getEmailFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberRemoveAction,
			email,
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

		email := getEmailFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ClusterMemberPromoteAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Promoted cluster member node %d to voter", nodeID),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member promoted to voter"}, http.StatusOK, logger.APILog)
	})
}
