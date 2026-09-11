// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"go.uber.org/zap"
)

const (
	DrainAction  = "cluster_member_drain"
	ResumeAction = "cluster_member_resume"
)

type DrainResponse struct {
	DrainState string `json:"drainState"`
}

// DrainClusterMember handles POST /api/v1/cluster/members/{id}/drain.
//
// Runs on the Raft leader (followers forward).
func DrainClusterMember(dbInstance *db.Database, amfInstance *amf.AMF, mmeInstance *mme.MME, bgpService *bgp.BGPService, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dbInstance.ClusterEnabled() && !dbInstance.IsLeader() {
			writeError(r.Context(), w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil, logger.APILog)

			return
		}

		nodeID, ok := parseMemberIDPath(r)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", nil, logger.APILog)
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

		if member.DrainState == db.DrainStateDraining || member.DrainState == db.DrainStateDrained {
			writeResponse(r.Context(), w, DrainResponse{
				DrainState: member.DrainState,
			}, http.StatusOK, logger.APILog)

			return
		}

		if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateDraining); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError,
				"Failed to persist drain state", err, logger.APILog)

			return
		}

		// Leadership transfer is the last local step: transferring earlier
		// would strand subsequent replicated writes on a now-follower.
		transferred := false

		if nodeID == dbInstance.NodeID() && dbInstance.ClusterEnabled() && dbInstance.IsLeader() {
			if err := dbInstance.LeadershipTransfer(); err != nil {
				logger.APILog.Warn("leadership transfer failed during self-drain; drain state is already draining",
					zap.Error(err))
			} else {
				transferred = true
			}
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			DrainAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Node %d drain, leadership_transferred=%v", nodeID, transferred),
		)

		writeResponse(r.Context(), w, DrainResponse{
			DrainState: db.DrainStateDraining,
		}, http.StatusOK, logger.APILog)
	})
}

// ResumeClusterMember handles POST /api/v1/cluster/members/{id}/resume.
//
// Runs on the leader.
func ResumeClusterMember(dbInstance *db.Database, mmeInstance *mme.MME, bgpService *bgp.BGPService, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dbInstance.ClusterEnabled() && !dbInstance.IsLeader() {
			writeError(r.Context(), w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil, logger.APILog)

			return
		}

		nodeID, ok := parseMemberIDPath(r)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid node ID", nil, logger.APILog)
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

		if member.DrainState == db.DrainStateActive {
			writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member resumed"}, http.StatusOK, logger.APILog)
			return
		}

		if err := dbInstance.SetDrainState(r.Context(), nodeID, db.DrainStateActive); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError,
				"Failed to clear drain state", err, logger.APILog)

			return
		}

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			ResumeAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Node %d resumed", nodeID),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member resumed"}, http.StatusOK, logger.APILog)
	})
}

func parseMemberIDPath(r *http.Request) (int, bool) {
	idStr := r.PathValue("id")
	if idStr == "" {
		return 0, false
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}
