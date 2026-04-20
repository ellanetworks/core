// Copyright 2024 Ella Networks
// SPDX-License-Identifier: Apache-2.0

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
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.PDUAddress != nil {
		_, releaseErr := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID)
		if releaseErr != nil {
			logger.SmfLog.Warn("release UE IP address failed", zap.Error(releaseErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID), logger.DNN(smContext.Dnn), zap.String("smContextRef", smContextRef))
		}
	}

	err := s.releaseTunnel(ctx, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release tunnel")
		s.removeSessionUnlocked(ctx, smContextRef)

		return fmt.Errorf("release tunnel failed: %v", err)
	}

	s.removeSessionUnlocked(ctx, smContextRef)

	return nil
}

// releaseTunnel deactivates the GTP data path and sends a PFCP deletion request.
func (s *SMF) releaseTunnel(ctx context.Context, smContext *SMContext) error {
	if smContext.Tunnel == nil {
		return nil
	}

	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR(s)

	if smContext.PFCPContext == nil {
		smContext.Tunnel = nil
		return nil
	}

	if err := s.upf.DeleteSession(ctx, smContext.PFCPContext.RemoteSEID); err != nil {
		return fmt.Errorf("send PFCP session deletion request failed: %v", err)
	}

	smContext.Tunnel = nil
	smContext.PFCPContext = nil

	return nil
}

// removeSessionUnlocked removes a session from the pool without releasing the IP
// (caller has already released it or does not want to).
func (s *SMF) removeSessionUnlocked(_ context.Context, ref string) {
	s.mu.Lock()
	delete(s.pool, ref)
	s.mu.Unlock()

	logger.SmfLog.Info("SM Context removed", zap.String("smContextRef", ref))
}
