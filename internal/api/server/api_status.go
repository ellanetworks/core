package server

import (
	"net/http"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
)

type ClusterStatusResponse struct {
	Enabled          bool   `json:"enabled"`
	Role             string `json:"role"`
	NodeID           int    `json:"nodeId"`
	IsLeader         bool   `json:"isLeader"`
	LeaderNodeID     int    `json:"leaderNodeId"`
	AppliedIndex     uint64 `json:"appliedIndex"`
	ClusterID        string `json:"clusterId,omitempty"`
	LeaderAPIAddress string `json:"leaderAPIAddress,omitempty"`
}

type StatusResponse struct {
	Version       string                 `json:"version"`
	Revision      string                 `json:"revision"`
	Initialized   bool                   `json:"initialized"`
	Ready         bool                   `json:"ready"`
	SchemaVersion int                    `json:"schemaVersion"`
	Cluster       *ClusterStatusResponse `json:"cluster,omitempty"`
}

func GetStatus(dbInstance *db.Database, ready *atomic.Bool) http.Handler {
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
			Version:       ver.Version,
			Revision:      ver.Revision,
			Initialized:   initialized,
			Ready:         ready.Load(),
			SchemaVersion: db.SchemaVersion(),
		}

		if dbInstance.ClusterEnabled() {
			role := dbInstance.RaftState()
			clusterStatus := &ClusterStatusResponse{
				Enabled:      true,
				Role:         role,
				NodeID:       dbInstance.NodeID(),
				IsLeader:     dbInstance.IsLeader(),
				AppliedIndex: dbInstance.RaftAppliedIndex(),
			}

			op, err := dbInstance.GetOperator(ctx)
			if err == nil && op.ClusterID != "" {
				clusterStatus.ClusterID = op.ClusterID
			}

			clusterStatus.LeaderAPIAddress, clusterStatus.LeaderNodeID = resolveLeader(dbInstance)
			statusResponse.Cluster = clusterStatus

			w.Header().Set("X-Ella-Role", role)
		}

		writeResponse(r.Context(), w, statusResponse, http.StatusOK, logger.APILog)
	})
}
