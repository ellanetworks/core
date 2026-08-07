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
)

// UpdateSmContextN2HandoverPreparing handles the handover-required N2 message
// and returns a PDUSession Resource Setup Request Transfer for the target radio.
func (s *SMF) UpdateSmContextN2HandoverPreparing(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_preparing",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	smContext, unlock, err := s.sessionFor(smContextRef, Access5G)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "no session on 5GS for the reference")

		return nil, err
	}

	defer unlock()

	if err := handleHandoverRequiredTransfer(n2Data); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover required transfer")

		return nil, fmt.Errorf("handle HandoverRequiredTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&smContext.PolicyData.Ambr, &smContext.PolicyData.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6, nasToNgapPDUSessionType(smContext.PDUSessionType))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource setup request transfer")

		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

func handleHandoverRequiredTransfer(b []byte) error {
	if _, err := libngap.ParseHandoverRequiredTransfer(b); err != nil {
		return fmt.Errorf("failed to unmarshall handover required transfer: %w", err)
	}

	return nil
}

// UpdateSmContextN2HandoverPrepared handles the handover request acknowledge
// from the target radio and returns a Handover Command Transfer.
func (s *SMF) UpdateSmContextN2HandoverPrepared(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_prepared",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	smContext, unlock, err := s.sessionFor(smContextRef, Access5G)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "no session on 5GS for the reference")

		return nil, err
	}

	defer unlock()

	if err := handleHandoverRequestAcknowledgeTransfer(n2Data, smContext); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover request acknowledge transfer")

		return nil, fmt.Errorf("handle HandoverRequestAcknowledgeTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildHandoverCommandTransfer(smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6)
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

	smContext, unlock, err := s.sessionFor(smContextRef, Access5G)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "no session on 5GS for the reference")

		return err
	}

	defer unlock()

	if smContext.Tunnel.DataPath.Activated {
		if smContext.PFCPContext == nil {
			span.RecordError(fmt.Errorf("pfcp session context not found"))
			span.SetStatus(codes.Error, "pfcp session context not found")

			return fmt.Errorf("pfcp session context not found")
		}

		var (
			pdrList []*PDR
			farList []*FAR
		)

		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)

		smContext.Tunnel.DataPath.UpLinkTunnel.PDR.State = RuleUpdate
		pdrList = append(pdrList, smContext.Tunnel.DataPath.UpLinkTunnel.PDR)

		if err := s.upf.ModifySession(ctx, BuildModifyRequest(
			smContext.PFCPContext.RemoteSEID,
			"",
			pdrList, farList, nil,
		)); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to modify PFCP session")

			return fmt.Errorf("failed to send PFCP session modification request: %v", err)
		}

		s.registerIPv6SessionIfNeeded(ctx, smContext)

		logger.SmfLog.Info("Sent PFCP session modification for N2 handover completion",
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID))
	}

	return nil
}

func handleHandoverRequestAcknowledgeTransfer(b []byte, smContext *SMContext) error {
	transfer, err := libngap.ParseHandoverRequestAcknowledgeTransfer(b)
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover request acknowledge transfer: %w", err)
	}

	smContext.bindAccessTunnel(anchorFromGTPTunnel(transfer.DLNGUUPTNLInformation.GTPTunnel))

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate
	}

	return nil
}

// UpdateSmContextXnHandoverPathSwitchReq handles an Xn handover path-switch request.
func (s *SMF) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_handover_path_switch_request",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	smContext, unlock, err := s.sessionFor(smContextRef, Access5G)
	if err != nil {
		return nil, err
	}

	defer unlock()

	pdrList, farList, n2buf, err := handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		"",
		pdrList, farList, nil,
	)); err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	// Re-register the IPv6 session with the new gNB tunnel endpoint.
	s.registerIPv6SessionIfNeeded(ctx, smContext)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return n2buf, nil
}

func handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data []byte, smContext *SMContext) ([]*PDR, []*FAR, []byte, error) {
	logger.SmfLog.Debug("handle Path Switch Request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	if err := handlePathSwitchRequestTransfer(n2Data, smContext); err != nil {
		return nil, nil, nil, fmt.Errorf("handle PathSwitchRequestTransfer failed: %v", err)
	}

	n2Buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	var pdrList []*PDR

	var farList []*FAR

	if smContext.Tunnel.DataPath.Activated {
		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)

		// Include the UL PDR so its updated OuterHeaderRemoval reaches the UPF.
		smContext.Tunnel.DataPath.UpLinkTunnel.PDR.State = RuleUpdate
		pdrList = append(pdrList, smContext.Tunnel.DataPath.UpLinkTunnel.PDR)
	}

	return pdrList, farList, n2Buf, nil
}

func handlePathSwitchRequestTransfer(b []byte, smContext *SMContext) error {
	pathSwitchRequestTransfer, err := libngap.ParsePathSwitchRequestTransfer(b)
	if err != nil {
		return err
	}

	smContext.bindAccessTunnel(anchorFromGTPTunnel(pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel))

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate
	}

	return nil
}

// UpdateSmContextHandoverFailed handles a path switch failure.
func (s *SMF) UpdateSmContextHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_handover_failed",
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
