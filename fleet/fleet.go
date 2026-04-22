// Copyright 2026 Ella Networks

package fleet

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

// recentUsageDays is the number of recent days (including today) to include
// in each sync payload. Fleet uses upsert semantics, so re-sending the same
// day is safe and ensures late-arriving counters are propagated.
const recentUsageDays = 3

// syncInterval is the cadence at which each node contacts Fleet. Long
// enough to keep the Raft log overhead from leader-side state writes
// manageable, short enough to push timely metrics/flows.
const syncInterval = 15 * time.Second

// ConfigReloader is called after a fleet sync applies a new config to the
// database so that runtime components (e.g. UPF/BPF) can be reloaded to
// match. Implementations must be safe for concurrent use.
type ConfigReloader interface {
	ReloadNAT(natEnabled bool) error
	ReloadFlowAccounting(flowAccountingEnabled bool) error
}

// SyncHandle is returned by ResumeSync and lets the caller attach a
// ConfigReloader after the sync loop is already running.
type SyncHandle struct {
	mu       sync.Mutex
	reloader ConfigReloader
}

func (h *SyncHandle) SetConfigReloader(r ConfigReloader) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reloader = r
}

func (h *SyncHandle) getReloader() ConfigReloader {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.reloader
}

// SyncCallback is invoked after each sync attempt with the success flag.
type SyncCallback func(ctx context.Context, success bool)

// StatusProvider returns the current local status of this node.
type StatusProvider func() client.EllaCoreStatus

// MetricsProvider returns the current local metrics snapshot.
type MetricsProvider func() client.EllaCoreMetrics

var (
	mu             sync.Mutex
	cancelPrevSync context.CancelFunc
)

func statusHash(s client.EllaCoreStatus) [sha256.Size]byte {
	b, _ := json.Marshal(s)
	return sha256.Sum256(b)
}

// statusTracker tracks the SHA-256 hash of the last successfully sent
// status and decides whether to include it in the next sync request.
// Status is always included on the first call.
type statusTracker struct {
	lastHash [sha256.Size]byte
	hasSent  bool
}

func (t *statusTracker) Prepare(s client.EllaCoreStatus) *client.EllaCoreStatus {
	h := statusHash(s)
	if !t.hasSent || h != t.lastHash {
		return &s
	}

	return nil
}

func (t *statusTracker) Confirm(s client.EllaCoreStatus) {
	t.lastHash = statusHash(s)
	t.hasSent = true
}

// syncer abstracts the fleet HTTP Sync call for testability.
type syncer interface {
	Sync(ctx context.Context, params *client.SyncParams) (*client.SyncResponse, error)
}

// syncDB abstracts the database operations needed by the sync loop. The
// leader-only operations (UpdateConfig, UpdateFleetConfigRevision,
// GetRawDailyUsage) are not called on followers.
type syncDB interface {
	UpdateConfig(ctx context.Context, cfg client.SyncConfig) error
	UpdateFleetConfigRevision(ctx context.Context, revision int64) error
	GetRawDailyUsage(ctx context.Context, start, end time.Time) ([]db.DailyUsage, error)
	GetFleet(ctx context.Context) (*db.Fleet, error)
	IsLeader() bool
	NodeID() int
}

const maxFlowRetries = 3

type syncRunner struct {
	syncer          syncer
	db              syncDB
	statusProvider  StatusProvider
	metricsProvider MetricsProvider
	tracker         statusTracker
	version         string
	clusterID       string
	lastKnownRev    int64
	onSync          SyncCallback
	buffer          *FleetBuffer
	heldFlows       []client.FlowEntry
	flowRetries     int
	handle          *SyncHandle
}

