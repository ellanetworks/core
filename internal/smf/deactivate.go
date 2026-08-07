// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// DeactivateSmContext switches the downlink FAR to buffering when the UE goes
// idle, by restating the session to the UPF.
func (s *SMF) DeactivateSmContext(ctx context.Context, smContextRef string) error {
	return s.deactivateSmContext(ctx, smContextRef, Access5G)
}

func (s *SMF) deactivateSmContext(ctx context.Context, smContextRef string, access AccessType) error {
	ctx, span := tracer.Start(ctx, "smf/deactivate_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("smf.context_ref", smContextRef),
		),
	)
	defer span.End()

	smContext, unlock, err := s.sessionFor(smContextRef, access)
	if err != nil {
		return err
	}

	defer unlock()

	if smContext.Tunnel == nil && smContext.UPFSession == nil {
		logger.WithTrace(ctx, logger.SmfLog).Debug("session already torn down, skipping deactivation", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil
	}

	if err := suspendDownlink(smContext); err != nil {
		return fmt.Errorf("error handling UP connection state: %v", err)
	}

	if smContext.UPFSession == nil {
		return fmt.Errorf("UPF session context not found")
	}

	seid := smContext.UPFSession.SEID

	if err := s.applySession(ctx, smContext, nil); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("UPF session state could not be applied, clearing stale tunnel", zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID), logger.SEID(seid))
		smContext.Tunnel = nil
		smContext.UPFSession = nil

		return fmt.Errorf("failed to apply the UPF session state (seid=%d): %v", seid, err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Applied the UPF session state", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}

// suspendDownlink puts the downlink into buffering with notification, naming no
// tunnel endpoint: the access released the one it had.
func suspendDownlink(smContext *SMContext) error {
	if smContext.Tunnel == nil {
		return nil
	}

	if smContext.Tunnel.DataPath.DownLinkTunnel.PDR == nil {
		return fmt.Errorf("AN Release Error, PDR is nil")
	}

	far := smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	far.ApplyAction.Forw = false
	far.ApplyAction.Buff = true
	far.ApplyAction.Nocp = true

	if far.ForwardingParameters != nil {
		far.ForwardingParameters.OuterHeaderCreation = nil
	}

	return nil
}
