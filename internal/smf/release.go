// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ReleaseSmContext tears down a PDU session entirely: releases the IP address,
// deletes the PFCP session on the UPF, and removes the context from the pool.
func (s *SMF) ReleaseSmContext(ctx context.Context, smContextRef string) error {
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

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

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
// stays bound to this IMSI.
//
// Caller holds sc.Mutex, and holds it again on return. Releasing the leases is a
// database write — in a cluster a Raft round trip with a multi-second timeout —
// so sc.Mutex is dropped across it: SCTP dispatch is one goroutine per
// association, so holding it stalls every other UE on the same gNB/eNB. The
// tunnel and addresses are already cleared by then, so a reader arriving in the
// window sees a fully released session rather than a half-torn one.
func (s *SMF) releaseUserPlaneThenAddresses(ctx context.Context, sc *SMContext) error {
	if err := s.releaseTunnel(ctx, sc); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("user-plane teardown failed; keeping IP lease to prevent reuse with stale NAT conntrack",
			zap.Error(err), logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), logger.DNN(sc.Dnn))

		return err
	}

	if sc.PDUIPV4Address == nil && sc.PDUIPV6Prefix == nil {
		return nil
	}

	// Cleared before unlocking, so a second releaser in the window does not queue
	// a duplicate.
	var (
		supi  = sc.Supi
		dnn   = sc.Dnn
		keyID = sc.keyID()
		psi   = sc.PDUSessionID
		hasV4 = sc.PDUIPV4Address != nil
		hasV6 = sc.PDUIPV6Prefix != nil
	)

	sc.PDUIPV4Address = nil
	sc.PDUIPV6Prefix = nil

	sc.Mutex.Unlock()
	defer sc.Mutex.Lock()

	dn, err := s.store.ResolveDNN(ctx, dnn)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("resolve data network for UE address release failed; lease will be reclaimed by the retention sweep",
			zap.Error(err), logger.SUPI(supi.String()), logger.PDUSessionID(psi), logger.DNN(dnn))

		return fmt.Errorf("resolve data network for address release: %w", err)
	}

	imsi := supi.IMSI()

	if hasV4 {
		if _, err := dn.ReleaseIP(ctx, imsi, keyID); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv4 address", zap.Error(err))
		}
	}

	if hasV6 {
		if _, err := dn.ReleaseIPv6(ctx, imsi, keyID); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv6 address", zap.Error(err))
		}
	}

	return nil
}

// Has to run wherever the tunnel is dropped, not only on release: the responder
// is keyed by uplink TEID alone, so an entry that outlives its tunnel answers
// for a TEID this session no longer owns. Caller holds sc.Mutex.
func (s *SMF) unregisterIPv6Session(ctx context.Context, smContext *SMContext) {
	if smContext.Tunnel == nil || smContext.PDUIPV6Prefix == nil {
		return
	}

	ulTEID := smContext.Tunnel.N3TEID
	if ulTEID == 0 {
		return
	}

	if err := s.upf.UnregisterIPv6Session(ctx, ulTEID); err != nil {
		logger.SmfLog.Warn("failed to unregister IPv6 session for RA",
			zap.Error(err),
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)
	}
}

func (s *SMF) releaseTunnel(ctx context.Context, smContext *SMContext) error {
	if smContext.Tunnel == nil {
		return nil
	}

	// Before the teardown, so in-flight RS events are dropped cleanly.
	s.unregisterIPv6Session(ctx, smContext)

	if smContext.PFCPContext == nil {
		smContext.Tunnel = nil
		return nil
	}

	s.upf.FlushUsage(ctx, smContext.PFCPContext.SEID)

	if err := s.upf.DeleteSession(ctx, smContext.PFCPContext.SEID); err != nil {
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
