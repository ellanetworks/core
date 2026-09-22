// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
)

const (
	DrainAction  = "cluster_member_drain"
	ResumeAction = "cluster_member_resume"
)

type DrainResponse struct {
	DrainState string `json:"drainState"`
}

// DrainClusterMember handles POST /api/v1/cluster/members/{id}/drain.
func DrainClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		actor := getActorFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			DrainAction,
			actor,
			getClientIP(r),
			fmt.Sprintf("Node %s drain", nodeID),
		)

		writeResponse(r.Context(), w, DrainResponse{
			DrainState: db.DrainStateDraining,
		}, http.StatusOK, logger.APILog)
	})
}

// ResumeClusterMember handles POST /api/v1/cluster/members/{id}/resume.
func ResumeClusterMember(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			fmt.Sprintf("Node %s resumed", nodeID),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Cluster member resumed"}, http.StatusOK, logger.APILog)
	})
}

func parseMemberIDPath(r *http.Request) (string, bool) {
	id, err := pki.NormalizeNodeID(r.PathValue("id"))
	if err != nil {
		return "", false
	}

	return id, true
}
