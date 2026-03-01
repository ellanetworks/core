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

// ConfigReloader is called after a fleet sync applies a new config to the
// database so that runtime components (e.g. UPF/BPF) can be reloaded to
// match. Implementations must be safe for concurrent use.
type ConfigReloader interface {
	ReloadNAT(natEnabled bool) error
	ReloadFlowAccounting(flowAccountingEnabled bool) error
}

// SyncHandle is an opaque handle returned by ResumeSync that allows the
// caller to attach a ConfigReloader after the sync loop has started. This
// is necessary because the UPF may not be available when ResumeSync is
// first called.
type SyncHandle struct {
	mu       sync.Mutex
	reloader ConfigReloader
}

// SetConfigReloader sets (or replaces) the ConfigReloader that the sync
// loop will invoke after applying a new config. It is safe to call from
// any goroutine.
func (h *SyncHandle) SetConfigReloader(r ConfigReloader) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.reloader = r
}

// getReloader returns the current ConfigReloader, or nil.
func (h *SyncHandle) getReloader() ConfigReloader {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.reloader
}

// SyncCallback is called after each sync attempt.
// The success parameter indicates whether the sync was successful.
type SyncCallback func(ctx context.Context, success bool)

var (
	mu             sync.Mutex
	cancelPrevSync context.CancelFunc
)

// StatusProvider returns the current status of the Ella Core instance.
// It is called before each sync to send fresh status information to the fleet.
type StatusProvider func() client.EllaCoreStatus

// MetricsProvider returns the current metrics of the Ella Core instance.
// It is called before each sync to send fresh metrics to the fleet.
type MetricsProvider func() client.EllaCoreMetrics

// statusHash returns the SHA-256 hash of the JSON-serialized status.
// It is used to detect whether the status has changed since the last sync.
func statusHash(s client.EllaCoreStatus) [sha256.Size]byte {
	b, _ := json.Marshal(s)
	return sha256.Sum256(b)
}

// statusTracker tracks the SHA-256 hash of the last successfully sent status
// and decides whether the status should be included in the next sync request.
// Status is always included on the first call (first sync after startup).
// After a successful send, call Confirm to update the stored hash. If the
// sync fails, skip Confirm so the status is re-sent next cycle.
type statusTracker struct {
	lastHash [sha256.Size]byte
	hasSent  bool
}

// Prepare compares the current status to the last successfully sent hash.
// It returns a non-nil pointer when the status should be included in the
// request, or nil when it can be omitted.
func (t *statusTracker) Prepare(s client.EllaCoreStatus) *client.EllaCoreStatus {
	h := statusHash(s)
	if !t.hasSent || h != t.lastHash {
		return &s
	}

	return nil
}

// Confirm records the hash of the status that was just sent. Call this only
// after a successful sync where Prepare returned non-nil.
func (t *statusTracker) Confirm(s client.EllaCoreStatus) {
	t.lastHash = statusHash(s)
	t.hasSent = true
}

// syncer abstracts the fleet HTTP Sync call for testability.
type syncer interface {
	Sync(ctx context.Context, params *client.SyncParams) (*client.SyncResponse, error)
}

// syncDB abstracts the database operations needed during the sync loop.
type syncDB interface {
	UpdateConfig(ctx context.Context, cfg client.SyncConfig) error
	UpdateFleetConfigRevision(ctx context.Context, revision int64) error
	GetRawDailyUsage(ctx context.Context, start, end time.Time) ([]db.DailyUsage, error)
}

// maxFlowRetries is the number of consecutive failed sync cycles after which
// a held flow batch is dropped.
const maxFlowRetries = 3

// syncRunner holds all state and dependencies for the sync loop.
// It is created by ResumeSync and driven by a ticker.
type syncRunner struct {
	syncer          syncer
	db              syncDB
	statusProvider  StatusProvider
	metricsProvider MetricsProvider
	tracker         statusTracker
	version         string
	lastKnownRev    int64
	onSync          SyncCallback
	buffer          *FleetBuffer
	heldFlows       []client.FlowEntry
	flowRetries     int
	handle          *SyncHandle
}

// runOneCycle performs a single sync cycle: collects data, sends it to Fleet,
// and processes the response (apply config, update revision).
func (r *syncRunner) runOneCycle(ctx context.Context) {
	currentStatus := r.statusProvider()

	// Determine which flows to include. If a previous batch is held for
	// retry, re-send it without draining new flows from the buffer.
	var flowsToSend []client.FlowEntry

	if len(r.heldFlows) > 0 {
		flowsToSend = r.heldFlows
	} else if r.buffer != nil {
		flowsToSend = r.buffer.DrainFlows()
	}

	params := &client.SyncParams{
		Version:           r.version,
		LastKnownRevision: r.lastKnownRev,
		Status:            r.tracker.Prepare(currentStatus),
		Metrics:           r.metricsProvider(),
		Flows:             flowsToSend,
		SubscriberUsage:   r.collectUsage(ctx),
	}

	resp, err := r.syncer.Sync(ctx, params)
	if err != nil {
		logger.EllaLog.Error("sync failed", zap.Error(err))

		// Hold the flow batch for retry on the next cycle.
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

	// Sync succeeded — clear any held flow batch.
	r.heldFlows = nil
	r.flowRetries = 0

	if params.Status != nil {
		r.tracker.Confirm(currentStatus)
	}

	if resp.Config != nil {
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

// reloadConfig notifies runtime components (UPF/BPF) about configuration
// changes received from Fleet. If no ConfigReloader has been set yet (e.g.
// during the initial sync before UPF starts) the call is silently skipped.
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

// collectUsage fetches per-subscriber daily counters for the last few days
// from the database and converts them into the fleet sync format.
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

func ResumeSync(ctx context.Context, fleetURL string, key *ecdsa.PrivateKey, certPEM []byte, caPEM []byte, dbInstance *db.Database, statusProvider StatusProvider, metricsProvider MetricsProvider, onSync SyncCallback, buffer *FleetBuffer) (*SyncHandle, error) {
	fC := client.New(fleetURL)

	if err := fC.ConfigureMTLS(string(certPEM), key, string(caPEM)); err != nil {
		return nil, fmt.Errorf("couldn't configure mTLS: %w", err)
	}

	fleetData, err := dbInstance.GetFleet(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get fleet data: %w", err)
	}

	handle := &SyncHandle{}

	runner := &syncRunner{
		syncer:          fC,
		db:              dbInstance,
		statusProvider:  statusProvider,
		metricsProvider: metricsProvider,
		version:         version.GetVersion().Version,
		lastKnownRev:    fleetData.ConfigRevision,
		onSync:          onSync,
		buffer:          buffer,
		handle:          handle,
	}

	// Initial sync (synchronous, always includes status on first call).
	runner.runOneCycle(ctx)

	mu.Lock()

	if cancelPrevSync != nil {
		cancelPrevSync()
	}

	syncCtx, cancel := context.WithCancel(ctx)
	cancelPrevSync = cancel

	mu.Unlock()

	ticker := time.NewTicker(5 * time.Second)

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
