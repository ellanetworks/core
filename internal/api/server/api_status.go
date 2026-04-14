package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
)

type ClusterStatusResponse struct {
	Enabled       bool   `json:"enabled"`
	Role          string `json:"role"`
	NodeID        int    `json:"nodeId"`
	AppliedIndex  uint64 `json:"appliedIndex"`
	ClusterID     string `json:"clusterId,omitempty"`
	SchemaVersion int    `json:"schemaVersion"`
}

type StatusResponse struct {
	Version     string                 `json:"version"`
	Revision    string                 `json:"revision"`
	Initialized bool                   `json:"initialized"`
	Cluster     *ClusterStatusResponse `json:"cluster,omitempty"`
}

func GetStatus(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		numUsers, err := dbInstance.CountUsers(ctx)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Unable to retrieve number of users", err, logger.APILog)
			return
		}

		initialized := numUsers > 0

		ver := version.GetVersion()

		statusResponse := StatusResponse{
			Version:     ver.Version,
			Revision:    ver.Revision,
			Initialized: initialized,
		}

		if dbInstance.ClusterEnabled() {
			clusterStatus := &ClusterStatusResponse{
				Enabled:       true,
				Role:          dbInstance.RaftState(),
				NodeID:        dbInstance.NodeID(),
				AppliedIndex:  dbInstance.RaftAppliedIndex(),
				SchemaVersion: db.SchemaVersion(),
			}

			op, err := dbInstance.GetOperator(ctx)
			if err == nil && op.ClusterID != "" {
				clusterStatus.ClusterID = op.ClusterID
			}

			statusResponse.Cluster = clusterStatus
		}

		writeResponse(r.Context(), w, statusResponse, http.StatusOK, logger.APILog)
	})
}
