// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/procedure"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ReleaseSmContext tears down a PDU session entirely: releases the IP address,
// deletes the PFCP session on the UPF, and removes the context from the pool.
func (s *SMF) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	return s.releaseSmContext(ctx, smContextRef, Access5G)
}

// releaseSmContext tears the session down for the access serving it. A release
// arriving on the target of a move that access never bound names an abandoned
// leg, not the session: the leg is dropped and the session stays where it is.
func (s *SMF) releaseSmContext(ctx context.Context, smContextRef string, access AccessType) error {
	ctx, span := tracer.Start(ctx, "smf/release_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("smf.context_ref", smContextRef),
		),
	)
	defer span.End()

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		// Releasing an already-released session is a no-op success: the release is
		// idempotent, so a caller that tears down the user plane up front and again on
		// completion (e.g. the 4G deactivation handshake) does not see a spurious error.
		logger.SmfLog.Debug("release: sm context already released", zap.String("smContextRef", smContextRef))

		return nil
	}

	if err := smContext.procedures.Begin(procedure.Release); err != nil {
		return fmt.Errorf("release session %q: %w", smContextRef, err)
	}

	defer smContext.procedures.End(procedure.Release)

	smContext.mu.Lock()

	if smContext.Access != access {
		defer smContext.mu.Unlock()

		if smContext.pending != nil && smContext.pending.to == access {
			smContext.pending = nil

			return nil
		}

		return fmt.Errorf("session %q is on %s", smContext.Ref, smContext.Access)
	}

	// The access a transfer was moving onto installed its own state from the
	// accept it already sent, and holds no routing this release would reach.
	if abandoned := smContext.pending; abandoned != nil {
		id := smContext.SessionIdentity
		id.EBI = abandoned.ebi

		defer func() { s.reportRelease(ctx, smContext, abandoned.to, id) }()
	}

	defer smContext.mu.Unlock()

	smContext.pending = nil

	// Stop any outstanding network-requested procedure retransmission so it does
	// not keep firing against a released session.
	smContext.stopProcedureTimer()

	err := s.releaseUserPlaneThenAddresses(ctx, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release user plane")
	}

	// Remove from pool after all network I/O is complete.
	s.dropFromPool(smContext)

	return err
}

// releaseUserPlaneThenAddresses purges the UPF session (and its NAT conntrack)
// before releasing the IP leases: an address freed while its conntrack survives
// can be re-leased to another subscriber that then receives the previous
// subscriber's flows. On teardown failure the leases are kept, so the address
// stays bound to this IMSI. Caller holds sc.mu.
func (s *SMF) releaseUserPlaneThenAddresses(ctx context.Context, sc *SMContext) error {
	if err := s.releaseTunnel(ctx, sc); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("user-plane teardown failed; keeping IP lease to prevent reuse with stale NAT conntrack",
			zap.Error(err), logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), logger.DNN(sc.Dnn))

		return err
	}

	if sc.PDUIPV4Address == nil && sc.PDUIPV6Prefix == nil {
		return nil
	}

	dn, err := s.store.ResolveDNN(ctx, sc.Dnn)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("resolve data network for UE address release failed; keeping IP lease",
			zap.Error(err), logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), logger.DNN(sc.Dnn))

		return fmt.Errorf("resolve data network for address release: %w", err)
	}

	s.releaseAllocatedAddresses(ctx, dn, sc)

	return nil
}

func (s *SMF) releaseTunnel(ctx context.Context, smContext *SMContext) error {
	if smContext.Tunnel == nil {
		return nil
	}

	// Unregister the IPv6 session from the RA responder before tearing down
	// the tunnel so that any in-flight RS events are dropped cleanly.
	if smContext.PDUIPV6Prefix != nil {
		ulTEID := smContext.Tunnel.DataPath.UpLinkTunnel.TEID
		if ulTEID != 0 {
			if err := s.upf.UnregisterIPv6Session(ctx, ulTEID); err != nil {
				logger.SmfLog.Warn("failed to unregister IPv6 session for RA",
					zap.Error(err),
					logger.SUPI(smContext.Supi.String()),
					logger.PDUSessionID(smContext.PDUSessionID),
				)
			}
		}
	}

	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR()

	if smContext.PFCPContext == nil {
		smContext.Tunnel = nil
		return nil
	}

	s.upf.FlushUsage(ctx, smContext.PFCPContext.RemoteSEID)

	if err := s.upf.DeleteSession(ctx, smContext.PFCPContext.RemoteSEID); err != nil {
		return fmt.Errorf("send PFCP session deletion request failed: %v", err)
	}

	smContext.Tunnel = nil
	smContext.PFCPContext = nil

	return nil
}

// removeSessionUnlocked removes a session from the pool without releasing the IP
// (caller has already released it or does not want to). ref is the session's unique
// Ref; a no-op if it is already gone.
func (s *SMF) removeSessionUnlocked(_ context.Context, ref string) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	s.dropFromPool(sc)
}
