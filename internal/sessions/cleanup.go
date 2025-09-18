package sessions

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	RunEvery = 30 * time.Second
)

func CleanUp(ctx context.Context, dbInstance *db.Database) {
	ticker := time.NewTicker(RunEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.SessionsLog.Info("Session cleanup stopped")
			return
		case <-ticker.C:
			tickCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			numDel, err := dbInstance.DeleteExpiredSessions(tickCtx)

			cancel()

			if err != nil {
				logger.SessionsLog.Error("error deleting expired sessions", zap.Error(err))
				continue
			}

			if numDel > 0 {
				logger.SessionsLog.Info("deleted expired sessions", zap.Int("num", numDel))
			}
		}
	}
}
