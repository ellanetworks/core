// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
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

		state, err := dbInstance.SetDrainStateIf(r.Context(), nodeID,
			[]string{db.DrainStateActive}, db.DrainStateDraining)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", nil, logger.APILog)
				return
			}

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
			fmt.Sprintf("Node %d drain, state=%s", nodeID, state),
		)

		writeResponse(r.Context(), w, DrainResponse{
			DrainState: state,
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

		if _, err := dbInstance.SetDrainStateIf(r.Context(), nodeID,
			[]string{db.DrainStateDraining, db.DrainStateDrained}, db.DrainStateActive); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Cluster member not found", nil, logger.APILog)
				return
			}

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
