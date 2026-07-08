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

	if smContext.PDUIPV4Address != nil {
		_, releaseErr := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID)
		if releaseErr != nil {
			logger.SmfLog.Warn("release UE IP address failed", zap.Error(releaseErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID), logger.DNN(smContext.Dnn), zap.String("smContextRef", smContextRef))
		}
	}

	if smContext.PDUIPV6Prefix != nil {
		_, releaseErr := s.store.ReleaseIPv6(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID)
		if releaseErr != nil {
			logger.SmfLog.Warn("release UE IPv6 address failed", zap.Error(releaseErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID), logger.DNN(smContext.Dnn), zap.String("smContextRef", smContextRef))
		}
	}

	err := s.releaseTunnel(ctx, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release tunnel")
	}

	// Remove from pool after all network I/O is complete.
	s.dropFromPool(smContext)

	return err
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

	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR(s)

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
