package fleet

import (
	"context"
	"crypto/ecdsa"
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

// collectSubscriberUsage fetches per-subscriber daily counters for the last
// few days from the database and converts them into the fleet sync format.
func collectSubscriberUsage(ctx context.Context, dbInstance *db.Database, days int) []client.SubscriberUsageEntry {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -(days - 1))

	rows, err := dbInstance.GetRawDailyUsage(ctx, start, now)
	if err != nil {
		logger.EllaLog.Warn("failed to collect subscriber usage for fleet sync", zap.Error(err))
		return nil
	}

	entries := make([]client.SubscriberUsageEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, client.SubscriberUsageEntry{
			EpochDay:      r.EpochDay,
			IMSI:          r.IMSI,
			UplinkBytes:   r.BytesUplink,
			DownlinkBytes: r.BytesDownlink,
		})
	}

	return entries
}

func ResumeSync(ctx context.Context, fleetURL string, key *ecdsa.PrivateKey, certPEM []byte, caPEM []byte, dbInstance *db.Database, statusProvider StatusProvider, metricsProvider MetricsProvider, onSync SyncCallback) error {
	fC := client.New(fleetURL)

	if err := fC.ConfigureMTLS(string(certPEM), key, string(caPEM)); err != nil {
		return fmt.Errorf("couldn't configure mTLS: %w", err)
	}

	fleetData, err := dbInstance.GetFleet(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get fleet data: %w", err)
	}

	lastKnownRevision := fleetData.ConfigRevision

	syncParams := &client.SyncParams{
		Version:           version.GetVersion().Version,
		LastKnownRevision: lastKnownRevision,
		Status:            statusProvider(),
		Metrics:           metricsProvider(),
		SubscriberUsage:   collectSubscriberUsage(ctx, dbInstance, recentUsageDays),
	}

	if resp, err := fC.Sync(ctx, syncParams); err != nil {
		logger.EllaLog.Error("initial sync failed", zap.Error(err))

		if onSync != nil {
			onSync(ctx, false)
		}
	} else {
		if resp.Config != nil {
			if err := dbInstance.UpdateConfig(ctx, *resp.Config); err != nil {
				logger.EllaLog.Error("failed to apply fleet config", zap.Error(err))
			} else {
				lastKnownRevision = resp.ConfigRevision
				syncParams.LastKnownRevision = lastKnownRevision

				if err := dbInstance.UpdateFleetConfigRevision(ctx, resp.ConfigRevision); err != nil {
					logger.EllaLog.Error("failed to update config revision", zap.Error(err))
				}
			}
		} else {
			logger.EllaLog.Info("Fleet config is up to date, no changes applied")
		}

		if onSync != nil {
			onSync(ctx, true)
		}
	}

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
				syncParams.Status = statusProvider()
				syncParams.Metrics = metricsProvider()
				syncParams.SubscriberUsage = collectSubscriberUsage(syncCtx, dbInstance, recentUsageDays)

				resp, err := fC.Sync(syncCtx, syncParams)
				if err != nil {
					logger.EllaLog.Error("sync failed", zap.Error(err))

					if onSync != nil {
						onSync(syncCtx, false)
					}
				} else {
					if resp.Config != nil {
						if err := dbInstance.UpdateConfig(syncCtx, *resp.Config); err != nil {
							logger.EllaLog.Error("failed to apply fleet config", zap.Error(err))
						} else {
							lastKnownRevision = resp.ConfigRevision
							syncParams.LastKnownRevision = lastKnownRevision

							if err := dbInstance.UpdateFleetConfigRevision(syncCtx, resp.ConfigRevision); err != nil {
								logger.EllaLog.Error("failed to update config revision", zap.Error(err))
							}
						}
					} else {
						logger.EllaLog.Debug("Fleet config is up to date, no changes applied")
					}

					if onSync != nil {
						onSync(syncCtx, true)
					}
				}
			case <-syncCtx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	logger.EllaLog.Info("Resumed fleet sync from existing credentials")

	return nil
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
