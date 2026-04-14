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
	NodeID      int    `json:"nodeId"`
	RaftAddress string `json:"raftAddress"`
	APIAddress  string `json:"apiAddress"`
}

type AddClusterMemberRequest struct {
	NodeID      int    `json:"nodeId"`
	RaftAddress string `json:"raftAddress"`
	APIAddress  string `json:"apiAddress"`
	ClusterID   string `json:"clusterId,omitempty"`
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
				NodeID:      m.NodeID,
				RaftAddress: m.RaftAddress,
				APIAddress:  m.APIAddress,
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

		if err := dbInstance.AddVoter(req.NodeID, req.RaftAddress); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to add voter to Raft cluster", err, logger.APILog)
			return
		}

		member := &db.ClusterMember{
			NodeID:      req.NodeID,
			RaftAddress: req.RaftAddress,
			APIAddress:  req.APIAddress,
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
			fmt.Sprintf("Added cluster member node %d at %s", req.NodeID, req.RaftAddress),
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
