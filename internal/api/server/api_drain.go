package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const DrainAction = "cluster_drain"

type DrainRequest struct {
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

type DrainResponse struct {
	Message               string `json:"message"`
	TransferredLeadership bool   `json:"transferredLeadership"`
}

func DrainNode(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DrainRequest
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
				return
			}
		}

		transferred := false

		if dbInstance.IsLeader() && dbInstance.ClusterEnabled() {
			if err := dbInstance.LeadershipTransfer(); err != nil {
				logger.APILog.Warn("Leadership transfer failed during drain",
					zap.Error(err),
				)
			} else {
				transferred = true
			}
		}

		email := getEmailFromContext(r)

		logger.LogAuditEvent(
			r.Context(),
			DrainAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Node %d draining, leadership_transferred=%v", dbInstance.NodeID(), transferred),
		)

		writeResponse(r.Context(), w, DrainResponse{
			Message:               "draining",
			TransferredLeadership: transferred,
		}, http.StatusOK, logger.APILog)
	})
}
