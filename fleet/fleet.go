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

var (
	mu             sync.Mutex
	cancelPrevSync context.CancelFunc
)

func ResumeSync(ctx context.Context, fleetURL string, key *ecdsa.PrivateKey, certPEM []byte, caPEM []byte) error {
	fC := client.New(fleetURL)

	if err := fC.ConfigureMTLS(string(certPEM), key, string(caPEM)); err != nil {
		return fmt.Errorf("couldn't configure mTLS: %w", err)
	}

	syncParams := &client.SyncParams{
		Version: version.GetVersion().Version,
	}

	if err := fC.Sync(ctx, syncParams); err != nil {
		logger.EllaLog.Error("initial sync failed", zap.Error(err))
	} else {
		logger.EllaLog.Info("Initial sync sent successfully to fleet")
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
				} else {
					logger.EllaLog.Info("Sync sent successfully to fleet")
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
