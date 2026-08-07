// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UpdateResult carries the N1/N2 messages produced by an SM context update.
type UpdateResult struct {
	ReleaseN2 bool   // true when N2 info signals PDU session resource release
	N1Msg     []byte // NAS message for the UE (may be nil)
	N2Msg     []byte // NGAP transfer for the RAN (may be nil)

	// The AMF holds a routing context per PDU session and has no other way to
	// learn this: the message that ends the session produces no N1 or N2 answer.
	SessionRemoved bool
}

// UpdateSmContextN1Msg handles a NAS N1 message update (e.g. PDU session release request).
func (s *SMF) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*UpdateResult, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_n1_msg",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("%w: %s", ErrSMContextNotFound, smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	rsp, err := s.handleUpdateN1Msg(ctx, n1Msg, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle N1 message")

		return nil, fmt.Errorf("error handling N1 message: %v", err)
	}

	return rsp, nil
}

func (s *SMF) handleUpdateN1Msg(ctx context.Context, n1Msg []byte, smContext *SMContext) (*UpdateResult, error) {
	if n1Msg == nil {
		return nil, nil
	}

	msg, err := fgs.ParseMessage(n1Msg)
	if err != nil && !naslib.SoftOnly(err) {
		return nil, fmt.Errorf("error decoding N1SmMessage: %v", err)
	}

	gsm, ok := msg.(fgs.GSMMessage)
	if !ok {
		logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg is not a 5GSM message",
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		return nil, nil
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Update SM Context Request N1SmMessage", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	msgType := gsm.MessageType()
	pti := uint8(gsm.TransactionIdentity())

	switch verdict, cause := smfNas.PolicePTI(msgType, pti, smContext.IsPTIInUse); verdict {
	case smfNas.PTIIgnore:
		logger.WithTrace(ctx, logger.SmfLog).Info("ignoring 5GSM message with reserved PTI", zap.Uint8("MessageType", uint8(msgType)), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil, nil
	case smfNas.PTIRespondStatus:
		n1SmMsg, err := smfNas.BuildGSM5GSMStatus(fgs.PDUSessionID(smContext.PDUSessionID), naslib.ProcedureTransactionIdentity(pti), cause)
		if err != nil {
			return nil, fmt.Errorf("build GSM 5GSM STATUS failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil
	}

	switch msg := msg.(type) {
	case *fgs.PDUSessionReleaseRequest:
		// A UE-requested release runs as a network-requested release
		// (TS 24.501 §6.4.3.3 → §6.3.3): the UE-allocated PTI is carried on the
		// Release Command and held until the matching Release Complete; T3592
		// retransmits the command meanwhile.
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Release Request received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		if err := s.startRelease(ctx, smContext, pti, fgs.GSMCauseRegularDeactivation); err != nil {
			return nil, fmt.Errorf("start PDU session release: %w", err)
		}

		return nil, nil

	case *fgs.PDUSessionModificationRequest:
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Modification Request received; rejecting", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		// The UE cannot set its own QoS; the authorized QoS is network-determined
		// and not modifiable on UE request, so the request is rejected
		// (TS 24.501 clause 6.4.2.4).
		n1SmMsg, err := smfNas.BuildGSMPDUSessionModificationReject(fgs.PDUSessionID(smContext.PDUSessionID), naslib.ProcedureTransactionIdentity(pti), fgs.GSMCauseRequestRejectedUnspecified)
		if err != nil {
			return nil, fmt.Errorf("build GSM PDUSessionModificationReject failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil

	case *fgs.PDUSessionReleaseComplete:
		// Release acknowledged; stop T3592 and tear down the user plane, held active
		// through the release window (TS 24.501 §6.3.3.3).
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Release Complete received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		smContext.stopProcedureTimer()
		smContext.ClearPTIInUse(pti)
		s.teardownAndRemove(ctx, smContext)

		return &UpdateResult{SessionRemoved: true}, nil

	case *fgs.PDUSessionModificationComplete:
		// The UE accepted the modification; stop T3591 and commit the new policy
		// (TS 24.501 §6.3.2.2, "consider the PDU session as modified").
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Modification Complete received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		smContext.stopProcedureTimer()
		smContext.ClearPTIInUse(pti)

		if smContext.pendingPolicy != nil {
			smContext.PolicyData = smContext.pendingPolicy
			smContext.pendingPolicy = nil
		}

		return nil, nil

	case *fgs.PDUSessionModificationCommandReject:
		// The UE rejected the modification; stop T3591 and discard the pending policy,
		// keeping the previous configuration (TS 24.501 §6.3.2.4, §6.3.2.5).
		logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg PDU Session Modification Command Reject received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		smContext.stopProcedureTimer()
		smContext.ClearPTIInUse(pti)
		smContext.pendingPolicy = nil

		return nil, nil

	case *fgs.GSMStatus:
		removed := s.handle5GSMStatus(ctx, smContext, pti, msg.Cause)

		return &UpdateResult{SessionRemoved: removed}, nil

	case *fgs.UnknownGSMMessage:
		// TS 24.501 §7.4: a 5GSM message type the receiver does not implement is
		// ignored except that it draws a 5GSM STATUS with cause #97, naming the
		// PDU session and transaction the offending message carried.
		logger.WithTrace(ctx, logger.SmfLog).Warn("unimplemented 5GSM message type",
			zap.Stringer("MessageType", msgType), logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID))

		n1SmMsg, err := smfNas.BuildGSM5GSMStatus(msg.SessionIdentity(), msg.TransactionIdentity(),
			fgs.GSMCauseMessageTypeNonExistentOrNotImplemented)
		if err != nil {
			return nil, fmt.Errorf("build GSM 5GSM STATUS failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil

	default:
		logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg type not supported in SM Context Update", zap.Stringer("MessageType", msgType), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil, nil
	}
}

// UpdateSmContextN2InfoPduResSetupRsp handles the N2 PDUSession Resource Setup Response.
func (s *SMF) UpdateSmContextN2InfoPduResSetupRsp(ctx context.Context, smContextRef string, n2Data []byte) error {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_pdu_resource_setup_response",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	dropped, err := s.bindNGRANDownlink(ctx, smContext, n2Data, span)
	if err != nil {
		return err
	}

	s.dropSourceRouting(ctx, smContext.Ref, dropped)

	return nil
}

func (s *SMF) bindNGRANDownlink(ctx context.Context, smContext *SMContext, n2Data []byte, span trace.Span) (*droppedSource, error) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || !smContext.Tunnel.Activated {
		span.RecordError(fmt.Errorf("session already released"))
		span.SetStatus(codes.Error, "session already released")

		return nil, fmt.Errorf("session already released")
	}

	pdrList, farList, err := handleUpdateN2MsgPDUResourceSetupResp(n2Data, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle N2 message")

		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		span.RecordError(fmt.Errorf("pfcp session context not found"))
		span.SetStatus(codes.Error, "pfcp session context not found")

		return nil, fmt.Errorf("pfcp session context not found")
	}

	commit, err := s.beginTransferCommit(ctx, smContext, Access5G)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to move the session to 5GS")

		return nil, fmt.Errorf("failed to move the session to 5GS: %w", err)
	}

	var qers []*QER

	policyID := ""

	if commit != nil {
		qers = commit.qers
		policyID = commit.policy.PolicyID
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.SEID,
		policyID,
		pdrList, farList, qers,
	)); err != nil {
		if commit != nil {
			commit.restore()
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to modify PFCP session")

		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	s.registerIPv6SessionIfNeeded(ctx, smContext)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	if commit == nil {
		return nil, nil
	}

	return smContext.finishTransferCommit(commit), nil
}

func handleUpdateN2MsgPDUResourceSetupResp(binaryDataN2SmInformation []byte, smContext *SMContext) ([]*PDR, []*FAR, error) {
	logger.SmfLog.Debug("received n2 sm info type", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	var pdrList []*PDR

	var farList []*FAR

	if smContext.Tunnel.Activated {
		smContext.Tunnel.DownlinkPDR.FAR.ApplyAction = models.ApplyAction{Forw: true}
		smContext.Tunnel.DownlinkPDR.FAR.ForwardingParameters = &models.ForwardingParameters{}

		pdrList = append(pdrList, smContext.Tunnel.DownlinkPDR)
		farList = append(farList, smContext.Tunnel.DownlinkPDR.FAR)

		// Initial PDR creation set the UL OuterHeaderRemoval before the gNB IP was
		// known, so the corrected value has to reach the UPF too.
		pdrList = append(pdrList, smContext.Tunnel.UplinkPDR)
	}

	if err := handlePDUSessionResourceSetupResponseTransfer(binaryDataN2SmInformation, smContext); err != nil {
		return nil, nil, fmt.Errorf("handle PDUSessionResourceSetupResponseTransfer failed: %v", err)
	}

	return pdrList, farList, nil
}

func anchorFromGTPTunnel(t libngap.GTPTunnel) AnchorBinding {
	ipv4, ipv6 := ngap.ParseTransportLayerAddress(t.TransportLayerAddress)

	return AnchorBinding{
		TEID: uint32(t.GTPTEID),
		IPv4: ipv4,
		IPv6: ipv6,
	}
}

func handlePDUSessionResourceSetupResponseTransfer(b []byte, smContext *SMContext) error {
	transfer, err := libngap.ParsePDUSessionResourceSetupResponseTransfer(b)
	if err != nil {
		return fmt.Errorf("failed to unmarshall resource setup response transfer: %w", err)
	}

	// UPTransportLayerInformation is a CHOICE whose only modelled alternative is
	// gTPTunnel; the decoder refuses choice-Extensions on our behalf.
	smContext.bindAccessTunnel(anchorFromGTPTunnel(transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel))

	return nil
}

// UpdateSmContextN2InfoPduResSetupFail handles a PDUSession Resource Setup failure.
func (s *SMF) UpdateSmContextN2InfoPduResSetupFail(ctx context.Context, smContextRef string, n2Data []byte) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_pdu_resource_setup_fail",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	return handlePDUSessionResourceSetupUnsuccessfulTransfer(n2Data)
}

func handlePDUSessionResourceSetupUnsuccessfulTransfer(b []byte) error {
	transfer, err := libngap.ParsePDUSessionResourceSetupUnsuccessfulTransfer(b)
	if err != nil {
		return fmt.Errorf("failed to unmarshall resource setup unsuccessful transfer: %w", err)
	}

	logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful", logger.Cause(transfer.Cause.String()))

	return nil
}

// UpdateSmContextN2InfoPduResRelRsp handles the final N2 PDU Session Resource Release Response.
func (s *SMF) UpdateSmContextN2InfoPduResRelRsp(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_pdu_resource_release_response",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		// Session already removed (e.g. by slice-mismatch release); return nil to
		// keep the response idempotent.
		logger.SmfLog.Info("SM context already removed, skipping",
			zap.String("smContextRef", smContextRef))

		return nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	// N2 release complete; stop T3592 (TS 24.501 §6.3.3).
	smContext.stopProcedureTimer()

	if smContext.PDUSessionReleaseDueToDupPduID {
		// Duplicate-PDU-ID release: the tunnel was already torn down when the
		// duplicate was detected (UpdateSmContextCauseDuplicatePDUSessionID).
		smContext.PDUSessionReleaseDueToDupPduID = false
		s.RemoveSession(ctx, smContext.Ref)
	} else {
		// Network-requested release: tear down the user plane, held active through
		// the release window (TS 23.502 §4.3.4).
		s.teardownAndRemove(ctx, smContext)
	}

	return nil
}

// UpdateSmContextCauseDuplicatePDUSessionID handles duplicate PDU session ID by releasing
// the existing session and building a release command for the radio.
func (s *SMF) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_cause_duplicate_pdu_session_id",
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

	smContext.PDUSessionReleaseDueToDupPduID = true

	n2Rsp, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource release command transfer")

		return nil, fmt.Errorf("build PDUSession Resource Release Command Transfer Error: %v", err)
	}

	if err := s.releaseTunnel(ctx, smContext); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release tunnel")

		return nil, fmt.Errorf("failed to release tunnel: %v", err)
	}

	return n2Rsp, nil
}

// registerIPv6SessionIfNeeded registers the IPv6 session with the UPF's RA
// responder if the session has a delegated IPv6 prefix and the gNB's tunnel
// endpoint is known. Must be called with smContext.Mutex held.
func (s *SMF) registerIPv6SessionIfNeeded(ctx context.Context, smContext *SMContext) {
	if smContext.PDUIPV6Prefix == nil || smContext.Tunnel == nil {
		return
	}

	anInfo := smContext.Tunnel.AN
	if anInfo.TEID == 0 {
		return
	}

	var gnbIP netip.Addr

	if anInfo.IPv6 != nil {
		if addr, ok := netip.AddrFromSlice(anInfo.IPv6.To16()); ok {
			gnbIP = addr
		}
	}

	if !gnbIP.IsValid() && anInfo.IPv4 != nil {
		if addr, ok := netip.AddrFromSlice(anInfo.IPv4.To4()); ok {
			gnbIP = addr
		}
	}

	if !gnbIP.IsValid() {
		return
	}

	prefixAddr, ok := netip.AddrFromSlice(smContext.PDUIPV6Prefix.To16())
	if !ok {
		return
	}

	var qfi uint8
	if smContext.PolicyData != nil {
		qfi = smContext.PolicyData.QosData.QFI
	}

	var mtu uint32
	if smContext.PolicyData != nil {
		mtu = uint32(smContext.PolicyData.MTU)
	}

	reg := &models.IPv6SessionRegistration{
		UplinkTEID:   smContext.Tunnel.N3TEID,
		DownlinkTEID: anInfo.TEID,
		GnbN3Addr:    gnbIP,
		Prefix:       netip.PrefixFrom(prefixAddr, 64),
		MTU:          mtu,
		QFI:          qfi,
		// A 4G S1-U bearer carries the RA PSC-less; 5G N3 carries it in the PDU
		// Session Container. The encap follows the access.
		S1U: !smContext.Access.usesPSC(),
	}

	if err := s.upf.RegisterIPv6Session(ctx, reg); err != nil {
		logger.SmfLog.Warn("failed to register IPv6 session for RA",
			zap.Error(err),
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)
	}
}
