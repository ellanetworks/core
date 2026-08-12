// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	libngap "github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UpdateSmContextN2HandoverPreparing handles the handover-required N2 message
// and returns a PDUSession Resource Setup Request Transfer for the target radio.
func (s *SMF) UpdateSmContextN2HandoverPreparing(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_preparing",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("sm context has no user-plane tunnel: %s", smContextRef)
	}

	if smContext.PolicyData == nil {
		return nil, fmt.Errorf("sm context has no policy: %s", smContextRef)
	}

	if err := handleHandoverRequiredTransfer(n2Data); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover required transfer")

		return nil, fmt.Errorf("handle HandoverRequiredTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildHandoverRequestTransfer(&smContext.PolicyData.Ambr, &smContext.PolicyData.QosData, smContext.Tunnel.N3TEID, smContext.Tunnel.N3IPv4, smContext.Tunnel.N3IPv6, nasToNgapPDUSessionType(smContext.PDUSessionType), nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build handover request transfer")

		return nil, fmt.Errorf("build Handover Request Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

func handleHandoverRequiredTransfer(b []byte) error {
	if _, err := libngap.ParseHandoverRequiredTransfer(b); err != nil {
		return fmt.Errorf("failed to unmarshall handover required transfer: %w", err)
	}

	return nil
}

func (s *SMF) UpdateSmContextN2HandoverPrepared(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_prepared",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("sm context has no user-plane tunnel: %s", smContextRef)
	}

	if err := handleHandoverRequestAcknowledgeTransfer(n2Data, smContext); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover request acknowledge transfer")

		return nil, fmt.Errorf("handle HandoverRequestAcknowledgeTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildHandoverCommandTransfer()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build handover command transfer")

		return nil, fmt.Errorf("build Handover Command Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

// UpdateSmContextN2HandoverComplete handles the handover completion phase, sending
// the UPF an N4 Session Modification Request with the new AN tunnel info
// (TS 23.502).
func (s *SMF) UpdateSmContextN2HandoverComplete(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_complete",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM context reference is missing"))
		span.SetStatus(codes.Error, "SM context reference is missing")

		return fmt.Errorf("SM context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	dropped, err := s.switchDownlinkToTargetNGRAN(ctx, smContext, span)
	if err != nil {
		if errors.Is(err, errTransferRolledBack) {
			s.reportSessionNotMovedTo5GS(ctx, smContext)

			if releaseErr := s.releaseSession(ctx, smContextRef); releaseErr != nil {
				logger.WithTrace(ctx, logger.SmfLog).Warn("failed to release a session whose move onto 5GS was rolled back",
					zap.Error(releaseErr), zap.String("ref", smContextRef))
			}
		}

		return err
	}

	s.dropSourceRouting(ctx, smContext.Ref, dropped)

	return nil
}

func (s *SMF) switchDownlinkToTargetNGRAN(ctx context.Context, smContext *SMContext, span trace.Span) (*droppedSource, error) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("sm context has no user-plane tunnel: %s", smContext.Ref)
	}

	commit, err := s.beginTransferCommit(ctx, smContext, Access5G)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to move a PDN connection onto 5GS")

		return nil, fmt.Errorf("failed to move a PDN connection onto 5GS: %w", err)
	}

	if commit == nil && smContext.Access != Access5G {
		err := fmt.Errorf("session %q is on %s, not 5GS", smContext.Ref, smContext.Access)
		span.RecordError(err)
		span.SetStatus(codes.Error, "session is not on 5GS")

		return nil, err
	}

	if !smContext.Tunnel.Activated {
		if commit == nil {
			smContext.handoverSourceAN = nil

			return nil, nil
		}

		commit.restore()

		return nil, fmt.Errorf("session %q has no activated user plane to switch onto 5GS", smContext.Ref)
	}

	if smContext.PFCPContext == nil {
		if commit != nil {
			commit.restore()
		}

		span.RecordError(fmt.Errorf("pfcp session context not found"))
		span.SetStatus(codes.Error, "pfcp session context not found")

		return nil, fmt.Errorf("pfcp session context not found")
	}

	var (
		policyID string
		qers     []*QER
	)

	restoreBinding := func() {}

	if commit != nil {
		policyID = commit.policy.PolicyID
		qers = commit.qers
		restoreBinding = smContext.stageAccessBinding()
		smContext.Tunnel.DownlinkPDR.FAR.ApplyAction = models.ApplyAction{Forw: true}
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.SEID,
		policyID,
		[]*PDR{smContext.Tunnel.UplinkPDR},
		[]*FAR{smContext.Tunnel.DownlinkPDR.FAR},
		qers,
	)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to modify PFCP session")

		if commit != nil {
			restoreBinding()
			commit.restore()

			smContext.releasing = true

			return nil, fmt.Errorf("%w: %v", errTransferRolledBack, err)
		}

		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	smContext.handoverSourceAN = nil

	var dropped *droppedSource
	if commit != nil {
		dropped = smContext.finishTransferCommit(commit)
	}

	s.registerIPv6SessionIfNeeded(ctx, smContext, Access5G)

	logger.SmfLog.Info("Sent PFCP session modification for N2 handover completion",
		logger.SUPI(smContext.Supi.String()),
		logger.PDUSessionID(smContext.PDUSessionID))

	return dropped, nil
}

