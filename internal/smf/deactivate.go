// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (s *SMF) DeactivateSmContext(ctx context.Context, smContextRef string) error {
	return s.deactivateSession(ctx, smContextRef, Access5G)
}

func (s *SMF) deactivateSession(ctx context.Context, smContextRef string, by AccessType) error {
	ctx, span := tracer.Start(ctx, "smf/deactivate_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("smf.context_ref", smContextRef),
		),
	)
	defer span.End()

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Access != by {
		logger.WithTrace(ctx, logger.SmfLog).Debug("skipping deactivation for an access that no longer serves the session",
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		return nil
	}

	// Leave any network-requested procedure timer running: CM/ECM-IDLE is resolved
	// by paging, not by abandoning the procedure (TS 24.501 §6.3.2.5/§6.3.3.5).

	// Session already torn down; nothing to deactivate.
	if smContext.Tunnel == nil && smContext.PFCPContext == nil {
		logger.WithTrace(ctx, logger.SmfLog).Debug("session already torn down, skipping deactivation",
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		return nil
	}

	if smContext.Tunnel == nil || smContext.PFCPContext == nil {
		return fmt.Errorf("session %q has half a user plane to deactivate", smContextRef)
	}

	seid := smContext.PFCPContext.SEID

	next := smContext.Tunnel.dataPlane
	next.Downlink = DownlinkBuffering

	if err := s.applyDataPlane(ctx, smContext, next, ""); err != nil {
		// Any other failure leaves the UPF holding the session, its PDRs, its TEID
		// and its UE-IP map entries; nothing sweeps orphaned SEIDs, so dropping the
		// SEID there strands all of them.
		if errors.Is(err, models.ErrSessionNotFound) {
			logger.WithTrace(ctx, logger.SmfLog).Warn("PFCP session is gone on the UPF, clearing stale tunnel",
				zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID),
				logger.SEID(seid))
			// The responder is keyed by uplink TEID with no owner check, so an entry
			// outliving its tunnel keeps answering for a TEID this session lost.
			s.unregisterIPv6Session(ctx, smContext)
			smContext.Tunnel = nil
			smContext.PFCPContext = nil
		} else {
			logger.WithTrace(ctx, logger.SmfLog).Warn("PFCP session modification failed; keeping the SEID so the session can still be deleted",
				zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID),
				logger.SEID(seid))
		}

		return fmt.Errorf("deactivate the user plane of session %d: %w", seid, err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}
