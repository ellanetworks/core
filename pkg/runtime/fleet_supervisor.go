// Copyright 2026 Ella Networks

package runtime

import (
	"context"
	"time"

	"github.com/ellanetworks/core/fleet"
	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf"
	"go.uber.org/zap"
)

// fleetSupervisorInterval is how often the supervisor re-checks whether
// the Core is currently Fleet-managed.
const fleetSupervisorInterval = 30 * time.Second

// runFleetSupervisor polls the replicated fleet row on every node and
// starts or stops the per-node sync loop to match. Registration and
// unregistration go through the leader; followers observe the state
// change via Raft replication.
func runFleetSupervisor(ctx context.Context, dbInstance *db.Database, cfg config.Config, amfInstance *amf.AMF, upfInstance *upf.UPF, buffer *fleet.FleetBuffer) {
	ticker := time.NewTicker(fleetSupervisorInterval)
	defer ticker.Stop()

	var running bool

	check := func() {
		managed, err := dbInstance.IsFleetManaged(ctx)
		if err != nil {
			logger.EllaLog.Warn("fleet supervisor: couldn't read fleet state", zap.Error(err))
			return
		}

		switch {
		case managed && !running:
			if err := startFleetSync(ctx, dbInstance, cfg, amfInstance, upfInstance, buffer); err != nil {
				logger.EllaLog.Warn("fleet supervisor: failed to start sync", zap.Error(err))
				return
			}

			running = true

		case !managed && running:
			fleet.StopSync()

			running = false
		}
	}

	check()

	for {
		select {
		case <-ctx.Done():
			if running {
				fleet.StopSync()
			}

			return
		case <-ticker.C:
			check()
		}
	}
}

func startFleetSync(ctx context.Context, dbInstance *db.Database, cfg config.Config, amfInstance *amf.AMF, upfInstance *upf.UPF, buffer *fleet.FleetBuffer) error {
	fleetData, err := dbInstance.GetFleet(ctx)
	if err != nil {
		return err
	}

	key, err := dbInstance.LoadOrGenerateFleetKey(ctx)
	if err != nil {
		return err
	}

	clusterID := ""

	if op, err := dbInstance.GetOperator(ctx); err == nil {
		clusterID = op.ClusterID
	}

	statusProvider := func() client.EllaCoreStatus {
		return server.BuildStatus(context.Background(), dbInstance, cfg, amfInstance)
	}

	metricsProvider := func() client.EllaCoreMetrics {
		return server.BuildMetrics()
	}

	handle, err := fleet.ResumeSync(ctx, fleet.ResumeSyncInput{
		FleetURL:        fleetData.URL,
		Key:             key,
		CertPEM:         fleetData.Certificate,
		CAPEM:           fleetData.CACertificate,
		DB:              dbInstance,
		StatusProvider:  statusProvider,
		MetricsProvider: metricsProvider,
		OnSync: func(syncCtx context.Context, success bool) {
			// Only the leader persists the last-sync timestamp — the
			// fleet table is raft-replicated and followers must not
			// propose into it (would desync).
			if success && dbInstance.IsLeader() {
				if err := dbInstance.UpdateFleetSyncStatus(syncCtx); err != nil {
					logger.EllaLog.Error("couldn't update fleet sync status", zap.Error(err))
				}
			}
		},
		Buffer:    buffer,
		ClusterID: clusterID,
	})
	if err != nil {
		return err
	}

	if upfInstance != nil {
		handle.SetConfigReloader(upfInstance)
	}

	return nil
}
