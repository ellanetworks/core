// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DeleteSession deletes an existing UPF session by SEID.
func (conn *SessionEngine) DeleteSession(ctx context.Context, req *models.DeleteRequest) (*models.DeleteResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/delete_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attrs.SessionOperation("delete"),
			attrs.SEID(req.SEID),
		),
	)
	defer span.End()

	session := conn.GetSession(req.SEID)
	if session == nil {
		err := fmt.Errorf("%w: SEID %d", models.ErrSessionNotFound, req.SEID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "session not found")

		return nil, err
	}

	session.opMu.Lock()
	defer session.opMu.Unlock()

	if session.deleted && conn.GetSession(req.SEID) != session {
		return &models.DeleteResponse{}, nil
	}

	// Mark deleted before teardown so a filter propagation that acquires opMu
	// after this point skips the session (see applyFilterIndexToSession).
	session.deleted = true

	bpfObjects := conn.BpfObjects
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	var pdrErr error

	session.addCleanupPDRs(slices.Collect(maps.Values(session.ListPDRs()))...)
	session.clearPDRs()

	var remaining []SPDRInfo

	for _, pdrInfo := range session.cleanupPDRs {
		if err := pdrContext.withdrawPDR(pdrInfo, bpfObjects); err != nil {
			pdrErr = errors.Join(pdrErr, err)

			remaining = append(remaining, pdrInfo)
		}
	}

	session.cleanupPDRs = remaining

	if bpfObjects != nil && !session.unpublished {
		if err := bpfObjects.DeleteClassifier(req.SEID); err != nil {
			pdrErr = errors.Join(pdrErr, err)
		}
	}

	var remainingRoutes []netip.Prefix

	for _, fr := range session.FramedRoutes() {
		if err := withdrawFramedRoute(bpfObjects, session, fr); err != nil {
			pdrErr = errors.Join(pdrErr, err)

			remainingRoutes = append(remainingRoutes, fr)
		}
	}

	session.SetFramedRoutes(remainingRoutes)

	if pdrErr != nil {
		span.RecordError(pdrErr)
		return nil, fmt.Errorf("could not withdraw session references: %w", pdrErr)
	}

	if !session.unpublished {
		if err := conn.waitForPrograms(session); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("wait for running datapath programs: %w", err)
		}
	}

	if err := conn.deleteQERMeters(session, slices.Sorted(maps.Keys(session.qerMeters))); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("could not remove session QER meters: %w", err)
	}

	usage, err := conn.closeUsage(ctx, session)
	if err != nil {
		span.RecordError(err)
		return &models.DeleteResponse{Usage: usage}, fmt.Errorf("could not remove session URRs: %w", err)
	}

	if bpfObjects != nil {
		bpfObjects.ClearNotifiedForSEID(req.SEID)
	}

	if b := conn.downlinkBuffer(); b != nil {
		b.Drop(req.SEID)
	}

	ueIPv4, _ := session.UEAddresses()
	conn.purgeNATConntrack(ueIPv4)

	policyID := session.PolicyID()

	conn.mu.Lock()
	delete(conn.sessions, req.SEID)
	conn.deregisterPolicy(policyID, req.SEID)
	conn.mu.Unlock()

	conn.ReleaseResources(req.SEID)

	logger.From(ctx, logger.UpfLog).Info("Deleted session", logger.SEID(req.SEID))

	return &models.DeleteResponse{Usage: usage}, nil
}
