package server

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

// resolveLeader looks up the current leader in the cluster_members
// table and returns the leader's API address and node-id. Either field
// is zero when no leader is known or the leader's row is not yet
// present.
func resolveLeader(dbInstance *db.Database) (apiAddress string, nodeID int) {
	raftAddr := dbInstance.LeaderAddress()
	if raftAddr == "" {
		return "", 0
	}

	members, err := dbInstance.ListClusterMembers(context.Background())
	if err != nil {
		return "", 0
	}

	for _, m := range members {
		if m.RaftAddress == raftAddr {
			return m.APIAddress, m.NodeID
		}
	}

	return "", 0
}

// PendingMigrationResponse is non-nil only during a rolling-upgrade
// window. Surfaced under cluster.pendingMigration.
type PendingMigrationResponse struct {
	CurrentSchema int `json:"currentSchema"`
	TargetSchema  int `json:"targetSchema"`
	LaggardNodeId int `json:"laggardNodeId,omitempty"`
}

type ClusterStatusResponse struct {
	Enabled          bool   `json:"enabled"`
	Role             string `json:"role"`
	NodeID           int    `json:"nodeId"`
	IsLeader         bool   `json:"isLeader"`
	LeaderNodeID     int    `json:"leaderNodeId"`
	AppliedIndex     uint64 `json:"appliedIndex"`
	ClusterID        string `json:"clusterId,omitempty"`
	LeaderAPIAddress string `json:"leaderAPIAddress,omitempty"`

	// AppliedSchemaVersion is what the cluster has committed; the
	// top-level SchemaVersion is what this binary supports. They
	// differ only during a rolling upgrade.
	AppliedSchemaVersion int                       `json:"appliedSchemaVersion"`
	PendingMigration     *PendingMigrationResponse `json:"pendingMigration,omitempty"`
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

			// Schema fields are best-effort: read errors don't fail status.
			if applied, err := dbInstance.CurrentSchemaVersion(ctx); err == nil {
				clusterStatus.AppliedSchemaVersion = applied
			} else {
				logger.APILog.Warn("status: read applied schema failed", zap.Error(err))
			}

			if pending, err := dbInstance.PendingMigrationInfo(ctx); err == nil {
				if pending.Pending {
					clusterStatus.PendingMigration = &PendingMigrationResponse{
						CurrentSchema: pending.CurrentSchema,
						TargetSchema:  pending.TargetSchema,
						LaggardNodeId: pending.LaggardNodeID,
					}
				}
			} else {
				logger.APILog.Warn("status: read pending migration info failed", zap.Error(err))
			}

			statusResponse.Cluster = clusterStatus

			w.Header().Set("X-Ella-Role", role)
		}

		writeResponse(r.Context(), w, statusResponse, http.StatusOK, logger.APILog)
	})
}
