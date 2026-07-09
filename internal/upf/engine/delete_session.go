// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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

	// Delete every PDR best-effort. A PDR whose BPF map entry is already gone —
	// e.g. a duplicate PDR sharing a downlink UE-IP key that an earlier iteration
	// removed — is a benign no-op: the session must still be fully removed,
	// otherwise the engine keeps reporting usage for a torn-down session and its
	// stale forwarding state leaks into a later session that reuses the UE IP.
	var pdrErr error

	for _, pdrInfo := range session.ListPDRs() {
		if err := pdrContext.deletePDR(pdrInfo, bpfObjects); err != nil {
			pdrErr = errors.Join(pdrErr, err)
		}
	}

	policyID := session.PolicyID()

	conn.mu.Lock()
	delete(conn.sessions, req.SEID)
	conn.deregisterPolicy(policyID, req.SEID)
	conn.mu.Unlock()

	conn.ReleaseResources(req.SEID)

	if pdrErr != nil {
		span.RecordError(pdrErr)
		logger.WithTrace(ctx, logger.UpfLog).Warn("deleted session with residual PDR-delete errors",
			logger.SEID(req.SEID), zap.Error(pdrErr))
	}

	logger.WithTrace(ctx, logger.UpfLog).Info("Deleted session", logger.SEID(req.SEID))

	return nil
}
