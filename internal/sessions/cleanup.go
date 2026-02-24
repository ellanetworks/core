package sessions

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	RunEvery = 30 * time.Second
)

var tracer = otel.Tracer("ella-core/sessions")

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
			tickCtx, span := tracer.Start(tickCtx, "session cleanup",
				trace.WithSpanKind(trace.SpanKindInternal),
			)

			numDel, err := dbInstance.DeleteExpiredSessions(tickCtx)
			if err != nil {
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
