package sessions

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	RunEvery = 30 * time.Second
)

var tracer = otel.Tracer("ella-core/sessions")

// LeaderGuard tracks leadership state for the session cleanup worker.
// Implements raft.LeaderCallback.
type LeaderGuard struct {
	isLeader atomic.Bool
}

func NewLeaderGuard() *LeaderGuard {
	return &LeaderGuard{}
}

func (g *LeaderGuard) OnBecameLeader()   { g.isLeader.Store(true) }
func (g *LeaderGuard) OnLostLeadership() { g.isLeader.Store(false) }
func (g *LeaderGuard) IsLeader() bool    { return g.isLeader.Load() }

func CleanUp(ctx context.Context, dbInstance *db.Database, guard *LeaderGuard) {
	ticker := time.NewTicker(RunEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.SessionsLog.Info("Session cleanup stopped")
			return
		case <-ticker.C:
			if !guard.IsLeader() {
				continue
			}

			tickCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			tickCtx, span := tracer.Start(tickCtx, "sessions/cleanup",
				trace.WithSpanKind(trace.SpanKindInternal),
			)

			numDel, err := dbInstance.DeleteExpiredSessions(tickCtx)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to delete expired sessions")
				logger.WithTrace(tickCtx, logger.SessionsLog).Error("error deleting expired sessions", zap.Error(err))
				span.End()
				cancel()

				continue
			}

			if numDel > 0 {
				logger.WithTrace(tickCtx, logger.SessionsLog).Info("deleted expired sessions", zap.Int("num", numDel))
			}

			span.End()
			cancel()
		}
	}
}