func (r *syncRunner) runOneCycle(ctx context.Context) {
	currentStatus := r.statusProvider()
	isLeader := r.db.IsLeader()

	var flowsToSend []client.FlowEntry

	if len(r.heldFlows) > 0 {
		flowsToSend = r.heldFlows
	} else if r.buffer != nil {
		flowsToSend = r.buffer.DrainFlows()
	}

	params := &client.SyncParams{
		Version:           r.version,
		NodeID:            r.db.NodeID(),
		ClusterID:         r.clusterID,
		IsLeader:          isLeader,
		LastKnownRevision: r.lastKnownRev,
		Status:            r.tracker.Prepare(currentStatus),
		Metrics:           r.metricsProvider(),
		Flows:             flowsToSend,
	}

	if isLeader {
		params.SubscriberUsage = r.collectUsage(ctx)
	}

	resp, err := r.syncer.Sync(ctx, params)
	if err != nil {
		logger.EllaLog.Error("sync failed", zap.Error(err))

		if len(flowsToSend) > 0 {
			r.heldFlows = flowsToSend
			r.flowRetries++

			if r.flowRetries >= maxFlowRetries {
				logger.EllaLog.Warn("dropping flow batch after max retries",
					zap.Int("dropped_flows", len(r.heldFlows)),
					zap.Int("retries", r.flowRetries))
				r.heldFlows = nil
				r.flowRetries = 0
			}
		}

		if r.onSync != nil {
			r.onSync(ctx, false)
		}

		return
	}

	r.heldFlows = nil
	r.flowRetries = 0

	if params.Status != nil {
		r.tracker.Confirm(currentStatus)
	}

	// Only the leader applies config and records the revision — a follower
	// must not write to the replicated fleet table (that would desync the
	// revision across nodes mid-cycle) and must not apply config changes
	// (the leader's apply already replicates).
	if isLeader && resp.Config != nil {
		if err := r.db.UpdateConfig(ctx, *resp.Config); err != nil {
			logger.EllaLog.Error("failed to apply fleet config", zap.Error(err))
		} else {
			r.lastKnownRev = resp.ConfigRevision

			if err := r.db.UpdateFleetConfigRevision(ctx, resp.ConfigRevision); err != nil {
				logger.EllaLog.Error("failed to update config revision", zap.Error(err))
			}

			r.reloadConfig(resp.Config)
		}
	}

	if r.onSync != nil {
		r.onSync(ctx, true)
	}
}

func (r *syncRunner) reloadConfig(cfg *client.SyncConfig) {
	if r.handle == nil {
		return
	}

	reloader := r.handle.getReloader()
	if reloader == nil {
		return
	}

	if err := reloader.ReloadNAT(cfg.Networking.NAT); err != nil {
		logger.EllaLog.Error("failed to reload NAT after fleet sync", zap.Error(err))
	}

	if err := reloader.ReloadFlowAccounting(cfg.Networking.FlowAccounting); err != nil {
		logger.EllaLog.Error("failed to reload flow accounting after fleet sync", zap.Error(err))
	}
}

func (r *syncRunner) collectUsage(ctx context.Context) []client.SubscriberUsageEntry {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -(recentUsageDays - 1))

	rows, err := r.db.GetRawDailyUsage(ctx, start, now)
	if err != nil {
		logger.EllaLog.Warn("failed to collect subscriber usage for fleet sync", zap.Error(err))
		return nil
	}

	entries := make([]client.SubscriberUsageEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, client.SubscriberUsageEntry{
			EpochDay:      row.EpochDay,
			IMSI:          row.IMSI,
			UplinkBytes:   row.BytesUplink,
			DownlinkBytes: row.BytesDownlink,
		})
	}

	return entries
}

// ResumeSyncInput groups the dependencies for starting the sync loop.
type ResumeSyncInput struct {
	FleetURL        string
	Key             *ecdsa.PrivateKey
	CertPEM         []byte
	CAPEM           []byte
	DB              *db.Database
	StatusProvider  StatusProvider
	MetricsProvider MetricsProvider
	OnSync          SyncCallback
	Buffer          *FleetBuffer
	ClusterID       string
}

// ResumeSync spins up the per-node sync loop. Every node — leader or
// follower — reports metrics, flows, and status; only the current leader
// sends subscriber usage and applies config pushed back from Fleet.
func ResumeSync(ctx context.Context, in ResumeSyncInput) (*SyncHandle, error) {
	fc := client.New(in.FleetURL)

	if err := fc.ConfigureMTLS(string(in.CertPEM), in.Key, string(in.CAPEM)); err != nil {
		return nil, fmt.Errorf("couldn't configure mTLS: %w", err)
	}

	fleetData, err := in.DB.GetFleet(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get fleet data: %w", err)
	}

	handle := &SyncHandle{}

	runner := &syncRunner{
		syncer:          fc,
		db:              in.DB,
		statusProvider:  in.StatusProvider,
		metricsProvider: in.MetricsProvider,
		version:         version.GetVersion().Version,
		clusterID:       in.ClusterID,
		lastKnownRev:    fleetData.ConfigRevision,
		onSync:          in.OnSync,
		buffer:          in.Buffer,
		handle:          handle,
	}

	runner.runOneCycle(ctx)

	mu.Lock()

	if cancelPrevSync != nil {
		cancelPrevSync()
	}

	syncCtx, cancel := context.WithCancel(ctx)
	cancelPrevSync = cancel

	mu.Unlock()

	ticker := time.NewTicker(syncInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				runner.runOneCycle(syncCtx)
			case <-syncCtx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	logger.EllaLog.Info("Resumed fleet sync from existing credentials")

	return handle, nil
}

// StopSync cancels the running sync goroutine, if any.
func StopSync() {
	mu.Lock()
	defer mu.Unlock()

	if cancelPrevSync != nil {
		cancelPrevSync()
		cancelPrevSync = nil

		logger.EllaLog.Info("Fleet sync stopped")
	}
}
