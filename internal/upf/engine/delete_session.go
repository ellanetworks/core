// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DeleteSession deletes an existing UPF session by SEID.
func (conn *SessionEngine) DeleteSession(ctx context.Context, req *models.DeleteRequest) error {
	ctx, span := tracer.Start(ctx, "upf/delete_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("session.operation", "delete"),
			attribute.Int64("session.seid", int64(req.SEID)),
		),
	)
	defer span.End()

	session := conn.GetSession(req.SEID)
	if session == nil {
		err := fmt.Errorf("session not found for SEID %d", req.SEID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "session not found")

		return err
	}

	bpfObjects := conn.BpfObjects
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	for _, pdrInfo := range session.ListPDRs() {
		if err := pdrContext.deletePDR(pdrInfo, bpfObjects); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to delete PDR")

			return fmt.Errorf("couldn't delete PDR: %w", err)
		}
	}

	conn.RemoveSession(req.SEID)

	logger.WithTrace(ctx, logger.UpfLog).Info("Deleted session", logger.SEID(req.SEID))

	conn.ReleaseResources(req.SEID)

	return nil
}
