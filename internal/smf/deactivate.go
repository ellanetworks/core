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

	// Leave any network-requested procedure timer running: CM/ECM-IDLE is resolved
	// by paging, not by abandoning the procedure (TS 24.501 §6.3.2.5/§6.3.3.5).

	// Session already torn down; nothing to deactivate.
	if smContext.Tunnel == nil && smContext.PFCPContext == nil {
		logger.WithTrace(ctx, logger.SmfLog).Debug("session already torn down, skipping deactivation",
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

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

	err = s.upf.ModifySession(ctx, BuildModifyRequest(
		remoteSEID,
		"",
		nil, farList, nil,
	))
	if err != nil {
		// A rejection means the PFCP session is gone (e.g. after a UPF restart);
		// clear it so later Activate/Release calls don't reuse a stale session.
		logger.WithTrace(ctx, logger.SmfLog).Warn("PFCP session modification failed, clearing stale tunnel",
			zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID),
			logger.SEID(localSEID))
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
