package server

import (
	"net/http"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

// PendingMigrationResponse describes the cluster's schema-migration
// readiness. When the cluster is fully migrated (applied schema equals
// every voter's binary max), the parent ClusterStatusResponse omits
// this field entirely.
type PendingMigrationResponse struct {
	// CurrentSchema is the schema version the cluster has applied.
	CurrentSchema int `json:"currentSchema"`

	// TargetSchema is the highest version the cluster could migrate
	// to now, bounded by the local binary and every voter's
	// MaxSchemaVersion.
	TargetSchema int `json:"targetSchema"`

	// LaggardNodeId is the voter blocking the migration; non-zero
	// only when target equals current (blocked). Zero when the
	// migration is in progress (target > current) or when there is
	// no quorum-blocking laggard.
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

	// AppliedSchemaVersion is the schema version every node in the
	// cluster has committed. Differs from the top-level
	// SchemaVersion (which is the local binary's max) during a
	// rolling upgrade window between the new binary deploying and
	// the leader proposing CmdMigrateShared.
	AppliedSchemaVersion int `json:"appliedSchemaVersion"`

	// PendingMigration is non-nil when the cluster has a migration
	// in flight or blocked by a laggard voter. See
	// PendingMigrationResponse.
	PendingMigration *PendingMigrationResponse `json:"pendingMigration,omitempty"`
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

			// Mid-rolling-upgrade visibility: applied schema vs. the
			// binary's max, and which voter (if any) is holding a
			// pending migration up. Read errors are non-fatal — the
			// status response should never fail because of them.
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
