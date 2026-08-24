// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sessions

import (
	"context"
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

func CleanUp(ctx context.Context, dbInstance *db.Database) {
	ticker := time.NewTicker(RunEvery)
	defer ticker.Stop()

	runCleanupPass(ctx, dbInstance)

	for {
		select {
		case <-ctx.Done():
			logger.SessionsLog.Info("Session cleanup stopped")
			return
		case <-ticker.C:
		}

		runCleanupPass(ctx, dbInstance)
	}
}

func runCleanupPass(ctx context.Context, dbInstance *db.Database) {
	if !dbInstance.IsLeader() {
		return
	}

	tickCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tickCtx, span := tracer.Start(tickCtx, "sessions/cleanup",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	numDel, err := dbInstance.DeleteExpiredSessions(tickCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete expired sessions")
		logger.WithTrace(tickCtx, logger.SessionsLog).Error("error deleting expired sessions", zap.Error(err))

		return
	}

	if numDel > 0 {
		logger.WithTrace(tickCtx, logger.SessionsLog).Info("deleted expired sessions", zap.Int("num", numDel))
	}
}
