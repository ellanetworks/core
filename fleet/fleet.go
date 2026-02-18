package fleet

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

// SyncCallback is called after each sync attempt.
// The success parameter indicates whether the sync was successful.
type SyncCallback func(ctx context.Context, success bool)

var (
	mu             sync.Mutex
	cancelPrevSync context.CancelFunc
)

func ResumeSync(ctx context.Context, fleetURL string, key *ecdsa.PrivateKey, certPEM []byte, caPEM []byte, onSync SyncCallback) error {
	fC := client.New(fleetURL)

	if err := fC.ConfigureMTLS(string(certPEM), key, string(caPEM)); err != nil {
		return fmt.Errorf("couldn't configure mTLS: %w", err)
	}

	syncParams := &client.SyncParams{
		Version: version.GetVersion().Version,
	}

	if err := fC.Sync(ctx, syncParams); err != nil {
		logger.EllaLog.Error("initial sync failed", zap.Error(err))

		if onSync != nil {
			onSync(ctx, false)
		}
	} else {
		logger.EllaLog.Info("Initial sync sent successfully to fleet")

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
				if err := fC.Sync(syncCtx, syncParams); err != nil {
					logger.EllaLog.Error("sync failed", zap.Error(err))

					if onSync != nil {
						onSync(syncCtx, false)
					}
				} else {
					logger.EllaLog.Info("Sync sent successfully to fleet")

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
