// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Delete removes a session and everything it holds on the datapath.
func (conn *SessionEngine) Delete(ctx context.Context, seid uint64) error {
	ctx, span := tracer.Start(ctx, "upf/delete_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("session.operation", "delete"),
			attribute.Int64("session.seid", int64(seid)),
		),
	)
	defer span.End()

	session := conn.GetSession(seid)
	if session == nil {
		err := fmt.Errorf("session not found for SEID %d", seid)
		span.RecordError(err)
		span.SetStatus(codes.Error, "session not found")

		return err
	}

	session.opMu.Lock()
	defer session.opMu.Unlock()

	if session.deleted {
		return nil
	}

	// Marked deleted before teardown so a filter propagation that acquires opMu
	// after this point skips the session (see applyFilterIndexToSession).
	session.deleted = true

	bpfObjects := conn.BpfObjects

	// An already-removed map key is a benign no-op; the session must still be fully
	// removed so a torn-down session is not orphaned in the engine (reporting usage and
	// leaking forwarding state to a later session that reuses the UE IP).
	var teardownErr error

	for _, pdrInfo := range session.ListPDRs() {
		if err := deletePDR(seid, pdrInfo, bpfObjects, conn.FteIDResourceManager); err != nil {
			teardownErr = errors.Join(teardownErr, err)
		}
	}

	for urrID := range session.URRs() {
		if err := bpfObjects.DeleteUrr(seid, urrID); err != nil {
			teardownErr = errors.Join(teardownErr, err)
		}
	}

	for _, fr := range session.FramedRoutes() {
		if err := bpfObjects.DeleteFramedDownlink(fr); err != nil {
			teardownErr = errors.Join(teardownErr, err)
		}
	}

	if bpfObjects != nil {
		bpfObjects.ClearNotifiedForSEID(seid)
	}

	ueIPv4, _ := session.UEAddresses()
	conn.purgeNATConntrack(ueIPv4)

	policyID := session.PolicyID()

	conn.mu.Lock()
	delete(conn.sessions, seid)
	conn.deregisterPolicy(policyID, seid)
	conn.mu.Unlock()

	conn.ReleaseResources(seid)

	if teardownErr != nil {
		span.RecordError(teardownErr)
		logger.WithTrace(ctx, logger.UpfLog).Warn("deleted session with residual teardown errors",
			logger.SEID(seid), zap.Error(teardownErr))
	}

	logger.WithTrace(ctx, logger.UpfLog).Info("Deleted session", logger.SEID(seid))

	return nil
}
