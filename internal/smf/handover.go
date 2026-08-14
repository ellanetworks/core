// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
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

	dropped, err := s.switchDownlinkToTargetNGRAN(ctx, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to bind the downlink")
	}

	return s.finishBinding(ctx, smContext, dropped, err)
}

func (s *SMF) switchDownlinkToTargetNGRAN(ctx context.Context, smContext *SMContext) (*droppedSource, error) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil {
		return nil, fmt.Errorf("sm context has no user-plane tunnel: %s", smContext.Ref)
	}

	target := smContext.handoverTargetAN
	if target == nil {
		return nil, fmt.Errorf("session %q has no prepared handover to complete", smContext.Ref)
	}

	dropped, err := s.bindDownlink(ctx, smContext, Access5G, *target)
	if err != nil {
		return nil, err
	}

	smContext.handoverTargetAN = nil

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

	accepted := make([]libngap.QosFlowIdentifier, 0, len(transfer.QosFlowSetupResponse))
	for _, f := range transfer.QosFlowSetupResponse {
		accepted = append(accepted, f.QosFlowIdentifier)
	}

	if err := smContext.admittedSignalledFlow(accepted); err != nil {
		return err
	}

	target := anchorFromGTPTunnel(transfer.DLNGUUPTNLInformation.GTPTunnel)
	smContext.handoverTargetAN = &target

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

func (s *SMF) UpdateSmContextN2HandoverCanceled(ctx context.Context, smContextRef string) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_canceled",
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

	if smContext.handoverTargetAN == nil {
		return nil
	}

	smContext.handoverTargetAN = nil

	logger.WithTrace(ctx, logger.SmfLog).Info("dropped the target endpoint of an abandoned N2 handover",
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

	logger.SmfLog.Debug("handle Path Switch Request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	an, err := anchorFromPathSwitchRequest(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	n2buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.N3TEID, smContext.Tunnel.N3IPv4, smContext.Tunnel.N3IPv6)
	if err != nil {
		return nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	next := smContext.Tunnel.dataPlane
	next.AN = an

	if err := s.applyDataPlane(ctx, smContext, next, ""); err != nil {
		return nil, err
	}

	// Re-register the IPv6 session with the new gNB tunnel endpoint.
	s.registerIPv6SessionIfNeeded(ctx, smContext, Access5G)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return n2buf, nil
}

func anchorFromPathSwitchRequest(b []byte, smContext *SMContext) (AnchorBinding, error) {
	pathSwitchRequestTransfer, err := libngap.ParsePathSwitchRequestTransfer(b)
	if err != nil {
		return AnchorBinding{}, err
	}

	accepted := make([]libngap.QosFlowIdentifier, 0, len(pathSwitchRequestTransfer.QosFlowAccepted))
	for _, f := range pathSwitchRequestTransfer.QosFlowAccepted {
		accepted = append(accepted, f.QosFlowIdentifier)
	}

	if err := smContext.admittedSignalledFlow(accepted); err != nil {
		return AnchorBinding{}, err
	}

	return anchorFromGTPTunnel(pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel), nil
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

func (sc *SMContext) signalledQFI() uint8 {
	if sc.pending != nil && sc.pending.policy != nil {
		return transferPolicy(sc.PolicyData, sc.pending.policy).QosData.QFI
	}

	if sc.PolicyData == nil {
		return 0
	}

	return sc.PolicyData.QosData.QFI
}

func (sc *SMContext) admittedSignalledFlow(accepted []libngap.QosFlowIdentifier) error {
	want := sc.signalledQFI()
	for _, qfi := range accepted {
		if uint8(qfi) == want {
			return nil
		}
	}

	return fmt.Errorf("the NG-RAN node admitted QoS flows %v for session %q, not the signalled QFI %d",
		accepted, sc.Ref, want)
}