func handleHandoverRequestAcknowledgeTransfer(b []byte, smContext *SMContext) error {
	transfer, err := libngap.ParseHandoverRequestAcknowledgeTransfer(b)
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover request acknowledge transfer: %w", err)
	}

	source := smContext.Tunnel.AN
	smContext.handoverSourceAN = &source

	smContext.bindAccessTunnel(anchorFromGTPTunnel(transfer.DLNGUUPTNLInformation.GTPTunnel), Access5G)

	return nil
}

func (s *SMF) UpdateSmContextN2HandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_failed",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.abandonTransferTo(Access5G)

	transfer, err := libngap.ParseHandoverResourceAllocationUnsuccessfulTransfer(n2Data)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover resource allocation unsuccessful transfer")

		return fmt.Errorf("failed to unmarshall handover resource allocation unsuccessful transfer: %w", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("target NG-RAN node refused a PDU session at handover",
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID),
		zap.String("cause", transfer.Cause.String()))

	return nil
}

// Idempotent: a session with no prepared handover, or one already completed, is
// a no-op.
func (s *SMF) UpdateSmContextN2HandoverCanceled(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_canceled",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.pending != nil && smContext.pending.to == Access5G {
		smContext.clearPendingLocked()
	}

	source := smContext.handoverSourceAN
	if source == nil || smContext.Tunnel == nil {
		return nil
	}

	smContext.handoverSourceAN = nil

	smContext.bindAccessTunnel(*source, smContext.Access)

	if !smContext.Tunnel.Activated || smContext.PFCPContext == nil {
		return nil
	}

	dlFAR := smContext.Tunnel.DownlinkPDR.FAR
	ulPDR := smContext.Tunnel.UplinkPDR

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.SEID,
		"",
		[]*PDR{ulPDR}, []*FAR{dlFAR}, nil,
	)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to restore the source access tunnel")

		return fmt.Errorf("failed to restore the source access tunnel: %w", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("restored the source access tunnel after an abandoned N2 handover",
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}

// UpdateSmContextXnHandoverPathSwitchReq handles an Xn handover path-switch request.
func (s *SMF) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_handover_path_switch_request",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("sm context has no user-plane tunnel: %s", smContextRef)
	}

	restoreBinding := smContext.stageAccessBinding()

	pdrList, farList, n2buf, err := handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data, smContext)
	if err != nil {
		restoreBinding()

		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		restoreBinding()

		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.SEID,
		"",
		pdrList, farList, nil,
	)); err != nil {
		restoreBinding()

		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	// Re-register the IPv6 session with the new gNB tunnel endpoint.
	s.registerIPv6SessionIfNeeded(ctx, smContext, Access5G)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return n2buf, nil
}

func handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data []byte, smContext *SMContext) ([]*PDR, []*FAR, []byte, error) {
	logger.SmfLog.Debug("handle Path Switch Request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	if err := handlePathSwitchRequestTransfer(n2Data, smContext); err != nil {
		return nil, nil, nil, fmt.Errorf("handle PathSwitchRequestTransfer failed: %v", err)
	}

	n2Buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.N3TEID, smContext.Tunnel.N3IPv4, smContext.Tunnel.N3IPv6)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	var (
		pdrList []*PDR
		farList []*FAR
	)

	if smContext.Tunnel.Activated {
		pdrList = []*PDR{smContext.Tunnel.UplinkPDR}
		farList = []*FAR{smContext.Tunnel.DownlinkPDR.FAR}
	}

	return pdrList, farList, n2Buf, nil
}

func handlePathSwitchRequestTransfer(b []byte, smContext *SMContext) error {
	pathSwitchRequestTransfer, err := libngap.ParsePathSwitchRequestTransfer(b)
	if err != nil {
		return err
	}

	smContext.bindAccessTunnel(anchorFromGTPTunnel(pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel), Access5G)

	return nil
}

func (s *SMF) UpdateSmContextXnHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_xn_handover_failed",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	return handlePathSwitchRequestSetupFailedTransfer(n2Data)
}

func handlePathSwitchRequestSetupFailedTransfer(b []byte) error {
	if _, err := libngap.ParsePathSwitchRequestSetupFailedTransfer(b); err != nil {
		return fmt.Errorf("failed to unmarshall path switch request setup failed transfer: %w", err)
	}

	return nil
}
