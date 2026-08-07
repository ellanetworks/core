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
// idle, via a PFCP session modification.
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

	if smContext.Tunnel == nil && smContext.PFCPContext == nil {
		logger.WithTrace(ctx, logger.SmfLog).Debug("session already torn down, skipping deactivation", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil
	}

	farList, err := handleUpCnxStateDeactivate(smContext)
	if err != nil {
		return fmt.Errorf("error handling UP connection state: %v", err)
	}

	if smContext.PFCPContext == nil {
		return fmt.Errorf("pfcp session context not found for upf")
	}

	localSEID := smContext.PFCPContext.LocalSEID
	remoteSEID := smContext.PFCPContext.RemoteSEID

	err = s.upf.ModifySession(ctx, BuildModifyRequest(remoteSEID, "", nil, farList, nil))
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("PFCP session modification failed, clearing stale tunnel", zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID), logger.SEID(localSEID))
		smContext.Tunnel = nil
		smContext.PFCPContext = nil

		return fmt.Errorf("failed to send PFCP session modification request (localSEID=%d, remoteSEID=%d): %v", localSEID, remoteSEID, err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}

func handleUpCnxStateDeactivate(smContext *SMContext) ([]*FAR, error) {
	if smContext.Tunnel == nil {
		return nil, nil
	}

	if smContext.Tunnel.DataPath.DownLinkTunnel.PDR == nil {
		return nil, fmt.Errorf("AN Release Error, PDR is nil")
	}

	smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate
	smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction.Forw = false
	smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction.Buff = true
	smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction.Nocp = true

	if smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters != nil {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = nil
	}

	farList := []*FAR{smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR}

	return farList, nil
}
